import subprocess
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple

import typer
from InquirerPy import inquirer
from InquirerPy.prompts.fuzzy import FuzzyPrompt
from rich.console import Console
from rich.progress import track

from .config import Config
from .utils import (
    copy_essential_files,
    detect_default_branch,
    find_git_repos,
    make_display_path,
    setup_dependencies,
)

console = Console(stderr=True)


class ClearOnToggleFuzzyPrompt(FuzzyPrompt):
    """FuzzyPrompt that clears the search buffer when toggling a choice with Tab."""

    def _handle_toggle_choice(self, _) -> None:
        super()._handle_toggle_choice(_)
        self._buffer.reset()


def _detect_workspace(cfg: Config) -> Optional[Tuple[Path, str]]:
    """Detect if cwd is inside a spawnpoint workspace.

    Returns (workspace_dir, branch_name) or None.
    Workspace dir is the branch folder (e.g. worktree_dir/<branch>/).
    """
    cwd = Path.cwd().resolve()

    all_worktree_dirs = [cfg.worktree_dir] + [
        d for d in cfg.additional_worktree_dirs if d != cfg.worktree_dir
    ]

    for worktree_dir in all_worktree_dirs:
        worktree_dir = worktree_dir.resolve()
        if not worktree_dir.exists():
            continue

        # Check if cwd is under worktree_dir
        try:
            rel = cwd.relative_to(worktree_dir)
        except ValueError:
            continue

        # The first component of the relative path is the branch folder
        parts = rel.parts
        if not parts:
            continue

        workspace_dir = worktree_dir / parts[0]
        if not workspace_dir.is_dir():
            continue

        # Infer branch name from an existing worktree in this workspace
        branch_name = _infer_branch_name(workspace_dir)
        if branch_name:
            return workspace_dir, branch_name

    return None


def _infer_branch_name(workspace_dir: Path) -> Optional[str]:
    """Get branch name from existing worktrees in a workspace."""
    # Check if workspace_dir itself is a worktree (single-repo case)
    if (workspace_dir / ".git").is_file():
        result = subprocess.run(
            ["git", "rev-parse", "--abbrev-ref", "HEAD"],
            cwd=workspace_dir, capture_output=True, text=True,
        )
        if result.returncode == 0:
            return result.stdout.strip()

    # Multi-repo: check subdirectories
    for sub in sorted(workspace_dir.iterdir()):
        if sub.is_dir() and (sub / ".git").is_file():
            result = subprocess.run(
                ["git", "rev-parse", "--abbrev-ref", "HEAD"],
                cwd=sub, capture_output=True, text=True,
            )
            if result.returncode == 0:
                return result.stdout.strip()

    return None


def _existing_repo_names(workspace_dir: Path) -> set[str]:
    """Get names of repos already in this workspace."""
    names = set()

    # Single-repo workspace: the workspace dir itself is a worktree
    if (workspace_dir / ".git").is_file():
        names.add(workspace_dir.name)

    # Multi-repo: subdirectories that are worktrees
    for sub in workspace_dir.iterdir():
        if sub.is_dir() and (sub / ".git").is_file():
            names.add(sub.name)

    return names


def run_add(cfg: Config):
    """Add repos to an existing spawnpoint workspace."""
    detected = _detect_workspace(cfg)

    if not detected:
        console.print("[bold red]Error:[/bold red] Not inside a spawnpoint workspace.")
        console.print("Run [bold]spawnpoint create[/bold] to create a new workspace,")
        console.print("or cd into an existing workspace first.")
        raise typer.Exit(code=1)

    workspace_dir, branch_name = detected
    existing_names = _existing_repo_names(workspace_dir)

    console.print(f"[bold blue]Workspace:[/bold blue] {workspace_dir.name}")
    console.print(f"[bold blue]Branch:[/bold blue] {branch_name}")
    if existing_names:
        console.print(f"[dim]Already present: {', '.join(sorted(existing_names))}[/dim]")

    # Scan for repos
    valid_dirs = [d for d in cfg.scan_dirs if d.is_dir()]
    if not valid_dirs:
        console.print("[bold red]Error:[/bold red] No valid scan directories configured.")
        raise typer.Exit(code=1)

    console.print(f"\n[bold blue]Scanning for git repositories...[/bold blue]")
    repos = find_git_repos(valid_dirs, cfg.scan_depth)

    # Filter out repos already in the workspace
    repos = [r for r in repos if r.name not in existing_names]

    if not repos:
        console.print("[yellow]No additional repositories found to add.[/yellow]")
        raise typer.Exit()

    choices = [make_display_path(repo, valid_dirs) for repo in repos]
    choice_to_path = dict(zip(choices, repos))

    selected_labels = ClearOnToggleFuzzyPrompt(
        message="Select repositories to add (type to search):",
        choices=choices,
        multiselect=True,
    ).execute()

    if not selected_labels:
        console.print("No repositories selected. Exiting.")
        raise typer.Exit()

    selected_repos = [choice_to_path[label] for label in selected_labels]

    # If existing workspace was single-repo, we need to restructure it to multi-repo
    needs_restructure = (workspace_dir / ".git").is_file()

    # Fetch & prune
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

    # Configure actions
    repo_actions: List[Dict[str, Any]] = []

    for repo_path in selected_repos:
        repo_name = repo_path.name
        target_path = (workspace_dir / repo_name).resolve()

        if target_path.exists():
            console.print(f"[yellow]Skipping {repo_name}: already exists in workspace.[/yellow]")
            continue

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
            detected_default = detect_default_branch(repo_path)
            if detected_default:
                choices_list = [detected_default, "Other (manual input)"]
                base_branch = inquirer.select(
                    message=f"[{repo_name}] Branch '{branch_name}' not found. Create from:",
                    choices=choices_list,
                    default=detected_default,
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

    # Show plan
    if needs_restructure:
        console.print(f"\n[bold yellow]Note:[/bold yellow] Workspace will be converted from single-repo to multi-repo layout.")
        console.print(f"  The existing worktree will be moved into {workspace_dir.name}/{existing_names.pop()}/")

    console.print(f"\n[bold]Plan:[/bold]")
    for action in repo_actions:
        if action["type"] == "add":
            console.print(f"  {action['repo_name']}: [green]add worktree[/green] for '{action['branch']}'")
        else:
            console.print(f"  {action['repo_name']}: [blue]create branch[/blue] '{action['branch']}' from '{action['base']}'")

    if not inquirer.confirm(message="Proceed?", default=True).execute():
        console.print("Aborted.")
        raise typer.Exit()

    # Restructure single-repo workspace to multi-repo if needed
    if needs_restructure:
        _restructure_to_multi_repo(workspace_dir)

    # Execute
    for action in track(repo_actions, description="Adding repos..."):
        repo_name = action["repo_name"]
        repo_path = action["repo_path"]
        target_path = action["target_path"]

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
                cmd = ["git", "worktree", "add", "-b", action["branch"], str(target_path), action["base"]]
                result = subprocess.run(cmd, cwd=repo_path, text=True, capture_output=True)
                if result.returncode == 0:
                    success = True
                    console.print(f"  [green]Created from {action['base']}.[/green]")
                else:
                    console.print(f"  [red]Failed:[/red] {result.stderr.strip()}")

            if success:
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

    console.print(f"\n[bold green]Done![/bold green]")
    console.print(f"Workspace: [bold blue]{workspace_dir}[/bold blue]")


def _restructure_to_multi_repo(workspace_dir: Path):
    """Convert a single-repo workspace into a multi-repo layout.

    Moves worktree_dir/<branch>/ contents into worktree_dir/<branch>/<repo_name>/.
    The repo_name is inferred from the git remote or the parent repo.
    """
    import shutil
    import tempfile

    # Read the .git file to find the parent repo name
    git_file = workspace_dir / ".git"
    content = git_file.read_text().strip()
    gitdir = Path(content.split("gitdir:", 1)[1].strip())
    if not gitdir.is_absolute():
        gitdir = (workspace_dir / gitdir).resolve()

    # gitdir is like /path/to/repo/.git/worktrees/<name>
    parent_repo = gitdir.parent.parent.parent
    repo_name = parent_repo.name

    console.print(f"  [dim]Restructuring: moving existing worktree into {repo_name}/[/dim]")

    # Move workspace contents to a temp dir, then back into a subdirectory
    tmp = Path(tempfile.mkdtemp(dir=workspace_dir.parent))
    try:
        # Move everything from workspace to temp
        for item in workspace_dir.iterdir():
            shutil.move(str(item), str(tmp / item.name))

        # Create the subdirectory and move everything back
        sub_dir = workspace_dir / repo_name
        sub_dir.mkdir()
        for item in tmp.iterdir():
            shutil.move(str(item), str(sub_dir / item.name))
    finally:
        if tmp.exists():
            shutil.rmtree(tmp, ignore_errors=True)

    # Update the .git file's gitdir path since the worktree moved
    new_git_file = sub_dir / ".git"
    old_content = new_git_file.read_text().strip()
    old_gitdir_str = old_content.split("gitdir:", 1)[1].strip()
    old_gitdir = Path(old_gitdir_str)

    if not old_gitdir.is_absolute():
        # Relative path needs updating: was relative to workspace_dir, now relative to sub_dir
        abs_gitdir = (workspace_dir / old_gitdir).resolve()
        # Make it absolute to be safe
        new_git_file.write_text(f"gitdir: {abs_gitdir}\n")
    # If absolute, no change needed

    # Also update the worktree config in the parent repo's .git/worktrees/<name>/gitdir
    gitdir_resolved = gitdir if gitdir.is_absolute() else (workspace_dir / gitdir).resolve()
    gitdir_file = gitdir_resolved / "gitdir"
    if gitdir_file.exists():
        gitdir_file.write_text(str(sub_dir / ".git") + "\n")

    console.print(f"  [green]Restructured successfully.[/green]")
