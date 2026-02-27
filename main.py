import os
import shutil
import subprocess
from pathlib import Path
from typing import List, Dict, Any

import typer
from InquirerPy import inquirer
from rich.console import Console
from rich.progress import track

app = typer.Typer()
console = Console()

HOME = Path.home()
CODE_DIR = HOME / "code"
WORK_DIR = CODE_DIR / "work" / "worktrees"

def is_git_repo(path: Path) -> bool:
    return (path / ".git").exists()

def find_git_repos(root_dir: Path, max_depth: int = 2) -> List[Path]:
    git_repos = []
    root_dir = root_dir.resolve()
    
    for path in root_dir.glob("*"):
        if path.is_dir():
            if is_git_repo(path):
                git_repos.append(path)
            elif max_depth > 1:
                for subpath in path.glob("*"):
                    if subpath.is_dir() and is_git_repo(subpath):
                        git_repos.append(subpath)
    
    return sorted(git_repos)

def copy_essential_files(source_dir: Path, target_dir: Path):
    """
    Copies .env* files, .vscode folder, docs folder, and agent files from source to target.
    """
    # 1. Glob patterns (like .env)
    for item in source_dir.glob(".env*"):
        if item.is_file():
            dest = target_dir / item.name
            if not dest.exists():
                try:
                    shutil.copy2(item, dest)
                    console.print(f"  [dim]Copied {item.name}[/dim]")
                except Exception as e:
                    console.print(f"  [red]Failed to copy {item.name}: {e}[/red]")

    # 2. Specific Agent/Doc files
    files_to_copy = ["AGENT.md", "CLAUDE.md", "GEMINI.md"]
    for filename in files_to_copy:
        src = source_dir / filename
        dest = target_dir / filename
        if src.exists() and src.is_file() and not dest.exists():
            try:
                shutil.copy2(src, dest)
                console.print(f"  [dim]Copied {filename}[/dim]")
            except Exception as e:
                console.print(f"  [red]Failed to copy {filename}: {e}[/red]")

    # 3. Directories (.vscode, docs)
    dirs_to_copy = [".vscode", "docs"]
    for dirname in dirs_to_copy:
        src = source_dir / dirname
        dest = target_dir / dirname
        if src.exists() and src.is_dir():
            try:
                # dirs_exist_ok=True allows copying into existing dir (merging/overwriting)
                shutil.copytree(src, dest, dirs_exist_ok=True)
                console.print(f"  [dim]Copied/Merged {dirname}/[/dim]")
            except Exception as e:
                console.print(f"  [red]Failed to copy {dirname}: {e}[/red]")

def setup_dependencies(target_dir: Path):
    """
    Detects project type and installs dependencies.
    """
    # Create a clean environment without the CLI's VIRTUAL_ENV
    clean_env = os.environ.copy()
    clean_env.pop("VIRTUAL_ENV", None)

    try:
        # Node.js
        if (target_dir / "pnpm-lock.yaml").exists():
            console.print("  [blue]Detected pnpm. Installing...[/blue]")
            subprocess.run(["pnpm", "install"], cwd=target_dir, env=clean_env, check=True)
        elif (target_dir / "bun.lockb").exists():
            console.print("  [blue]Detected bun. Installing...[/blue]")
            subprocess.run(["bun", "install"], cwd=target_dir, env=clean_env, check=True)
        elif (target_dir / "yarn.lock").exists():
            console.print("  [blue]Detected yarn. Installing...[/blue]")
            subprocess.run(["yarn", "install"], cwd=target_dir, env=clean_env, check=True)
        elif (target_dir / "package.json").exists():
            console.print("  [blue]Detected package.json. Running npm install...[/blue]")
            subprocess.run(["npm", "install"], cwd=target_dir, env=clean_env, check=True)

        # Go
        if (target_dir / "go.mod").exists():
            console.print("  [blue]Detected go.mod. Downloading modules...[/blue]")
            subprocess.run(["go", "mod", "download"], cwd=target_dir, env=clean_env, check=True)

        # Python
        has_uv = shutil.which("uv") is not None
        
        if (target_dir / "uv.lock").exists() and has_uv:
            console.print("  [blue]Detected uv.lock. Syncing...[/blue]")
            subprocess.run(["uv", "sync"], cwd=target_dir, env=clean_env, check=True)
        elif (target_dir / "poetry.lock").exists():
            console.print("  [blue]Detected poetry. Installing...[/blue]")
            subprocess.run(["poetry", "install"], cwd=target_dir, env=clean_env, check=True)
        elif (target_dir / "requirements.txt").exists():
            console.print("  [blue]Detected requirements.txt.[/blue]")
            
            if has_uv:
                console.print("  [blue]Using uv to create venv and install...[/blue]")
                subprocess.run(["uv", "venv"], cwd=target_dir, env=clean_env, check=True)
                subprocess.run(["uv", "pip", "install", "-r", "requirements.txt"], cwd=target_dir, env=clean_env, check=True)
            else:
                console.print("  [blue]Creating standard venv and installing...[/blue]")
                subprocess.run(["python3", "-m", "venv", ".venv"], cwd=target_dir, env=clean_env, check=True)
                pip_path = target_dir / ".venv" / "bin" / "pip"
                subprocess.run([str(pip_path), "install", "-r", "requirements.txt"], cwd=target_dir, env=clean_env, check=True)

    except subprocess.CalledProcessError as e:
        console.print(f"  [red]Dependency installation failed:[/red] {e}")
    except Exception as e:
        console.print(f"  [red]An error occurred during setup:[/red] {e}")

def get_preferred_base_branch(repo_path: Path) -> List[str]:
    """
    Returns a list of available base branches sorted by preference:
    development > staging > main > master
    """
    priorities = ["development", "staging", "main", "master"]
    available = []
    
    for branch in priorities:
        # Check local
        if subprocess.run(["git", "rev-parse", "--verify", branch], cwd=repo_path, capture_output=True).returncode == 0:
            available.append(branch)
            continue
        # Check remote (origin)
        if subprocess.run(["git", "rev-parse", "--verify", f"origin/{branch}"], cwd=repo_path, capture_output=True).returncode == 0:
            available.append(f"origin/{branch}")
    
    return available

@app.command()
def main():
    """
    Select git repositories from ~/code and create worktrees for a specific branch.
    """
    if not CODE_DIR.exists():
        console.print(f"[bold red]Error:[/bold red] {CODE_DIR} does not exist.")
        raise typer.Exit(code=1)

    console.print(f"[bold blue]Scanning {CODE_DIR} for git repositories...[/bold blue]")
    repos = find_git_repos(CODE_DIR)

    if not repos:
        console.print("[yellow]No git repositories found.[/yellow]")
        raise typer.Exit()

    # Create choices for InquirerPy
    choices = [str(repo.relative_to(CODE_DIR)) for repo in repos]

    selected_rel_paths = inquirer.fuzzy(
        message="Select repositories to create worktrees for (Type to search):",
        choices=choices,
        multiselect=True,
    ).execute()

    if not selected_rel_paths:
        console.print("No repositories selected. Exiting.")
        raise typer.Exit()

    branch_name = inquirer.text(message="Enter the target git branch name:").execute()
    
    if not branch_name:
        console.print("Branch name cannot be empty.")
        raise typer.Exit(code=1)

    # ---------------------------------------------------------
    # Phase 1: Preparation (Fetch & Prune)
    # ---------------------------------------------------------
    console.print(f"\n[bold blue]Preparing repositories...[/bold blue]")
    for rel_path in track(selected_rel_paths, description="Fetching & Pruning..."):
        repo_path = CODE_DIR / rel_path
        try:
            # Fetch to ensure we know about remote branches
            subprocess.run(["git", "fetch"], cwd=repo_path, capture_output=True)
            # Prune stale worktrees
            subprocess.run(["git", "worktree", "prune"], cwd=repo_path, capture_output=True)
        except Exception as e:
            console.print(f"[yellow]Warning fetching {rel_path}: {e}[/yellow]")

    # ---------------------------------------------------------
    # Phase 2: Configuration (Interactive)
    # ---------------------------------------------------------
    repo_actions: List[Dict[str, Any]] = []
    
    console.print(f"\n[bold blue]Configuring Worktrees...[/bold blue]")
    
    # Create work directory if it doesn't exist
    WORK_DIR.mkdir(parents=True, exist_ok=True)
    normalized_branch_dir = branch_name.replace("/", "-")

    is_single_repo = len(selected_rel_paths) == 1

    for rel_path in selected_rel_paths:
        repo_path = CODE_DIR / rel_path
        repo_name = repo_path.name
        
        if is_single_repo:
            target_path = (WORK_DIR / normalized_branch_dir).resolve()
        else:
            target_path = (WORK_DIR / normalized_branch_dir / repo_name).resolve()
        
        if target_path.exists():
            console.print(f"[yellow]Skipping {repo_name}: Target {target_path} already exists.[/yellow]")
            continue

        # Check if branch exists (local or remote)
        local_exists = subprocess.run(
            ["git", "rev-parse", "--verify", branch_name], 
            cwd=repo_path, 
            capture_output=True
        ).returncode == 0
        
        remote_exists = False
        if not local_exists:
            remote_exists = subprocess.run(
                ["git", "rev-parse", "--verify", f"origin/{branch_name}"], 
                cwd=repo_path, 
                capture_output=True
            ).returncode == 0
        
        if local_exists or remote_exists:
            repo_actions.append({
                "type": "add",
                "repo_path": repo_path,
                "target_path": target_path,
                "repo_name": repo_name,
                "branch": branch_name
            })
        else:
            # Branch needs creation. Ask for base.
            defaults = get_preferred_base_branch(repo_path)
            choices_list = defaults + ["Other (Manual Input)"]
            
            base_branch = inquirer.select(
                message=f"[{repo_name}] Branch '{branch_name}' missing. Create from base:",
                choices=choices_list,
                default=defaults[0] if defaults else None
            ).execute()
            
            if base_branch == "Other (Manual Input)":
                base_branch = inquirer.text(message=f"[{repo_name}] Enter base branch:").execute()
            
            repo_actions.append({
                "type": "create",
                "repo_path": repo_path,
                "target_path": target_path,
                "repo_name": repo_name,
                "branch": branch_name,
                "base": base_branch
            })

    if not repo_actions:
        console.print("[yellow]No actions to perform (all targets might exist). Exiting.[/yellow]")
        raise typer.Exit()

    # Confirm operation
    console.print(f"\n[bold]Plan:[/bold]")
    for action in repo_actions:
        if action["type"] == "add":
            console.print(f"  {action['repo_name']}: [green]Add worktree[/green] for '{action['branch']}'")
        else:
            console.print(f"  {action['repo_name']}: [blue]Create new branch[/blue] '{action['branch']}' from '{action['base']}'")
    
    if not inquirer.confirm(message="Proceed with execution?").execute():
        console.print("Aborted.")
        raise typer.Exit()

    # ---------------------------------------------------------
    # Phase 3: Execution
    # ---------------------------------------------------------
    for action in track(repo_actions, description="Creating Worktrees..."):
        repo_name = action["repo_name"]
        repo_path = action["repo_path"]
        target_path = action["target_path"]
        
        console.print(f"Processing [bold]{repo_name}[/bold]...")
        
        try:
            target_path.parent.mkdir(parents=True, exist_ok=True)
            success = False
            
            if action["type"] == "add":
                # git worktree add <path> <branch>
                cmd = ["git", "worktree", "add", str(target_path), action["branch"]]
                result = subprocess.run(cmd, cwd=repo_path, text=True, capture_output=True)
                if result.returncode == 0:
                    success = True
                    console.print(f"[green]Success:[/green] Worktree created.")
                else:
                     console.print(f"[red]Failed:[/red] {result.stderr.strip()}")
            
            elif action["type"] == "create":
                # git worktree add -b <new_branch> <path> <base>
                cmd = ["git", "worktree", "add", "-b", action["branch"], str(target_path), action["base"]]
                result = subprocess.run(cmd, cwd=repo_path, text=True, capture_output=True)
                if result.returncode == 0:
                    success = True
                    console.print(f"[green]Success (New Branch):[/green] Created from {action['base']}.")
                else:
                    console.print(f"[red]Failed to create branch:[/red] {result.stderr.strip()}")

            # Post-Processing
            if success:
                # Initialize submodules
                console.print(f"  [dim]Initializing submodules...[/dim]")
                subprocess.run(
                    ["git", "submodule", "update", "--init", "--recursive"], 
                    cwd=target_path, 
                    capture_output=True
                )
                
                copy_essential_files(repo_path, target_path)
                setup_dependencies(target_path)

        except Exception as e:
            console.print(f"[red]Error processing {repo_name}: {e}[/red]")

    console.print("\n[bold green]Done![/bold green]")
    
    if is_single_repo:
        console.print(f"Project created at: [bold blue]{repo_actions[0]['target_path']}[/bold blue]")
    else:
        # For multiple repos, they are all under the branch directory
        branch_dir = WORK_DIR / normalized_branch_dir
        console.print(f"Projects created in: [bold blue]{branch_dir.resolve()}[/bold blue]")

if __name__ == "__main__":
    app()
