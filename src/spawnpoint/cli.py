import shutil
import subprocess
import sys
from pathlib import Path

import typer
from InquirerPy import inquirer
from rich.console import Console

from . import __version__
from .config import (
    CONFIG_PATH,
    SPAWNPOINT_DIR,
    Config,
    config_exists,
    detect_scan_dirs,
    load_config,
    save_config,
)

app = typer.Typer(
    name="spawnpoint",
    help="Spawn multi-repo worktree workspaces for feature development.",
    no_args_is_help=True,
)
console = Console()


def _ensure_config() -> Config:
    """Load config, triggering init if none exists."""
    if not config_exists():
        console.print("[bold]Welcome to Spawnpoint![/bold] Let's set things up.\n")
        _run_init()
    return load_config()


def _run_init():
    """Interactive first-run setup."""
    cfg = Config()

    # Detect scan dirs
    detected = detect_scan_dirs()
    if detected:
        console.print("Found these code directories:")
        for d in detected:
            console.print(f"  [green]✓[/green] {d}")
        console.print()

        use_detected = inquirer.confirm(
            message="Use these as scan directories?",
            default=True,
        ).execute()

        if use_detected:
            cfg.scan_dirs = detected
        else:
            cfg.scan_dirs = []

    extras = inquirer.text(
        message="Add any other directories? (comma-separated, enter to skip)",
    ).execute()

    if extras and extras.strip():
        for d in extras.split(","):
            d = d.strip()
            if d:
                p = Path(d).expanduser().resolve()
                if p.is_dir():
                    if p not in cfg.scan_dirs:
                        cfg.scan_dirs.append(p)
                else:
                    console.print(f"  [yellow]Skipping {d} (does not exist)[/yellow]")

    if not cfg.scan_dirs:
        console.print("[yellow]No scan directories set. You can add them later in the config.[/yellow]")

    # Worktree dir
    default_wt = str(SPAWNPOINT_DIR / "workspaces")
    wt_dir = inquirer.text(
        message="Where should workspaces be created?",
        default=default_wt,
    ).execute()
    cfg.worktree_dir = Path(wt_dir).expanduser().resolve()

    # Auto install deps
    cfg.auto_install_deps = inquirer.confirm(
        message="Auto-install dependencies after creating worktrees?",
        default=True,
    ).execute()

    path = save_config(cfg)
    console.print(f"\n[green]Config saved to {path}[/green]")
    console.print(f"Run [bold]spawnpoint config[/bold] to view or edit later.\n")


@app.command()
def create():
    """Select repos and spawn worktree workspaces for a feature branch."""
    from .create import run_create
    cfg = _ensure_config()
    run_create(cfg)


@app.command()
def cleanup():
    """Select and remove worktree workspaces."""
    from .cleanup import run_cleanup
    cfg = _ensure_config()
    run_cleanup(cfg)


@app.command()
def init():
    """Run interactive setup (creates or overwrites config)."""
    if config_exists():
        overwrite = inquirer.confirm(
            message=f"Config already exists at {CONFIG_PATH}. Overwrite?",
            default=False,
        ).execute()
        if not overwrite:
            console.print("Keeping existing config.")
            raise typer.Exit()
    _run_init()


@app.command()
def config(
    edit: bool = typer.Option(False, "--edit", "-e", help="Open config in $EDITOR"),
    reset: bool = typer.Option(False, "--reset", help="Reset config to defaults"),
):
    """View or edit configuration."""
    if reset:
        confirm = inquirer.confirm(
            message="Reset config to defaults?",
            default=False,
        ).execute()
        if confirm:
            cfg = Config()
            cfg.scan_dirs = detect_scan_dirs()
            save_config(cfg)
            console.print("[green]Config reset to defaults.[/green]")
        return

    if edit:
        if not CONFIG_PATH.is_file():
            console.print("[yellow]No config file yet. Run [bold]spawnpoint init[/bold] first.[/yellow]")
            raise typer.Exit(code=1)
        editor = shutil.which("$EDITOR") or shutil.which("vim") or shutil.which("nano")
        import os
        editor = os.environ.get("EDITOR", editor or "vi")
        subprocess.run([editor, str(CONFIG_PATH)])
        return

    if not CONFIG_PATH.is_file():
        console.print("[yellow]No config file. Run [bold]spawnpoint init[/bold] to create one.[/yellow]")
        raise typer.Exit()

    console.print(f"[bold]Config:[/bold] {CONFIG_PATH}\n")
    console.print(CONFIG_PATH.read_text())


@app.command()
def update():
    """Update spawnpoint to the latest version."""
    if shutil.which("pipx"):
        console.print("[blue]Updating via pipx...[/blue]")
        result = subprocess.run(["pipx", "upgrade", "spawnpoint"], capture_output=True, text=True)
        if result.returncode == 0:
            console.print(f"[green]{result.stdout.strip()}[/green]")
        else:
            console.print(f"[red]{result.stderr.strip()}[/red]")
    else:
        console.print("[blue]Updating via pip...[/blue]")
        result = subprocess.run(
            [sys.executable, "-m", "pip", "install", "--upgrade", "spawnpoint"],
            capture_output=True, text=True,
        )
        if result.returncode == 0:
            console.print("[green]Updated successfully.[/green]")
        else:
            console.print(f"[red]{result.stderr.strip()}[/red]")


def version_callback(value: bool):
    if value:
        console.print(f"spawnpoint {__version__}")
        raise typer.Exit()


@app.callback()
def main(
    version: bool = typer.Option(
        False, "--version", "-v",
        help="Show version and exit.",
        callback=version_callback,
        is_eager=True,
    ),
):
    """Spawnpoint — Spawn multi-repo worktree workspaces for feature development."""
    pass
