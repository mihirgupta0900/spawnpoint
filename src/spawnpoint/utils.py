import os
import shutil
import subprocess
from pathlib import Path
from typing import List

from rich.console import Console

from .config import Config

console = Console()


def is_git_repo(path: Path) -> bool:
    return (path / ".git").exists()


def find_git_repos(scan_dirs: List[Path], max_depth: int = 2) -> List[Path]:
    """Find all git repos under the given directories, up to max_depth levels deep."""
    git_repos = []

    for root_dir in scan_dirs:
        if not root_dir.is_dir():
            continue
        root_dir = root_dir.resolve()

        for path in root_dir.glob("*"):
            if path.is_dir():
                if is_git_repo(path):
                    git_repos.append(path)
                elif max_depth > 1:
                    for subpath in path.glob("*"):
                        if subpath.is_dir() and is_git_repo(subpath):
                            git_repos.append(subpath)

    return sorted(set(git_repos))


def copy_essential_files(source_dir: Path, target_dir: Path, cfg: Config):
    """Copy configured files and directories from source to target worktree."""
    # Glob patterns (e.g. .env*)
    for pattern in cfg.copy_patterns_globs:
        for item in source_dir.glob(pattern):
            if item.is_file():
                dest = target_dir / item.name
                if not dest.exists():
                    try:
                        shutil.copy2(item, dest)
                        console.print(f"  [dim]Copied {item.name}[/dim]")
                    except Exception as e:
                        console.print(f"  [red]Failed to copy {item.name}: {e}[/red]")

    # Specific files
    for filename in cfg.copy_patterns_files:
        src = source_dir / filename
        dest = target_dir / filename
        if src.exists() and src.is_file() and not dest.exists():
            try:
                shutil.copy2(src, dest)
                console.print(f"  [dim]Copied {filename}[/dim]")
            except Exception as e:
                console.print(f"  [red]Failed to copy {filename}: {e}[/red]")

    # Directories
    for dirname in cfg.copy_patterns_dirs:
        src = source_dir / dirname
        dest = target_dir / dirname
        if src.exists() and src.is_dir():
            try:
                shutil.copytree(src, dest, dirs_exist_ok=True)
                console.print(f"  [dim]Copied {dirname}/[/dim]")
            except Exception as e:
                console.print(f"  [red]Failed to copy {dirname}: {e}[/red]")


def setup_dependencies(target_dir: Path):
    """Detect project type and install dependencies."""
    # Clean environment without CLI's VIRTUAL_ENV
    clean_env = os.environ.copy()
    clean_env.pop("VIRTUAL_ENV", None)

    try:
        # Node.js
        if (target_dir / "pnpm-lock.yaml").exists():
            console.print("  [blue]Detected pnpm. Installing...[/blue]")
            subprocess.run(["pnpm", "install"], cwd=target_dir, env=clean_env, check=True)
        elif (target_dir / "bun.lockb").exists() or (target_dir / "bun.lock").exists():
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

        # Ruby
        if (target_dir / "Gemfile").exists():
            console.print("  [blue]Detected Gemfile. Installing gems...[/blue]")
            subprocess.run(["bundle", "install"], cwd=target_dir, env=clean_env, check=True)

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
                subprocess.run(
                    ["uv", "pip", "install", "-r", "requirements.txt"],
                    cwd=target_dir, env=clean_env, check=True,
                )
            else:
                console.print("  [blue]Creating venv and installing...[/blue]")
                subprocess.run(["python3", "-m", "venv", ".venv"], cwd=target_dir, env=clean_env, check=True)
                pip_path = target_dir / ".venv" / "bin" / "pip"
                subprocess.run(
                    [str(pip_path), "install", "-r", "requirements.txt"],
                    cwd=target_dir, env=clean_env, check=True,
                )

    except subprocess.CalledProcessError as e:
        console.print(f"  [red]Dependency installation failed:[/red] {e}")
    except Exception as e:
        console.print(f"  [red]An error occurred during setup:[/red] {e}")


def get_preferred_base_branches(repo_path: Path, branch_priority: List[str]) -> List[str]:
    """Return available base branches sorted by priority."""
    available = []

    for branch in branch_priority:
        # Check local
        if subprocess.run(
            ["git", "rev-parse", "--verify", branch],
            cwd=repo_path, capture_output=True,
        ).returncode == 0:
            available.append(branch)
            continue
        # Check remote
        if subprocess.run(
            ["git", "rev-parse", "--verify", f"origin/{branch}"],
            cwd=repo_path, capture_output=True,
        ).returncode == 0:
            available.append(f"origin/{branch}")

    return available


def make_display_path(path: Path, scan_dirs: List[Path]) -> str:
    """Make a path relative to the most relevant scan dir for display."""
    for scan_dir in scan_dirs:
        try:
            return str(path.relative_to(scan_dir))
        except ValueError:
            continue
    return str(path)
