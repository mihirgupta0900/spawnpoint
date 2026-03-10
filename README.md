# Spawnpoint

Spawn multi-repo worktree workspaces for feature development.

Working on a feature that spans multiple repos? Spawnpoint creates a dedicated folder with git worktrees from each repo on the same branch, installs dependencies, and copies over config files — so you can start coding (or start a Claude session) immediately.

![Demo](./demo/demo.gif)

## Install

```
pipx install spawnpoint
```

Or with pip:

```
pip install spawnpoint
```

This installs both `spawnpoint` and `sp` as CLI commands. All examples below use `sp` for brevity.

## Quick Start

```
sp create     # select repos, name a branch, spawn worktrees
sp list       # view all workspaces
sp add        # add repos to the current workspace
sp cleanup    # select and remove worktree workspaces
```

On first run, Spawnpoint will ask you to configure your scan directories and workspace location.

## How It Works

1. **Select repos** — Spawnpoint scans your code directories and presents a fuzzy-searchable list of git repos
2. **Name a branch** — Enter a branch name for your feature
3. **Spawn** — For each repo, Spawnpoint:
   - Creates a git worktree (or new branch if needed)
   - Initializes submodules
   - Copies `.env` files, `CLAUDE.md`, and other config files from the original repo
   - Installs dependencies (detects npm/pnpm/yarn/bun, pip/uv/poetry, bundler, go modules)

All worktrees land in a single folder (`~/.spawnpoint/workspaces/<branch-name>/`) so you can open the whole workspace in your editor or start an AI coding session.

## Commands

| Command | Description |
|---|---|
| `sp create` | Spawn worktree workspaces |
| `sp list` | List all workspaces |
| `sp list --cd` | Interactively select a workspace to cd into |
| `sp add` | Add repos to the current workspace |
| `sp cleanup` | Remove worktree workspaces |
| `sp init` | Run interactive setup |
| `sp config` | View current config |
| `sp config --edit` | Edit config in $EDITOR |
| `sp config --reset` | Reset to defaults |
| `sp update` | Update to latest version |
| `sp --version` | Show version |

### Adding repos to a workspace

When you're inside a spawnpoint workspace and need another repo, run:

```
sp add
```

Spawnpoint detects the current workspace and branch, shows repos not yet in the workspace, and adds them. If the workspace was originally single-repo, it automatically restructures to multi-repo layout.

### Listing workspaces

```
sp list
```

Shows a table of all workspaces with repo count, branch, dirty status, and age.

Use `sp list --cd` (or `sp list` with shell integration) to interactively pick a workspace and cd into it.

## Configuration

Config lives at `~/.spawnpoint/config.toml`:

```toml
# Directories to scan for git repos
scan_dirs = ['~/code', '~/projects']

# Where workspaces are created
worktree_dir = '~/.spawnpoint/workspaces'

# Additional directories to scan during cleanup (for worktrees created at previous locations)
additional_worktree_dirs = []

# How deep to scan for repos (1-4)
scan_depth = 2

# Files/dirs to copy into new worktrees
copy_patterns_globs = ['.env*']
copy_patterns_files = ['AGENT.md', 'CLAUDE.md', 'GEMINI.md']
copy_patterns_dirs = ['.vscode', 'docs']

# Auto-install dependencies after worktree creation
auto_install_deps = true
```

### Additional worktree dirs

If you change `worktree_dir`, workspaces created at the old location won't be found during cleanup. Add the old path to `additional_worktree_dirs` so cleanup and list can still find them:

```toml
worktree_dir = '~/new-location/workspaces'
additional_worktree_dirs = ['~/.spawnpoint/workspaces']
```

When creating a new branch, Spawnpoint automatically detects the repo's default branch to use as the base. No configuration needed.

## Shell Integration

During `sp init`, you'll be offered to install a shell function that wraps common commands with auto-cd:

```sh
sp() {
    local cmd="${1:-create}"
    shift 2>/dev/null
    local cd_file="$HOME/.spawnpoint/.cd_path"
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
}
```

With shell integration:
- `sp` — create a workspace and cd into it
- `sp list` or `sp ls` — pick a workspace and cd into it
- `sp cleanup`, `sp add`, etc. — passed through to spawnpoint

Without shell integration, `sp` still works for all commands — you just won't get auto-cd for create/list.

## Requirements

- Python 3.10+
- git

## Uninstall

```
pipx uninstall spawnpoint
rm -rf ~/.spawnpoint
```

## License

MIT
