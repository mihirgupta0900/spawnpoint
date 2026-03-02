import subprocess
from pathlib import Path
from typing import Any, Dict, List

import typer
from InquirerPy import inquirer
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

console = Console()


def run_create(cfg: Config):
    """Select git repos and create worktrees for a feature branch."""
    if not cfg.scan_dirs:
        console.print("[bold red]Error:[/bold red] No scan directories configured.")
        console.print("Run [bold]spawnpoint init[/bold] to set up.")
        raise typer.Exit(code=1)

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

    repos = find_git_repos(valid_dirs, cfg.scan_depth)

    if not repos:
        console.print("[yellow]No git repositories found.[/yellow]")
        raise typer.Exit()

    # Display paths relative to scan dirs for readability
    choices = [make_display_path(repo, valid_dirs) for repo in repos]
    choice_to_path = dict(zip(choices, repos))

    selected_labels = inquirer.fuzzy(
        message="Select repositories (type to search):",
        choices=choices,
        multiselect=True,
    ).execute()

    if not selected_labels:
        console.print("No repositories selected. Exiting.")
        raise typer.Exit()

    branch_name = inquirer.text(message="Enter the branch name:").execute()

    if not branch_name:
        console.print("Branch name cannot be empty.")
        raise typer.Exit(code=1)

    selected_repos = [choice_to_path[label] for label in selected_labels]

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
            if detected:
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

    if not inquirer.confirm(message="Proceed?", default=True).execute():
        console.print("Aborted.")
        raise typer.Exit()

    # Phase 3: Execute
    for action in track(repo_actions, description="Creating worktrees..."):
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
    console.print(f"\n[dim]cd {workspace_path}[/dim]")
