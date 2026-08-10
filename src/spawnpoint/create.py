import subprocess
import time
from pathlib import Path
from typing import Any, Dict, List

import typer
from InquirerPy import inquirer
from rich.console import Console
from rich.progress import track

from .config import CD_PATH_FILE, Config
from .io import emit_json, parse_csv, require, resolve_repos
from .log import logger
from .prompts import select_repos
from .templates import (
    get_template,
    load_templates,
    merge_template,
    preselect_template_repos,
    prompt_for_template,
    template_repo_hint,
    upsert_template,
)
from .utils import (
    copy_essential_files,
    detect_default_branch,
    find_git_repos,
    make_display_path,
    setup_dependencies,
)

console = Console(stderr=True)


def _offer_save_template(selected_names: List[str], active_template) -> str | None:
    """Offer to remember an interactively-picked repo set. Returns a name or None.

    Skipped for a single repo (a template buys you little there) and when the
    selection still matches the template it came from.
    """
    if len(selected_names) < 2:
        return None
    if active_template and set(selected_names) == set(active_template.repos):
        return None

    if not inquirer.confirm(
        message=f"Save these {len(selected_names)} repos as a template for next time?",
        default=False,
    ).execute():
        return None

    name = (inquirer.text(message="Template name:").execute() or "").strip()
    if not name:
        console.print("[dim]No name given, template not saved.[/dim]")
        return None

    if name in load_templates() and not inquirer.confirm(
        message=f"Template '{name}' already exists. Overwrite it?",
        default=False,
    ).execute():
        console.print("[dim]Template not saved.[/dim]")
        return None

    return name


def run_create(
    cfg: Config,
    yes: bool = False,
    *,
    no_input: bool = False,
    repos_arg: str | None = None,
    branch: str | None = None,
    base: str | None = None,
    template: str | None = None,
    save_template: str | None = None,
    json_output: bool = False,
):
    """Select git repos and create worktrees for a feature branch."""
    if not cfg.scan_dirs:
        console.print("[bold red]Error:[/bold red] No scan directories configured.")
        console.print("Run [bold]spawnpoint init[/bold] to set up.")
        raise typer.Exit(code=1)

    # Validate a named template before the repo scan so typos fail fast.
    active_template = get_template(template) if template else None

    # Validate scan dirs exist
    valid_dirs = [d for d in cfg.scan_dirs if d.is_dir()]
    if not valid_dirs:
        console.print("[bold red]Error:[/bold red] None of your scan directories exist:")
        for d in cfg.scan_dirs:
            console.print(f"  {d}")
        raise typer.Exit(code=1)

    console.print("[bold blue]Scanning for git repositories...[/bold blue]")
    for d in valid_dirs:
        console.print(f"  [dim]{d}[/dim]")

    start = time.monotonic()
    repos = find_git_repos(valid_dirs, cfg.scan_depth)
    logger.debug("Repo scan completed in %.2fs, found %d repos", time.monotonic() - start, len(repos))

    if not repos:
        console.print("[yellow]No git repositories found.[/yellow]")
        raise typer.Exit()

    # Display paths relative to scan dirs for readability
    choices = [make_display_path(repo, valid_dirs) for repo in repos]
    choice_to_path = dict(zip(choices, repos))

    # Offer saved templates as a starting point when nothing was passed by flag.
    if active_template is None and not no_input and not repos_arg:
        saved_templates = load_templates()
        if saved_templates:
            active_template = prompt_for_template(saved_templates)

    if active_template:
        console.print(f"\n[bold blue]Template:[/bold blue] {active_template.name}")
        if repos_arg:
            console.print("  [dim]--repos overrides its repo list[/dim]")
        else:
            console.print(f"  [dim]repos: {', '.join(active_template.repos)}[/dim]")
        if base is None and active_template.base:
            base = active_template.base
            console.print(f"  [dim]base branch: {base}[/dim]")

    if no_input:
        if not repos_arg and not active_template:
            console.print("[bold red]Error:[/bold red] --no-input requires --repos or --template.")
            raise typer.Exit(code=1)
        if repos_arg:
            # An explicit --repos wins over the template's repo list.
            selected_repos = resolve_repos(parse_csv(repos_arg), choice_to_path, err=console)
        else:
            selected_repos = resolve_repos(
                active_template.repos,
                choice_to_path,
                err=console,
                hint=template_repo_hint(active_template.name),
            )
        if not selected_repos:
            console.print("[bold red]Error:[/bold red] No repositories to spawn.")
            raise typer.Exit(code=1)
    else:
        preselected = (
            preselect_template_repos(active_template, choice_to_path) if active_template else []
        )
        selected_labels = select_repos(
            "Select repositories (type to search):",
            choices,
            preselected=preselected,
        )

        if not selected_labels:
            console.print("No repositories selected. Exiting.")
            raise typer.Exit()

        selected_repos = [choice_to_path[label] for label in selected_labels]

    if no_input:
        branch_name = require(branch, "--branch", console)
    else:
        branch_name = inquirer.text(message="Enter the branch name:").execute()
        if not branch_name:
            console.print("Branch name cannot be empty.")
            raise typer.Exit(code=1)

    # Phase 1: Fetch & prune
    console.print(f"\n[bold blue]Preparing repositories...[/bold blue]")
    for repo_path in track(selected_repos, description="Fetching & pruning..."):
        try:
            subprocess.run(["git", "fetch"], cwd=repo_path, capture_output=True)
            subprocess.run(
                ["git", "remote", "set-head", "origin", "--auto"],
                cwd=repo_path, capture_output=True,
            )
            subprocess.run(["git", "worktree", "prune"], cwd=repo_path, capture_output=True)
        except Exception as e:
            console.print(f"[yellow]Warning fetching {repo_path.name}: {e}[/yellow]")

    # Phase 2: Configure worktree actions
    repo_actions: List[Dict[str, Any]] = []

    console.print(f"\n[bold blue]Configuring worktrees...[/bold blue]")

    cfg.worktree_dir.mkdir(parents=True, exist_ok=True)
    normalized_branch_dir = branch_name.replace("/", "-")
    is_single_repo = len(selected_repos) == 1

    for repo_path in selected_repos:
        repo_name = repo_path.name

        if is_single_repo:
            target_path = (cfg.worktree_dir / normalized_branch_dir).resolve()
        else:
            target_path = (cfg.worktree_dir / normalized_branch_dir / repo_name).resolve()

        if target_path.exists():
            console.print(f"[yellow]Skipping {repo_name}: {target_path} already exists.[/yellow]")
            continue

        # Check if branch exists
        local_exists = subprocess.run(
            ["git", "rev-parse", "--verify", branch_name],
            cwd=repo_path, capture_output=True,
        ).returncode == 0

        remote_exists = False
        if not local_exists:
            remote_exists = subprocess.run(
                ["git", "rev-parse", "--verify", f"origin/{branch_name}"],
                cwd=repo_path, capture_output=True,
            ).returncode == 0

        if local_exists or remote_exists:
            repo_actions.append({
                "type": "add",
                "repo_path": repo_path,
                "target_path": target_path,
                "repo_name": repo_name,
                "branch": branch_name,
            })
        else:
            # Branch needs creation — detect default branch
            detected = detect_default_branch(repo_path)
            if base:
                # An explicit --base (or a template's base) applies to every repo.
                base_branch = base
                console.print(f"  [dim]{repo_name}: creating from {base_branch}[/dim]")
            elif no_input:
                base_branch = detected
                if not base_branch:
                    console.print(
                        f"[bold red]Error:[/bold red] [{repo_name}] branch '{branch_name}' "
                        f"not found and no default branch detected. Pass --base."
                    )
                    raise typer.Exit(code=1)
                console.print(f"  [dim]{repo_name}: creating from {base_branch}[/dim]")
            elif yes and detected:
                base_branch = detected
                console.print(f"  [dim]{repo_name}: creating from {detected}[/dim]")
            elif detected:
                choices_list = [detected, "Other (manual input)"]
                base_branch = inquirer.select(
                    message=f"[{repo_name}] Branch '{branch_name}' not found. Create from:",
                    choices=choices_list,
                    default=detected,
                ).execute()
                if base_branch == "Other (manual input)":
                    base_branch = inquirer.text(message=f"[{repo_name}] Enter base branch:").execute()
            else:
                base_branch = inquirer.text(
                    message=f"[{repo_name}] Branch '{branch_name}' not found. Enter base branch:",
                ).execute()

            repo_actions.append({
                "type": "create",
                "repo_path": repo_path,
                "target_path": target_path,
                "repo_name": repo_name,
                "branch": branch_name,
                "base": base_branch,
            })

    if not repo_actions:
        console.print("[yellow]No actions to perform. Exiting.[/yellow]")
        raise typer.Exit()

    # Confirm
    console.print(f"\n[bold]Plan:[/bold]")
    for action in repo_actions:
        if action["type"] == "add":
            console.print(f"  {action['repo_name']}: [green]add worktree[/green] for '{action['branch']}'")
        else:
            console.print(f"  {action['repo_name']}: [blue]create branch[/blue] '{action['branch']}' from '{action['base']}'")

    if not no_input and not inquirer.confirm(message="Proceed?", default=True).execute():
        console.print("Aborted.")
        raise typer.Exit()

    # Remember this repo set for next time.
    selected_names = [make_display_path(p, valid_dirs) for p in selected_repos]
    if not save_template and not no_input:
        save_template = _offer_save_template(selected_names, active_template)

    if save_template:
        saved_path, existed = upsert_template(
            merge_template(save_template, selected_names, base=base)
        )
        console.print(
            f"[green]{'Updated' if existed else 'Saved'} template "
            f"'{save_template}'[/green] [dim]({saved_path})[/dim]"
        )
        console.print(f"[dim]Reuse it with: spawnpoint create -t {save_template}[/dim]")

    # Phase 3: Execute
    for action in track(repo_actions, description="Creating worktrees..."):
        repo_name = action["repo_name"]
        repo_path = action["repo_path"]
        target_path = action["target_path"]
        action["status"] = "failed"

        console.print(f"Processing [bold]{repo_name}[/bold]...")

        try:
            target_path.parent.mkdir(parents=True, exist_ok=True)
            success = False

            if action["type"] == "add":
                cmd = ["git", "worktree", "add", str(target_path), action["branch"]]
                result = subprocess.run(cmd, cwd=repo_path, text=True, capture_output=True)
                if result.returncode == 0:
                    success = True
                    console.print(f"  [green]Worktree created.[/green]")
                else:
                    console.print(f"  [red]Failed:[/red] {result.stderr.strip()}")

            elif action["type"] == "create":
                # Resolve to remote tip if available (fetch already ran in Phase 1)
                start_point = action["base"]
                check = subprocess.run(
                    ["git", "rev-parse", "--verify", f"origin/{action['base']}"],
                    cwd=repo_path, capture_output=True,
                )
                if check.returncode == 0:
                    start_point = f"origin/{action['base']}"

                cmd = ["git", "worktree", "add", "--no-track", "-b", action["branch"], str(target_path), start_point]
                result = subprocess.run(cmd, cwd=repo_path, text=True, capture_output=True)
                if result.returncode == 0:
                    success = True
                    console.print(f"  [green]Created from {action['base']}.[/green]")
                else:
                    console.print(f"  [red]Failed:[/red] {result.stderr.strip()}")

            if success:
                action["status"] = "created"
                # Init submodules
                console.print(f"  [dim]Initializing submodules...[/dim]")
                subprocess.run(
                    ["git", "submodule", "update", "--init", "--recursive"],
                    cwd=target_path, capture_output=True,
                )

                copy_essential_files(repo_path, target_path, cfg)

                if cfg.auto_install_deps:
                    setup_dependencies(target_path)

        except Exception as e:
            console.print(f"[red]Error processing {repo_name}: {e}[/red]")

    console.print("\n[bold green]Done![/bold green]")

    if is_single_repo and repo_actions:
        workspace_path = repo_actions[0]['target_path']
    else:
        workspace_path = (cfg.worktree_dir / normalized_branch_dir).resolve()

    console.print(f"Workspace: [bold blue]{workspace_path}[/bold blue]")
    CD_PATH_FILE.write_text(str(workspace_path))

    if json_output:
        emit_json({
            "workspace": str(workspace_path),
            "branch": branch_name,
            "template": active_template.name if active_template else None,
            "repos": [
                {
                    "name": a["repo_name"],
                    "branch": a["branch"],
                    "base": a.get("base"),
                    "action": a["type"],
                    "status": a.get("status", "failed"),
                }
                for a in repo_actions
            ],
        })

    elif no_input:
        # Plain stdout so agents can capture the workspace path via $(...).
        # Skipped when --json is set so stdout stays valid JSON.
        from .io import stdout_console
        stdout_console.print(str(workspace_path))
