import atexit
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

    _offer_shell_integration()


_CD_PATH_FILE = "$HOME/.spawnpoint/.cd_path"

_POSIX_SNIPPET = f"""
# spawnpoint shell integration
sp() {{
    local cmd="${{1:-create}}"
    shift 2>/dev/null
    local cd_file="{_CD_PATH_FILE}"
    rm -f "$cd_file"
    case "$cmd" in
        create)     spawnpoint create "$@" ;;
        list|ls)    spawnpoint list --cd "$@" ;;
        *)          spawnpoint "$cmd" "$@" ;;
    esac
    if [ -f "$cd_file" ]; then
        local dir=$(cat "$cd_file")
        rm -f "$cd_file"
        [ -n "$dir" ] && cd "$dir"
    fi
}}
"""
_FISH_SNIPPET = f"""
# spawnpoint shell integration
function sp
    set cmd (test (count $argv) -gt 0; and echo $argv[1]; or echo create)
    set rest $argv[2..]
    set cd_file "{_CD_PATH_FILE}"
    rm -f $cd_file
    switch $cmd
        case create
            spawnpoint create $rest
        case list ls
            spawnpoint list --cd $rest
        case '*'
            spawnpoint $cmd $rest
    end
    if test -f $cd_file
        set dir (cat $cd_file)
        rm -f $cd_file
        test -n "$dir"; and cd $dir
    end
end
"""


def _detect_shell_rc() -> list[Path]:
    """Return existing shell rc files, in preference order."""
    import os

    candidates = []
    shell = os.environ.get("SHELL", "")
    home = Path.home()

    # Prefer the active shell's rc first
    if "zsh" in shell:
        candidates = [home / ".zshrc", home / ".bashrc", home / ".bash_profile"]
    elif "bash" in shell:
        candidates = [home / ".bashrc", home / ".bash_profile", home / ".zshrc"]
    elif "fish" in shell:
        candidates = [home / ".config" / "fish" / "config.fish"]
    else:
        candidates = [home / ".zshrc", home / ".bashrc", home / ".bash_profile"]

    return [p for p in candidates if p.exists()]


def _offer_shell_integration():
    """Offer to add the sp() shell function to the user's rc file."""
    rc_files = _detect_shell_rc()

    if not rc_files:
        console.print("[dim]Tip: add this to your shell rc to auto-cd after creating a workspace:[/dim]")
        console.print(f"[bold]{_POSIX_SNIPPET.strip()}[/bold]")
        return

    # Check if already installed in any of them
    for rc in rc_files:
        if "spawnpoint shell integration" in rc.read_text():
            console.print(f"[dim]Shell integration already present in {rc}[/dim]")
            return

    choices = [str(rc) for rc in rc_files] + ["Skip"]
    target = inquirer.select(
        message="Add sp() shell function for auto-cd after create?",
        choices=choices,
        default=str(rc_files[0]),
    ).execute()

    if target == "Skip":
        console.print("[dim]Skipped. You can add it manually:[/dim]")
        console.print(f"[bold]{_POSIX_SNIPPET.strip()}[/bold]")
        return

    rc_path = Path(target)
    snippet = _FISH_SNIPPET if "fish" in rc_path.parts or rc_path.suffix == ".fish" else _POSIX_SNIPPET
    with rc_path.open("a") as f:
        f.write(snippet)
    console.print(f"[green]Added sp() to {rc_path}[/green]")
    console.print(f"[dim]Restart your shell or run: source {rc_path}[/dim]")


@app.command()
def create(
    yes: bool = typer.Option(False, "--yes", "-y", help="Auto-select default base branch when creating new branches"),
):
    """Select repos and spawn worktree workspaces for a feature branch."""
    from .create import run_create
    cfg = _ensure_config()
    run_create(cfg, yes=yes)


@app.command()
def add():
    """Add repos to an existing workspace (run from inside a workspace)."""
    from .add import run_add
    cfg = _ensure_config()
    run_add(cfg)


@app.command(name="list")
def list_cmd(
    cd: bool = typer.Option(False, "--cd", "-c", help="Interactively select a workspace to cd into"),
):
    """List all worktree workspaces."""
    from .list import run_list
    cfg = _ensure_config()
    run_list(cfg, cd=cd)


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
            _offer_shell_integration()
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
    debug: bool = typer.Option(
        False, "--debug",
        help="Enable debug logging.",
    ),
):
    """Spawnpoint — Spawn multi-repo worktree workspaces for feature development."""
    from .log import setup_logging
    setup_logging(debug=debug)

    # Non-blocking update check (skip for update/version commands)
    _args = sys.argv[1:]
    _skip_commands = {"update", "--version", "-v"}
    if not _skip_commands.intersection(_args):
        from .config import load_config as _load_cfg
        try:
            cfg = _load_cfg()
            if cfg.check_updates:
                from .version_check import get_update_notice, start_check
                start_check(__version__)
                atexit.register(_show_update_notice)
        except Exception:
            pass


def _show_update_notice() -> None:
    try:
        from .version_check import get_update_notice
        notice = get_update_notice()
        if notice:
            Console(stderr=True).print(notice)
    except Exception:
        pass
