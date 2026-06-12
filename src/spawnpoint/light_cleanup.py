import os
import shutil
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, field
from pathlib import Path
from typing import List, Optional

import typer
from InquirerPy import inquirer
from rich.console import Console

from .cleanup import BranchFolder, _scan_work_dir
from .config import Config
from .io import emit_json, parse_csv, resolve_names
from .log import logger

console = Console()

# Directories that are safe to delete and can be reinstalled
ARTIFACT_TARGETS = [
    ("node_modules",  "Node.js dependencies"),
    (".venv",         "Python virtualenv (.venv)"),
    ("venv",          "Python virtualenv (venv)"),
    ("env",           "Python virtualenv (env)"),
    (".tox",          "Tox test environments"),
    ("__pycache__",   "Python bytecode cache"),
    (".pytest_cache", "Pytest cache"),
    ("dist",          "Build output (dist)"),
    ("build",         "Build output (build)"),
    (".next",         "Next.js build cache"),
    (".nuxt",         "Nuxt.js build cache"),
    (".turbo",        "Turborepo cache"),
    ("target",        "Rust/Java build artifacts (target)"),
    (".gradle",       "Gradle cache"),
    (".parcel-cache", "Parcel build cache"),
    (".cache",        "Generic cache directory"),
]

ARTIFACT_NAMES = {name for name, _ in ARTIFACT_TARGETS}


def _dir_size_bytes(path: Path) -> int:
    total = 0
    try:
        for dirpath, _dirs, files in os.walk(path):
            for f in files:
                try:
                    total += os.path.getsize(os.path.join(dirpath, f))
                except OSError:
                    pass
    except OSError:
        pass
    return total


def _fmt_bytes(n: int) -> str:
    for unit in ("B", "KB", "MB", "GB"):
        if n < 1024:
            return f"{n:.1f} {unit}"
        n /= 1024
    return f"{n:.1f} TB"


@dataclass
class ArtifactDir:
    path: Path
    artifact_name: str
    size_bytes: int = 0


@dataclass
class WorkspaceArtifacts:
    folder: BranchFolder
    artifacts: List[ArtifactDir] = field(default_factory=list)

    @property
    def total_size(self) -> int:
        return sum(a.size_bytes for a in self.artifacts)

    @property
    def artifact_names(self) -> set[str]:
        return {a.artifact_name for a in self.artifacts}


def _find_artifacts_in_path(root: Path) -> List[ArtifactDir]:
    """Find all reinstallable artifact dirs under root (non-recursive past first hit)."""
    found: List[ArtifactDir] = []
    _walk(root, found)
    return found


def _walk(path: Path, found: List[ArtifactDir], depth: int = 0):
    if depth > 6:
        return
    try:
        for child in path.iterdir():
            if not child.is_dir() or child.is_symlink():
                continue
            if child.name in ARTIFACT_NAMES:
                found.append(ArtifactDir(path=child, artifact_name=child.name))
                # don't recurse into artifact dirs
            else:
                _walk(child, found, depth + 1)
    except PermissionError:
        pass


def _scan_workspace_artifacts(folder: BranchFolder) -> WorkspaceArtifacts:
    artifacts: List[ArtifactDir] = []
    for wt in folder.worktrees:
        artifacts.extend(_find_artifacts_in_path(wt.worktree_path))
    ws = WorkspaceArtifacts(folder=folder, artifacts=artifacts)
    return ws


def _compute_sizes(ws: WorkspaceArtifacts) -> None:
    with ThreadPoolExecutor(max_workers=8) as pool:
        futures = {pool.submit(_dir_size_bytes, a.path): a for a in ws.artifacts}
        for fut, art in futures.items():
            art.size_bytes = fut.result()


def run_light_cleanup(
    cfg: Config,
    *,
    no_input: bool = False,
    workspaces: str | None = None,
    artifact_types: str | None = None,
    json_output: bool = False,
):
    """Delete reinstallable artifact directories (node_modules, .venv, etc.) from workspaces."""
    work_dirs = [cfg.worktree_dir] + [
        d for d in cfg.additional_worktree_dirs if d != cfg.worktree_dir
    ]
    existing_dirs = [d for d in work_dirs if d.exists()]

    if not existing_dirs:
        console.print("[yellow]No workspaces directory found.[/yellow]")
        raise typer.Exit()

    console.print("[bold blue]Scanning for worktree workspaces...[/bold blue]")

    folders: list[BranchFolder] = []
    seen_paths: set[Path] = set()
    for work_dir in existing_dirs:
        for bf in _scan_work_dir(work_dir):
            resolved = bf.path.resolve()
            if resolved not in seen_paths:
                seen_paths.add(resolved)
                folders.append(bf)

    if not folders:
        console.print("[yellow]No worktree workspaces found.[/yellow]")
        raise typer.Exit()

    folders.sort(key=lambda bf: bf.oldest_modified)

    # --- Select workspaces ---
    if no_input:
        if not workspaces:
            console.print("[bold red]Error:[/bold red] --no-input requires --workspaces.")
            raise typer.Exit(code=1)
        name_to_bf = {bf.name: bf for bf in folders}
        requested = parse_csv(workspaces)
        selected_folders = resolve_names(requested, name_to_bf, kind="workspace", err=console)
    else:
        folder_choices = [bf.name for bf in folders]
        selected_names = inquirer.fuzzy(
            message="Select workspaces for light cleanup (type to search):",
            choices=folder_choices,
            multiselect=True,
        ).execute()

        if not selected_names:
            console.print("No workspaces selected. Exiting.")
            raise typer.Exit()

        name_to_bf = {bf.name: bf for bf in folders}
        selected_folders = [name_to_bf[n] for n in selected_names]

    # --- Scan artifacts ---
    console.print("[dim]Scanning for artifact directories...[/dim]")
    workspace_artifacts: list[WorkspaceArtifacts] = []
    for bf in selected_folders:
        ws = _scan_workspace_artifacts(bf)
        if ws.artifacts:
            workspace_artifacts.append(ws)

    if not workspace_artifacts:
        console.print("[green]No reinstallable artifact directories found.[/green]")
        raise typer.Exit()

    # Compute sizes in parallel per workspace
    console.print("[dim]Calculating sizes...[/dim]")
    with ThreadPoolExecutor(max_workers=4) as pool:
        list(pool.map(_compute_sizes, workspace_artifacts))

    # --- Select artifact types to delete ---
    all_found_types: set[str] = set()
    for ws in workspace_artifacts:
        all_found_types |= ws.artifact_names

    if no_input:
        if artifact_types:
            requested_types = set(parse_csv(artifact_types))
        else:
            # default: all found types
            requested_types = all_found_types
        selected_types = all_found_types & requested_types
    else:
        type_choices = []
        for name, desc in ARTIFACT_TARGETS:
            if name not in all_found_types:
                continue
            total = sum(
                a.size_bytes
                for ws in workspace_artifacts
                for a in ws.artifacts
                if a.artifact_name == name
            )
            type_choices.append(f"{name}  ({desc})  [{_fmt_bytes(total)}]")

        selected_type_labels = inquirer.checkbox(
            message="Which artifact types to delete?",
            choices=type_choices,
            default=type_choices,
        ).execute()

        if not selected_type_labels:
            console.print("Nothing selected. Exiting.")
            raise typer.Exit()

        selected_types = {label.split()[0] for label in selected_type_labels}

    # Filter artifacts down to selected types
    for ws in workspace_artifacts:
        ws.artifacts = [a for a in ws.artifacts if a.artifact_name in selected_types]
    workspace_artifacts = [ws for ws in workspace_artifacts if ws.artifacts]

    if not workspace_artifacts:
        console.print("[yellow]No matching artifacts after filtering.[/yellow]")
        raise typer.Exit()

    # --- Show plan ---
    total_bytes = sum(ws.total_size for ws in workspace_artifacts)
    console.print(f"\n[bold]Plan[/bold] — will free ~{_fmt_bytes(total_bytes)}:\n")
    for ws in workspace_artifacts:
        console.print(f"  [bold]{ws.folder.name}/[/bold]  (~{_fmt_bytes(ws.total_size)})")
        for art in sorted(ws.artifacts, key=lambda a: -a.size_bytes):
            rel = art.path.relative_to(ws.folder.path)
            console.print(f"    [dim]{rel}[/dim]  {_fmt_bytes(art.size_bytes)}")
    console.print("")

    if not no_input and not inquirer.confirm(
        message="Proceed with light cleanup?",
        default=True,
    ).execute():
        console.print("Aborted.")
        raise typer.Exit()

    # --- Execute ---
    deleted_report: list[dict] = []
    for ws in workspace_artifacts:
        ws_report: list[dict] = []
        for art in ws.artifacts:
            try:
                shutil.rmtree(art.path)
                logger.debug("Deleted %s", art.path)
                ws_report.append({"path": str(art.path), "size_bytes": art.size_bytes, "ok": True})
                console.print(f"  [green]✓[/green] {art.path.relative_to(ws.folder.path)}  ({_fmt_bytes(art.size_bytes)})")
            except Exception as exc:
                console.print(f"  [red]✗[/red] {art.path.relative_to(ws.folder.path)}: {exc}")
                ws_report.append({"path": str(art.path), "size_bytes": art.size_bytes, "ok": False, "error": str(exc)})
        deleted_report.append({"workspace": ws.folder.name, "artifacts": ws_report})

    freed = sum(
        a["size_bytes"] for ws in deleted_report for a in ws["artifacts"] if a["ok"]
    )
    console.print(f"\n[bold green]Done![/bold green] Freed ~{_fmt_bytes(freed)}.")

    if json_output:
        emit_json({"freed_bytes": freed, "workspaces": deleted_report})
