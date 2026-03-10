import shutil
import subprocess
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import List, Optional

import typer
from InquirerPy import inquirer
from rich.console import Console
from rich.progress import track

from .config import Config
from .log import logger

console = Console()


@dataclass
class WorktreeInfo:
    worktree_path: Path
    parent_repo_path: Optional[Path]
    branch_name: str
    is_dirty: bool
    last_modified: datetime


@dataclass
class BranchFolder:
    path: Path
    name: str
    worktrees: List[WorktreeInfo] = field(default_factory=list)

    @property
    def oldest_modified(self) -> datetime:
        if not self.worktrees:
            return datetime.fromtimestamp(self.path.stat().st_mtime, tz=timezone.utc)
        return min(w.last_modified for w in self.worktrees)

    @property
    def any_dirty(self) -> bool:
        return any(w.is_dirty for w in self.worktrees)


def _parse_git_file(path: Path) -> Optional[Path]:
    """Parse a .git file to extract the gitdir path and resolve the parent repo."""
    git_path = path / ".git"
    if not git_path.is_file():
        return None
    try:
        content = git_path.read_text().strip()
        if not content.startswith("gitdir:"):
            return None
        gitdir = Path(content.split("gitdir:", 1)[1].strip())
        if not gitdir.is_absolute():
            gitdir = (path / gitdir).resolve()
        # gitdir is like /path/to/repo/.git/worktrees/<name>
        parent_repo = gitdir.parent.parent.parent
        if (parent_repo / ".git").exists():
            return parent_repo
    except Exception:
        pass
    return None


def _get_branch_name(path: Path) -> str:
    result = subprocess.run(
        ["git", "rev-parse", "--abbrev-ref", "HEAD"],
        cwd=path, capture_output=True, text=True,
    )
    return result.stdout.strip() if result.returncode == 0 else "unknown"


def _is_dirty(path: Path) -> bool:
    result = subprocess.run(
        ["git", "status", "--porcelain"],
        cwd=path, capture_output=True, text=True,
    )
    return bool(result.stdout.strip()) if result.returncode == 0 else False


def _get_mtime(path: Path) -> datetime:
    return datetime.fromtimestamp(path.stat().st_mtime, tz=timezone.utc)


def _scan_worktree(path: Path) -> Optional[WorktreeInfo]:
    logger.debug("Scanning worktree: %s", path)
    parent_repo = _parse_git_file(path)
    if parent_repo is None:
        logger.debug("  Not a worktree (no .git file): %s", path)
        return None
    branch = _get_branch_name(path)
    dirty = _is_dirty(path)
    mtime = _get_mtime(path)
    logger.debug("  branch=%s dirty=%s parent=%s", branch, dirty, parent_repo)
    return WorktreeInfo(
        worktree_path=path,
        parent_repo_path=parent_repo,
        branch_name=branch,
        is_dirty=dirty,
        last_modified=mtime,
    )


def _scan_work_dir(work_dir: Path) -> List[BranchFolder]:
    if not work_dir.exists():
        return []

    logger.debug("Scanning work dir: %s", work_dir)

    # Collect all worktree paths to scan
    entries: List[tuple[Path, Path]] = []  # (branch_folder, worktree_path)
    seen_paths: set[Path] = set()

    for entry in sorted(work_dir.iterdir()):
        if not entry.is_dir():
            continue
        resolved = entry.resolve()
        if resolved in seen_paths:
            continue
        seen_paths.add(resolved)

        git_file = entry / ".git"
        if git_file.is_file():
            entries.append((entry, entry))
        else:
            for sub in sorted(entry.iterdir()):
                if sub.is_dir() and (sub / ".git").is_file():
                    entries.append((entry, sub))

    logger.debug("Found %d worktree paths to scan", len(entries))

    # Scan all worktrees in parallel
    results: dict[Path, List[WorktreeInfo]] = {}
    with ThreadPoolExecutor(max_workers=8) as pool:
        futures = {pool.submit(_scan_worktree, wt_path): (bf_path, wt_path) for bf_path, wt_path in entries}
        for future in futures:
            bf_path, wt_path = futures[future]
            wt = future.result()
            if wt:
                results.setdefault(bf_path, []).append(wt)

    folders: List[BranchFolder] = []
    for bf_path, worktrees in results.items():
        bf = BranchFolder(path=bf_path, name=bf_path.name, worktrees=worktrees)
        folders.append(bf)

    logger.debug("Found %d workspaces with worktrees", len(folders))
    return folders


def _format_age(dt: datetime) -> str:
    delta = datetime.now(tz=timezone.utc) - dt
    days = delta.days
    if days == 0:
        hours = delta.seconds // 3600
        return f"{hours}h ago" if hours > 0 else "just now"
    if days < 30:
        return f"{days}d ago"
    return f"{days // 30}mo ago"


def _format_choice(bf: BranchFolder) -> str:
    repo_count = len(bf.worktrees)
    dirty_str = "dirty" if bf.any_dirty else "clean"
    age = _format_age(bf.oldest_modified)
    repo_label = "repo" if repo_count == 1 else "repos"
    return f"{bf.name}  ({repo_count} {repo_label}, {dirty_str}, {age})"


def _remove_worktree(wt: WorktreeInfo, delete_branch: bool):
    parent = wt.parent_repo_path

    if parent is None or not parent.exists():
        console.print(f"  [yellow]Parent repo gone, removing directory: {wt.worktree_path}[/yellow]")
        shutil.rmtree(wt.worktree_path, ignore_errors=True)
        return

    cmd = ["git", "worktree", "remove", str(wt.worktree_path)]
    if wt.is_dirty:
        cmd.append("--force")
        console.print(f"  [yellow]Force-removing dirty worktree:[/yellow] {wt.worktree_path.name}")
    else:
        console.print(f"  Removing worktree: {wt.worktree_path.name}")

    result = subprocess.run(cmd, cwd=parent, capture_output=True, text=True)
    if result.returncode != 0:
        err = result.stderr.strip()
        console.print(f"  [red]worktree remove failed: {err}[/red]")
        if wt.worktree_path.exists():
            console.print(f"  [yellow]Falling back to rmtree[/yellow]")
            shutil.rmtree(wt.worktree_path, ignore_errors=True)

    if delete_branch and wt.branch_name not in ("unknown", "HEAD"):
        result = subprocess.run(
            ["git", "branch", "-d", wt.branch_name],
            cwd=parent, capture_output=True, text=True,
        )
        if result.returncode != 0:
            err = result.stderr.strip()
            if "not fully merged" in err:
                console.print(f"  [yellow]Branch '{wt.branch_name}' not merged, force-deleting[/yellow]")
                subprocess.run(
                    ["git", "branch", "-D", wt.branch_name],
                    cwd=parent, capture_output=True, text=True,
                )
            elif "not found" not in err:
                console.print(f"  [dim]Branch delete skipped: {err}[/dim]")


def run_cleanup(cfg: Config):
    """Remove worktree workspaces."""
    work_dirs = [cfg.worktree_dir] + [
        d for d in cfg.additional_worktree_dirs if d != cfg.worktree_dir
    ]
    existing_dirs = [d for d in work_dirs if d.exists()]

    if not existing_dirs:
        console.print("[yellow]No workspaces directory found.[/yellow]")
        raise typer.Exit()

    console.print("[bold blue]Scanning for worktree workspaces...[/bold blue]")
    for d in existing_dirs:
        console.print(f"[dim]{d}[/dim]")

    folders: List[BranchFolder] = []
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

    # Sort oldest-first
    folders.sort(key=lambda bf: bf.oldest_modified)

    folder_map = {_format_choice(bf): bf for bf in folders}

    selected_labels = inquirer.fuzzy(
        message="Select workspaces to remove (type to search):",
        choices=list(folder_map.keys()),
        multiselect=True,
    ).execute()

    if not selected_labels:
        console.print("No workspaces selected. Exiting.")
        raise typer.Exit()

    selected = [folder_map[label] for label in selected_labels]

    branch_pref = inquirer.select(
        message="Delete branches from parent repos?",
        choices=["Delete all branches", "Keep branches", "Ask per branch"],
    ).execute()

    # Show plan
    console.print(f"\n[bold]Plan:[/bold]")
    for bf in selected:
        console.print(f"\n  [bold]{bf.name}/[/bold]")
        for wt in bf.worktrees:
            dirty_tag = " [yellow](dirty — will force)[/yellow]" if wt.is_dirty else ""
            parent_label = wt.parent_repo_path.name if wt.parent_repo_path else "unknown"
            console.print(f"    {parent_label}: git worktree remove{dirty_tag}")
            if branch_pref == "Delete all branches":
                console.print(f"    {parent_label}: git branch -d {wt.branch_name}")
            elif branch_pref == "Ask per branch":
                console.print(f"    {parent_label}: [dim]will ask about '{wt.branch_name}'[/dim]")

    console.print("")
    if not inquirer.confirm(message="Proceed with cleanup?").execute():
        console.print("Aborted.")
        raise typer.Exit()

    # Execute
    parent_repos_seen: set[Path] = set()

    for bf in track(selected, description="Cleaning up..."):
        branch_decisions: dict[str, bool] = {}

        for wt in bf.worktrees:
            delete_branch = False
            if branch_pref == "Delete all branches":
                delete_branch = True
            elif branch_pref == "Ask per branch":
                key = f"{wt.parent_repo_path}:{wt.branch_name}"
                if key not in branch_decisions:
                    branch_decisions[key] = inquirer.confirm(
                        message=f"Delete branch '{wt.branch_name}' from {wt.parent_repo_path.name}?",
                        default=True,
                    ).execute()
                delete_branch = branch_decisions[key]

            _remove_worktree(wt, delete_branch)

            if wt.parent_repo_path and wt.parent_repo_path.exists():
                parent_repos_seen.add(wt.parent_repo_path)

        # Remove leftover directories
        if bf.path.exists():
            shutil.rmtree(bf.path, ignore_errors=True)

    # Prune all affected parent repos
    for repo in parent_repos_seen:
        subprocess.run(["git", "worktree", "prune"], cwd=repo, capture_output=True)

    console.print("\n[bold green]Done![/bold green]")
