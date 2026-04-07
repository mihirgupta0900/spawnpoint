import time
from pathlib import Path
from typing import List

import typer
from InquirerPy import inquirer
from rich.console import Console
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


def run_list(cfg: Config, cd: bool = False):
    """List all worktree workspaces, optionally selecting one to cd into."""
    start = time.monotonic()
    folders = _collect_folders(cfg)
    logger.debug("Scan completed in %.2fs, found %d workspaces", time.monotonic() - start, len(folders))

    if not folders:
        console.print("[yellow]No workspaces found.[/yellow]")
        raise typer.Exit()

    # Sort newest-first for listing
    folders.sort(key=lambda bf: bf.oldest_modified, reverse=True)

    if not cd:
        # Just print a table
        table = Table(show_header=True, header_style="bold")
        table.add_column("Workspace")
        table.add_column("Repos", justify="right")
        table.add_column("Branch")
        table.add_column("Status")
        table.add_column("Last Modified")

        for bf in folders:
            repo_count = str(len(bf.worktrees))
            branches = set(w.branch_name for w in bf.worktrees)
            branch_str = ", ".join(sorted(branches))
            dirty_str = "[yellow]dirty[/yellow]" if bf.any_dirty else "[green]clean[/green]"
            age = _format_age(bf.oldest_modified)

            table.add_row(bf.name, repo_count, branch_str, dirty_str, age)

        console.print(table)
        return

    # Interactive selection for cd
    folder_map = {}
    for bf in folders:
        repo_count = len(bf.worktrees)
        dirty_str = "dirty" if bf.any_dirty else "clean"
        age = _format_age(bf.oldest_modified)
        repo_label = "repo" if repo_count == 1 else "repos"
        label = f"{bf.name}  ({repo_count} {repo_label}, {dirty_str}, {age})"
        folder_map[label] = bf

    selected_label = inquirer.fuzzy(
        message="Select workspace:",
        choices=list(folder_map.keys()),
    ).execute()

    if not selected_label:
        raise typer.Exit()

    bf = folder_map[selected_label]
    CD_PATH_FILE.write_text(str(bf.path))
