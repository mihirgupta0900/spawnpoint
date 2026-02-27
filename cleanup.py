import shutil
import subprocess
from dataclasses import dataclass, field
from datetime import datetime, timezone
from pathlib import Path
from typing import List, Optional

import typer
from InquirerPy import inquirer
from rich.console import Console
from rich.progress import track

app = typer.Typer()
console = Console()

HOME = Path.home()
WORK_DIRS = [
    HOME / "code" / "work" / "worktrees",
    HOME / "code" / "work",
]


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


def parse_git_file(path: Path) -> Optional[Path]:
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
        # Walk up 3 levels: <name> -> worktrees -> .git -> repo
        parent_repo = gitdir.parent.parent.parent
        if (parent_repo / ".git").exists():
            return parent_repo
    except Exception:
        pass
    return None


def get_branch_name(path: Path) -> str:
    result = subprocess.run(
        ["git", "rev-parse", "--abbrev-ref", "HEAD"],
        cwd=path, capture_output=True, text=True,
    )
    return result.stdout.strip() if result.returncode == 0 else "unknown"


def is_dirty(path: Path) -> bool:
    result = subprocess.run(
        ["git", "status", "--porcelain"],
        cwd=path, capture_output=True, text=True,
    )
    return bool(result.stdout.strip()) if result.returncode == 0 else False


def get_mtime(path: Path) -> datetime:
    return datetime.fromtimestamp(path.stat().st_mtime, tz=timezone.utc)


def scan_worktree(path: Path) -> Optional[WorktreeInfo]:
    """Scan a single directory that has a .git file (worktree)."""
    parent_repo = parse_git_file(path)
    if parent_repo is None:
        # .git file broken or not a worktree
        return None
    return WorktreeInfo(
        worktree_path=path,
        parent_repo_path=parent_repo,
        branch_name=get_branch_name(path),
        is_dirty=is_dirty(path),
        last_modified=get_mtime(path),
    )


def scan_single_dir(work_dir: Path) -> List[BranchFolder]:
    """Scan a single work directory for branch folders containing worktrees."""
    if not work_dir.exists():
        return []

    folders: List[BranchFolder] = []
    seen_paths: set[Path] = set()

    for entry in sorted(work_dir.iterdir()):
        if not entry.is_dir():
            continue
        resolved = entry.resolve()
        if resolved in seen_paths:
            continue
        seen_paths.add(resolved)

        bf = BranchFolder(path=entry, name=entry.name)

        git_file = entry / ".git"
        if git_file.is_file():
            # Single-repo worktree
            wt = scan_worktree(entry)
            if wt:
                bf.worktrees.append(wt)
        else:
            # Multi-repo: check subdirectories
            for sub in sorted(entry.iterdir()):
                if sub.is_dir() and (sub / ".git").is_file():
                    wt = scan_worktree(sub)
                    if wt:
                        bf.worktrees.append(wt)

        if bf.worktrees:
            folders.append(bf)

    return folders


def scan_branch_folders() -> List[BranchFolder]:
    """Scan all WORK_DIRS for branch folders containing worktrees."""
    all_folders: List[BranchFolder] = []
    seen_paths: set[Path] = set()

    for work_dir in WORK_DIRS:
        for bf in scan_single_dir(work_dir):
            resolved = bf.path.resolve()
            if resolved in seen_paths:
                continue
            seen_paths.add(resolved)
            all_folders.append(bf)

    return all_folders


def format_age(dt: datetime) -> str:
    delta = datetime.now(tz=timezone.utc) - dt
    days = delta.days
    if days == 0:
        hours = delta.seconds // 3600
        return f"{hours}h ago" if hours > 0 else "just now"
    if days < 30:
        return f"{days}d ago"
    return f"{days // 30}mo ago"


def format_choice(bf: BranchFolder) -> str:
    repo_count = len(bf.worktrees)
    dirty_str = "dirty" if bf.any_dirty else "clean"
    age = format_age(bf.oldest_modified)
    repo_label = "repo" if repo_count == 1 else "repos"
    parent_dir = bf.path.parent.name
    return f"{bf.name}  ({parent_dir}/, {repo_count} {repo_label}, {dirty_str}, {age})"


def remove_worktree(wt: WorktreeInfo, delete_branch: bool):
    """Remove a single worktree and optionally its branch."""
    parent = wt.parent_repo_path

    if parent is None or not parent.exists():
        console.print(f"  [yellow]Parent repo gone, removing directory: {wt.worktree_path}[/yellow]")
        shutil.rmtree(wt.worktree_path, ignore_errors=True)
        return

    # git worktree remove
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
        # Fallback: remove directory
        if wt.worktree_path.exists():
            console.print(f"  [yellow]Falling back to rmtree[/yellow]")
            shutil.rmtree(wt.worktree_path, ignore_errors=True)

    # Delete branch if requested
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


@app.command()
def main():
    """Remove worktree workspaces created by the setup script."""
    existing_dirs = [d for d in WORK_DIRS if d.exists()]
    if not existing_dirs:
        console.print(f"[yellow]No worktree directories found[/yellow]")
        raise typer.Exit()

    dirs_str = ", ".join(str(d) for d in existing_dirs)
    console.print(f"[bold blue]Scanning for worktree workspaces...[/bold blue]")
    console.print(f"[dim]Directories: {dirs_str}[/dim]")
    folders = scan_branch_folders()

    if not folders:
        console.print("[yellow]No worktree workspaces found.[/yellow]")
        raise typer.Exit()

    # Sort oldest-first
    folders.sort(key=lambda bf: bf.oldest_modified)

    # Build lookup
    folder_map = {format_choice(bf): bf for bf in folders}

    selected_labels = inquirer.fuzzy(
        message="Select workspaces to remove (Type to search):",
        choices=list(folder_map.keys()),
        multiselect=True,
    ).execute()

    if not selected_labels:
        console.print("No workspaces selected. Exiting.")
        raise typer.Exit()

    selected = [folder_map[label] for label in selected_labels]

    # Branch deletion preference
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
                console.print(f"    {parent_label}: [dim]will ask about branch '{wt.branch_name}'[/dim]")

    console.print("")
    if not inquirer.confirm(message="Proceed with cleanup?").execute():
        console.print("Aborted.")
        raise typer.Exit()

    # Execute
    parent_repos_seen: set[Path] = set()

    for bf in track(selected, description="Cleaning up..."):
        # Per-branch "ask" decisions
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

            remove_worktree(wt, delete_branch)

            if wt.parent_repo_path and wt.parent_repo_path.exists():
                parent_repos_seen.add(wt.parent_repo_path)

        # Remove any leftover non-worktree dirs and the branch folder itself
        if bf.path.exists():
            shutil.rmtree(bf.path, ignore_errors=True)

    # Prune all affected parent repos
    for repo in parent_repos_seen:
        subprocess.run(
            ["git", "worktree", "prune"],
            cwd=repo, capture_output=True,
        )

    console.print("\n[bold green]Done![/bold green]")


if __name__ == "__main__":
    app()
