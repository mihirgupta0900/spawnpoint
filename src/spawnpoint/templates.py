"""Workspace templates — reusable named sets of repos.

A template records the repos you keep spawning together (say "image-gen" =
chottu + backend + daily-prophet) plus an optional default base branch, so
recurring work starts from one flag instead of re-picking the same repos.

Templates live in their own file (``~/.spawnpoint/templates.toml``) because the
CLI writes them, while ``config.toml`` stays a hand-edited settings file that
``init``/``config --reset`` regenerate.
"""

import time
import tomllib
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional

import typer
from InquirerPy import inquirer
from rich.console import Console
from rich.table import Table

from .config import SPAWNPOINT_DIR, TEMPLATES_PATH, Config
from .io import emit_json, parse_csv, require, resolve_repos
from .log import logger
from .prompts import select_repos
from .utils import find_git_repos, make_display_path

console = Console(stderr=True)

# Detail line shown per template in listings; keeps long repo lists readable.
_MAX_DETAIL_LEN = 60


@dataclass
class Template:
    """A named set of repos to spawn together."""

    name: str
    repos: List[str] = field(default_factory=list)
    base: Optional[str] = None
    description: Optional[str] = None


# --- Storage -----------------------------------------------------------------

_TOML_ESCAPES = {"\\": "\\\\", '"': '\\"', "\n": "\\n", "\r": "\\r", "\t": "\\t"}


def _toml_str(value: str) -> str:
    """Quote a string as a TOML basic string."""
    out = []
    for ch in value:
        if ch in _TOML_ESCAPES:
            out.append(_TOML_ESCAPES[ch])
        elif ord(ch) < 0x20 or ord(ch) == 0x7F:
            out.append(f"\\u{ord(ch):04X}")
        else:
            out.append(ch)
    return '"' + "".join(out) + '"'


def load_templates() -> Dict[str, Template]:
    """Load templates keyed by name. Returns an empty dict if none are saved."""
    if not TEMPLATES_PATH.is_file():
        return {}

    try:
        with open(TEMPLATES_PATH, "rb") as f:
            data = tomllib.load(f)
    except tomllib.TOMLDecodeError as e:
        console.print(f"[bold red]Error:[/bold red] {TEMPLATES_PATH} is not valid TOML: {e}")
        console.print("Fix the file by hand, or delete it to start over.")
        raise typer.Exit(code=1)

    templates: Dict[str, Template] = {}
    for name, body in (data.get("templates") or {}).items():
        if not isinstance(body, dict):
            logger.debug("Skipping malformed template entry: %s", name)
            continue
        templates[name] = Template(
            name=name,
            repos=[str(r) for r in body.get("repos", [])],
            base=body.get("base") or None,
            description=body.get("description") or None,
        )
    return templates


def save_templates(templates: Dict[str, Template]) -> Path:
    """Write all templates to TOML, sorted by name. Returns the file path."""
    SPAWNPOINT_DIR.mkdir(parents=True, exist_ok=True)

    lines = [
        "# Spawnpoint workspace templates",
        "# Managed by `spawnpoint template save` and `spawnpoint template delete`.",
        "# Repo names must match the names shown by `spawnpoint repos`.",
        "",
    ]
    for name in sorted(templates):
        template = templates[name]
        lines.append(f"[templates.{_toml_str(name)}]")
        if template.description:
            lines.append(f"description = {_toml_str(template.description)}")
        lines.append("repos = [" + ", ".join(_toml_str(r) for r in template.repos) + "]")
        if template.base:
            lines.append(f"base = {_toml_str(template.base)}")
        lines.append("")

    TEMPLATES_PATH.write_text("\n".join(lines))
    return TEMPLATES_PATH


def get_template(name: str) -> Template:
    """Return a template by name, or exit listing the valid names."""
    templates = load_templates()
    if name in templates:
        return templates[name]

    console.print(
        f"[bold red]Error:[/bold red] template '{name}' not found. "
        f"Valid: {', '.join(sorted(templates)) or '(none)'}"
    )
    console.print("Create one with [bold]spawnpoint template save <name>[/bold].")
    raise typer.Exit(code=1)


def upsert_template(template: Template) -> tuple[Path, bool]:
    """Save a template. Returns (path, existed_before)."""
    templates = load_templates()
    existed = template.name in templates
    templates[template.name] = template
    return save_templates(templates), existed


def merge_template(
    name: str,
    repos: List[str],
    *,
    base: Optional[str] = None,
    description: Optional[str] = None,
) -> Template:
    """Build a template, keeping the saved base/description when not overridden."""
    existing = load_templates().get(name)
    return Template(
        name=name,
        repos=repos,
        base=base or (existing.base if existing else None),
        description=description or (existing.description if existing else None),
    )


def template_payload(template: Template) -> dict:
    return {
        "name": template.name,
        "repos": template.repos,
        "base": template.base,
        "description": template.description,
    }


# --- Shared helpers ----------------------------------------------------------


def template_detail(template: Template) -> str:
    """Short one-line summary of a template, for pickers and tables."""
    detail = template.description or ", ".join(template.repos)
    if len(detail) > _MAX_DETAIL_LEN:
        detail = detail[: _MAX_DETAIL_LEN - 3] + "..."
    return detail


def template_repo_hint(name: str) -> str:
    """Failure hint for repo names that came from a template rather than a flag."""
    return (
        f"Those repo names came from template '{name}'. Update it with "
        f"[bold]spawnpoint template save {name} --repos ...[/bold]."
    )


def preselect_template_repos(template: Template, choice_to_path: Dict[str, Path]) -> List[str]:
    """Map a template's repo names onto available picker labels.

    Leniently skips repos that no longer exist (with a warning) so a stale
    template still gets you a pre-filled picker instead of an error.
    """
    label_by_name: Dict[str, str] = {label: label for label in choice_to_path}
    for label, path in choice_to_path.items():
        label_by_name.setdefault(path.name, label)

    labels: List[str] = []
    missing: List[str] = []
    for name in template.repos:
        label = label_by_name.get(name)
        if label is None:
            missing.append(name)
        elif label not in labels:
            labels.append(label)

    if missing:
        console.print(
            f"[yellow]Template '{template.name}' lists repos that are no longer "
            f"available: {', '.join(missing)}[/yellow]"
        )
    return labels


def prompt_for_template(templates: Dict[str, Template]) -> Optional[Template]:
    """Offer saved templates as a starting point. Returns None for manual picking."""
    manual = "Pick repos manually"
    by_label = {
        f"{templates[name].name}  ({template_detail(templates[name])})": templates[name]
        for name in sorted(templates)
    }
    selected = inquirer.select(
        message="Start from a template?",
        choices=[manual] + list(by_label),
        default=manual,
    ).execute()
    return by_label.get(selected)


def _scan_repo_choices(cfg: Config) -> Dict[str, Path]:
    """Scan configured dirs and return {display label: repo path}."""
    valid_dirs = [d for d in cfg.scan_dirs if d.is_dir()]
    if not valid_dirs:
        console.print("[bold red]Error:[/bold red] No valid scan directories configured.")
        console.print("Run [bold]spawnpoint init[/bold] to set up.")
        raise typer.Exit(code=1)

    start = time.monotonic()
    repos = find_git_repos(valid_dirs, cfg.scan_depth)
    logger.debug("Repo scan completed in %.2fs, found %d repos", time.monotonic() - start, len(repos))

    if not repos:
        console.print("[yellow]No git repositories found.[/yellow]")
        raise typer.Exit(code=1)

    return {make_display_path(repo, valid_dirs): repo for repo in repos}


# --- Commands ----------------------------------------------------------------


def run_template_list(*, json_output: bool = False):
    """List saved templates."""
    templates = load_templates()

    if json_output:
        emit_json([template_payload(templates[name]) for name in sorted(templates)])
        return

    if not templates:
        console.print("[yellow]No templates saved yet.[/yellow]")
        console.print(
            "Create one with [bold]spawnpoint template save <name>[/bold], or "
            "[bold]spawnpoint create --save-template <name>[/bold]."
        )
        return

    table = Table(show_header=True, header_style="bold")
    table.add_column("Template")
    table.add_column("Repos")
    table.add_column("Base")
    table.add_column("Description")

    for name in sorted(templates):
        template = templates[name]
        table.add_row(
            name,
            ", ".join(template.repos) or "[dim]none[/dim]",
            template.base or "[dim]auto[/dim]",
            template.description or "",
        )

    console.print(table)


def run_template_show(name: str, *, json_output: bool = False):
    """Show one template."""
    template = get_template(name)

    if json_output:
        emit_json(template_payload(template))
        return

    console.print(f"[bold]{template.name}[/bold]")
    if template.description:
        console.print(f"  {template.description}")
    console.print(f"  Repos: {', '.join(template.repos) or '(none)'}")
    console.print(f"  Base:  {template.base or 'auto-detected per repo'}")


def run_template_save(
    cfg: Config,
    name: str,
    *,
    repos_arg: Optional[str] = None,
    base: Optional[str] = None,
    description: Optional[str] = None,
    no_input: bool = False,
    json_output: bool = False,
):
    """Create or update a template, picking repos interactively unless --repos is given."""
    if not name.strip():
        console.print("[bold red]Error:[/bold red] Template name cannot be empty.")
        raise typer.Exit(code=1)

    existing = load_templates().get(name)
    choice_to_path = _scan_repo_choices(cfg)

    if no_input:
        repos_arg = require(repos_arg, "--repos", console)

    if repos_arg:
        selected = resolve_repos(parse_csv(repos_arg), choice_to_path, err=console)
    else:
        preselected = preselect_template_repos(existing, choice_to_path) if existing else []
        labels = select_repos(
            f"Select repositories for template '{name}' (type to search):",
            list(choice_to_path),
            preselected=preselected,
        )
        if not labels:
            console.print("No repositories selected. Nothing saved.")
            raise typer.Exit()
        selected = [choice_to_path[label] for label in labels]

    if not selected:
        console.print("[bold red]Error:[/bold red] A template needs at least one repo.")
        raise typer.Exit(code=1)

    valid_dirs = [d for d in cfg.scan_dirs if d.is_dir()]
    template = merge_template(
        name,
        [make_display_path(path, valid_dirs) for path in selected],
        base=base,
        description=description,
    )
    path, existed = upsert_template(template)

    verb = "Updated" if existed else "Saved"
    console.print(f"[green]{verb} template '{name}'[/green] ({', '.join(template.repos)})")
    console.print(f"[dim]{path}[/dim]")
    console.print(f"Use it with [bold]spawnpoint create --template {name}[/bold]")

    if json_output:
        emit_json(template_payload(template))


def run_template_delete(name: str, *, no_input: bool = False, json_output: bool = False):
    """Delete a template."""
    template = get_template(name)

    if not no_input:
        confirm = inquirer.confirm(
            message=f"Delete template '{name}' ({', '.join(template.repos)})?",
            default=False,
        ).execute()
        if not confirm:
            console.print("Aborted.")
            raise typer.Exit()

    templates = load_templates()
    templates.pop(name, None)
    save_templates(templates)

    console.print(f"[green]Deleted template '{name}'.[/green]")
    console.print("[dim]Existing workspaces are untouched.[/dim]")

    if json_output:
        emit_json({"deleted": name})
