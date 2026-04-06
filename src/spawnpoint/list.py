import json
import re
import subprocess
import time
import tomllib
from pathlib import Path
from typing import List

import typer
from InquirerPy import inquirer
from rich.console import Console
from rich.panel import Panel
from rich.table import Table

from .cleanup import BranchFolder, _format_age, _scan_work_dir
from .config import CD_PATH_FILE, Config
from .log import logger

console = Console(stderr=True)


def _collect_folders(cfg: Config) -> List[BranchFolder]:
    """Scan all worktree dirs and return deduplicated folders."""
    work_dirs = [cfg.worktree_dir] + [
        d for d in cfg.additional_worktree_dirs if d != cfg.worktree_dir
    ]
    logger.debug("Worktree dirs to scan: %s", work_dirs)

    folders: List[BranchFolder] = []
    seen_paths: set[Path] = set()
    for work_dir in work_dirs:
        if not work_dir.exists():
            continue
        for bf in _scan_work_dir(work_dir):
            resolved = bf.path.resolve()
            if resolved not in seen_paths:
                seen_paths.add(resolved)
                folders.append(bf)

    return folders


def _read_description(workspace_path: Path) -> str:
    """Read description from .spawnpoint-meta file."""
    meta_path = workspace_path / ".spawnpoint-meta"
    if not meta_path.is_file():
        return ""
    try:
        with open(meta_path, "rb") as f:
            data = tomllib.load(f)
        return data.get("description", "")
    except Exception:
        return ""


def _repo_names(bf: BranchFolder) -> str:
    """Get comma-separated repo names for a workspace."""
    names = []
    for w in bf.worktrees:
        names.append(w.parent_repo_path.name if w.parent_repo_path else w.worktree_path.name)
    return ", ".join(sorted(names))


def _get_last_commit(worktree_path: Path) -> str:
    try:
        result = subprocess.run(
            ["git", "log", "-1", "--oneline"],
            cwd=worktree_path, capture_output=True, text=True, timeout=5,
        )
        return result.stdout.strip() if result.returncode == 0 else ""
    except Exception:
        return ""


def _parse_owner_repo(worktree_path: Path) -> str | None:
    """Parse owner/repo from git remote URL."""
    try:
        result = subprocess.run(
            ["git", "remote", "get-url", "origin"],
            cwd=worktree_path, capture_output=True, text=True, timeout=5,
        )
        if result.returncode != 0:
            return None
        url = result.stdout.strip()
        match = re.match(r"(?:git@[\w.-]+:|https?://[\w.-]+/)(.+?)(?:\.git)?$", url)
        if match:
            return match.group(1)
    except Exception:
        pass
    return None


def _get_open_prs(worktree_path: Path, branch_name: str) -> list[dict] | None:
    """Get open PRs for a branch. Returns list of dicts or None on error."""
    owner_repo = _parse_owner_repo(worktree_path)
    if not owner_repo:
        return None
    try:
        result = subprocess.run(
            ["gh", "pr", "list", "--head", branch_name, "--repo", owner_repo,
             "--json", "number,title,url", "--limit", "5"],
            capture_output=True, text=True, timeout=15,
        )
        if result.returncode == 0:
            return json.loads(result.stdout)
    except (subprocess.TimeoutExpired, FileNotFoundError, Exception):
        pass
    return None


def _show_detail(bf: BranchFolder):
    """Show expanded detail for a single workspace."""
    desc = _read_description(bf.path)

    lines = []
    if desc:
        lines.append(f"[bold]Description:[/bold] {desc}")
        lines.append("")

    lines.append("[bold]Repos:[/bold]")
    for w in bf.worktrees:
        repo_name = w.parent_repo_path.name if w.parent_repo_path else w.worktree_path.name
        dirty_tag = " [yellow](dirty)[/yellow]" if w.is_dirty else " [green](clean)[/green]"
        lines.append(f"  [bold]{repo_name}[/bold] on {w.branch_name}{dirty_tag}")

        commit = _get_last_commit(w.worktree_path)
        if commit:
            lines.append(f"    [dim]Last commit: {commit}[/dim]")

        prs = _get_open_prs(w.worktree_path, w.branch_name)
        if prs:
            for pr in prs:
                lines.append(f"    [cyan]PR #{pr['number']}:[/cyan] {pr['title']} ({pr['url']})")
        elif prs is not None:
            lines.append(f"    [dim]No open PRs[/dim]")

    panel = Panel("\n".join(lines), title=f"[bold]{bf.name}[/bold]", border_style="blue")
    console.print(panel)


def _print_table(folders: List[BranchFolder]):
    """Print the workspace overview table."""
    table = Table(show_header=True, header_style="bold")
    table.add_column("Workspace")
    table.add_column("Repos")
    table.add_column("Branch")
    table.add_column("Status")
    table.add_column("Description")
    table.add_column("Last Modified")

    for bf in folders:
        repos_str = _repo_names(bf)
        branches = set(w.branch_name for w in bf.worktrees)
        branch_str = ", ".join(sorted(branches))
        dirty_str = "[yellow]dirty[/yellow]" if bf.any_dirty else "[green]clean[/green]"
        age = _format_age(bf.oldest_modified)
        desc = _read_description(bf.path)

        table.add_row(bf.name, repos_str, branch_str, dirty_str, desc, age)

    console.print(table)


def _build_folder_map(folders: List[BranchFolder]) -> dict[str, BranchFolder]:
    folder_map = {}
    for bf in folders:
        repo_count = len(bf.worktrees)
        dirty_str = "dirty" if bf.any_dirty else "clean"
        age = _format_age(bf.oldest_modified)
        repo_label = "repo" if repo_count == 1 else "repos"
        label = f"{bf.name}  ({repo_count} {repo_label}, {dirty_str}, {age})"
        folder_map[label] = bf
    return folder_map


def run_list(cfg: Config, cd: bool = False, detail: bool = False):
    """List all worktree workspaces, optionally selecting one to cd into."""
    start = time.monotonic()
    folders = _collect_folders(cfg)
    logger.debug("Scan completed in %.2fs, found %d workspaces", time.monotonic() - start, len(folders))

    if not folders:
        console.print("[yellow]No workspaces found.[/yellow]")
        raise typer.Exit()

    # Sort newest-first for listing
    folders.sort(key=lambda bf: bf.oldest_modified, reverse=True)

    if not cd or detail:
        _print_table(folders)

    if not cd and not detail:
        return

    if detail:
        folder_map = _build_folder_map(folders)
        selected_label = inquirer.fuzzy(
            message="Select workspace for details:",
            choices=list(folder_map.keys()),
        ).execute()
        if selected_label:
            _show_detail(folder_map[selected_label])
        return

    # Interactive selection for cd
    folder_map = _build_folder_map(folders)
    selected_label = inquirer.fuzzy(
        message="Select workspace:",
        choices=list(folder_map.keys()),
    ).execute()

    if not selected_label:
        raise typer.Exit()

    bf = folder_map[selected_label]
    CD_PATH_FILE.write_text(str(bf.path))
