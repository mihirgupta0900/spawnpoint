# feat: Open Source Spawnpoint CLI

> Make worktree-workspaces production-ready, rename to **Spawnpoint**, and ship with a `curl | sh` installer.

## Current State

- 2 Python scripts (`main.py`, `cleanup.py`) with shell wrappers
- 3 deps: `typer`, `InquirerPy`, `rich`
- All paths hardcoded (`~/code`, `~/code/work/worktrees`)
- No packaging, no config, no README, no `.gitignore`, no tests
- Runs from cloned repo via `./run.sh` which activates a local venv

---

## Phase 1: Project Restructure & Packaging

### 1.1 Rename & restructure to proper Python package

```
spawnpoint/
├── pyproject.toml          # build config, entry points, metadata
├── README.md
├── LICENSE                  # MIT
├── .gitignore
├── install.sh              # curl-installable script
├── src/
│   └── spawnpoint/
│       ├── __init__.py     # version string
│       ├── cli.py          # typer app, combines create + cleanup commands
│       ├── create.py       # worktree creation (from main.py)
│       ├── cleanup.py      # worktree removal (from cleanup.py)
│       ├── config.py       # config loading/saving/defaults
│       └── utils.py        # shared helpers (git ops, file copy, dep install)
├── tests/
│   └── ...
└── completions/            # generated shell completions (optional)
```

### 1.2 `pyproject.toml`

```toml
[project]
name = "spawnpoint"
version = "0.1.0"
description = "Spawn multi-repo worktree workspaces for feature development"
requires-python = ">=3.10"
dependencies = ["typer>=0.9", "InquirerPy>=0.3", "rich>=13"]
license = "MIT"

[project.scripts]
spawnpoint = "spawnpoint.cli:app"

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"
```

- Single entry point: `spawnpoint create`, `spawnpoint cleanup`
- Drop `run.sh` / `clean.sh` — replaced by proper console script

### 1.3 `.gitignore`

```
venv/
__pycache__/
*.egg-info/
dist/
build/
.env
```

---

## Phase 2: Configuration System

### 2.1 Directory layout

```
~/.spawnpoint/
├── config.toml              # user config
└── workspaces/              # default worktree output dir
    ├── feat-auth/           # one workspace per feature
    │   ├── repo-a/          # worktree checkout
    │   └── repo-b/          # worktree checkout
    └── fix-payments/
        └── ...
```

- Config + data co-located in `~/.spawnpoint/` — simple, discoverable
- No XDG split needed for a tool this focused

### 2.2 Config schema

```toml
# ~/.spawnpoint/config.toml

# Directories to scan for git repos
scan_dirs = ["~/code"]

# Where workspaces are created (default: ~/.spawnpoint/workspaces)
worktree_dir = "~/.spawnpoint/workspaces"

# How deep to scan for repos (default: 2)
scan_depth = 2

# Files/dirs to copy into new worktrees
copy_patterns = [".env*", "AGENT.md", "CLAUDE.md", "GEMINI.md", ".vscode/", "docs/"]

# Branch priority for base branch selection
branch_priority = ["development", "staging", "main", "master"]

# Auto-install dependencies after worktree creation
auto_install_deps = true
```

### 2.3 Smart defaults philosophy

- **scan_dirs:** Auto-detect on first run by looking for common dirs (`~/code`, `~/projects`, `~/repos`, `~/src`, `~/dev`). Suggest whichever exist.
- **worktree_dir:** `~/.spawnpoint/workspaces` — always works, no assumptions about user's dir structure
- **copy_patterns:** Ship with sensible defaults for AI-assisted dev (`.env*`, `CLAUDE.md`, etc.)
- **branch_priority:** Covers most git-flow and trunk-based setups
- **auto_install_deps:** `true` — detect package manager (`package.json` → npm/yarn/pnpm/bun, `Gemfile` → bundle, `requirements.txt` → pip, etc.)

### 2.4 Config resolution order

1. CLI flags (highest priority)
2. `~/.spawnpoint/config.toml`
3. Built-in defaults

### 2.5 `config.py` responsibilities

- `load_config()` — read TOML, merge with defaults
- `save_config(data)` — write TOML
- `get_config_path()` → `~/.spawnpoint/config.toml`
- `detect_scan_dirs()` — check for common code dirs, return those that exist
- Expand `~` in all path values

---

## Phase 3: Interactive First-Run Setup (`spawnpoint init`)

Triggered automatically on first run (no config file exists) OR manually via `spawnpoint init`.

### 3.1 Flow

```
$ spawnpoint create   # first run, no config exists

  Welcome to Spawnpoint! Let's set things up.

  Found these code directories:
    ✓ ~/code
    ✓ ~/projects

  ? Use these as scan directories? (Y/n) >
  ? Add any others? (comma-separated, or enter to skip) >

  ? Where should workspaces be created?
    [default: ~/.spawnpoint/workspaces] >

  ? Auto-install dependencies after creating worktrees? (Y/n) >

  Config saved to ~/.spawnpoint/config.toml
  Run `spawnpoint config` to edit later.
```

### 3.2 Additional commands

- `spawnpoint config` — print current config path & contents
- `spawnpoint config edit` — open config in `$EDITOR`
- `spawnpoint config reset` — regenerate defaults
- `spawnpoint update` — self-update (delegates to `pipx upgrade spawnpoint` or `pip install --upgrade spawnpoint`)

---

## Phase 4: Install Script (`install.sh`)

### 4.1 Strategy: pipx-based install

Since Spawnpoint is a Python CLI, the cleanest approach is:
1. Install script ensures `pipx` is available
2. Uses `pipx install spawnpoint` from PyPI (or from GitHub release)
3. pipx handles venv isolation automatically — no system pollution

Fallback: `pip install --user spawnpoint` if pipx unavailable.

### 4.2 Install script outline

```bash
#!/bin/sh
set -e

# Colors & logging
log()  { printf "\033[0;32m=>\033[0m %s\n" "$1" >&2; }
warn() { printf "\033[1;33mwarning:\033[0m %s\n" "$1" >&2; }
err()  { printf "\033[0;31merror:\033[0m %s\n" "$1" >&2; exit 1; }

# 1. Check Python >= 3.10
check_python() {
    for cmd in python3 python; do
        if command -v "$cmd" >/dev/null 2>&1; then
            ver=$("$cmd" -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')")
            major=$(echo "$ver" | cut -d. -f1)
            minor=$(echo "$ver" | cut -d. -f2)
            if [ "$major" -ge 3 ] && [ "$minor" -ge 10 ]; then
                PYTHON="$cmd"
                return 0
            fi
        fi
    done
    err "Python 3.10+ required. Install from https://python.org"
}

# 2. Ensure pipx (or fallback to pip --user)
ensure_pipx() {
    if command -v pipx >/dev/null 2>&1; then
        INSTALLER="pipx"
        return 0
    fi
    log "pipx not found. Installing pipx..."
    "$PYTHON" -m pip install --user pipx 2>/dev/null || {
        warn "Could not install pipx. Falling back to pip --user"
        INSTALLER="pip"
        return 0
    }
    INSTALLER="pipx"
}

# 3. Install spawnpoint
install_spawnpoint() {
    check_python
    ensure_pipx

    if [ "$INSTALLER" = "pipx" ]; then
        log "Installing spawnpoint via pipx..."
        pipx install spawnpoint
    else
        log "Installing spawnpoint via pip..."
        "$PYTHON" -m pip install --user spawnpoint
    fi
}

# 4. Verify & next steps
verify() {
    if command -v spawnpoint >/dev/null 2>&1; then
        log "spawnpoint installed successfully! ($(spawnpoint --version))"
    else
        warn "spawnpoint installed but not in PATH"
        warn "You may need to add ~/.local/bin to your PATH"
    fi

    echo ""
    echo "  Next steps:"
    echo "    spawnpoint create    # spawn worktree workspaces"
    echo "    spawnpoint cleanup   # remove worktree workspaces"
    echo "    spawnpoint --help    # see all commands"
    echo ""
}

install_spawnpoint
verify
```

### 4.3 Usage

```bash
curl -fsSL https://raw.githubusercontent.com/mihirgupta0900/spawnpoint/main/install.sh | sh
```

### 4.4 Uninstall

```bash
pipx uninstall spawnpoint
rm -rf ~/.spawnpoint
```

---

## Phase 5: Code Changes

### 5.1 Extract hardcoded values → config

| Current hardcoded value | Replace with |
|---|---|
| `CODE_DIR = HOME / "code"` (`main.py:16`) | `config.scan_dirs` |
| `WORK_DIR = CODE_DIR / "work" / "worktrees"` (`main.py:17`) | `config.worktree_dir` |
| `max_depth = 2` (`main.py:22`) | `config.scan_depth` |
| `WORK_DIRS = [...]` (`cleanup.py:17-20`) | derive from `config.worktree_dir` |
| copy patterns (`main.py:42-75`) | `config.copy_patterns` |
| branch priority (`main.py:137`) | `config.branch_priority` |

### 5.2 CLI structure (`cli.py`)

```python
import typer
app = typer.Typer(name="spawnpoint", help="Spawn multi-repo worktree workspaces")

@app.command()
def create(): ...    # from main.py

@app.command()
def cleanup(): ...   # from cleanup.py

@app.command()
def init(): ...      # first-run setup

@app.command()
def config(): ...    # view/edit config

@app.command()
def update(): ...    # self-update via pipx/pip
```

### 5.3 Refactors needed

- Move `find_git_repos`, `copy_essential_files`, `setup_dependencies` → `utils.py`
- Move `create_worktrees` logic → `create.py`
- Move cleanup logic → `cleanup.py`
- All path references go through `config.py`

---

## Phase 6: README & Docs

### README structure

```markdown
# Spawnpoint

Spawn multi-repo worktree workspaces for feature development.

## Install

    curl -fsSL https://raw.githubusercontent.com/mihirgupta0900/spawnpoint/main/install.sh | sh

Or with pipx:

    pipx install spawnpoint

## Quick Start

    spawnpoint create     # select repos, name a branch, spawn worktrees
    spawnpoint cleanup    # select and remove worktree workspaces

## What It Does

Working on a feature that spans multiple repos? Spawnpoint creates a dedicated
folder with worktrees from each repo on the same branch, installs dependencies,
and copies over config files — so you can start coding (or start a Claude session)
immediately.

## Configuration

    spawnpoint config          # view config
    spawnpoint config edit     # edit in $EDITOR

Config lives at ~/.spawnpoint/config.toml

## Requirements

- Python 3.10+
- git

## License

MIT
```

---

## Phase 7: Publishing

1. Publish to PyPI (`hatch build && hatch publish`)
2. Create GitHub repo `mihirgupta0900/spawnpoint`
3. Tag `v0.1.0`
4. Add GitHub Action for PyPI publish on tag

---

## File Review Order

1. `pyproject.toml` — package definition
2. `src/spawnpoint/config.py` — config system
3. `src/spawnpoint/utils.py` — shared helpers
4. `src/spawnpoint/create.py` — worktree creation
5. `src/spawnpoint/cleanup.py` — worktree removal
6. `src/spawnpoint/cli.py` — CLI entry point
7. `install.sh` — install script
8. `README.md` — docs
9. `.gitignore` — housekeeping

---

## Resolved

- **Name:** `spawnpoint` (available on PyPI)
- **GitHub:** `mihirgupta0900/spawnpoint`
- **License:** MIT
- **Init flow:** Auto-trigger on first `spawnpoint create` if no config exists
- **Config options:** scan_dirs, worktree_dir, scan_depth, copy_patterns, branch_priority, auto_install_deps — sufficient

## Remaining Questions

None — all resolved.
