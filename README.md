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

## Quick Start

```
spawnpoint create     # select repos, name a branch, spawn worktrees
spawnpoint cleanup    # select and remove worktree workspaces
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
| `spawnpoint create` | Spawn worktree workspaces |
| `spawnpoint cleanup` | Remove worktree workspaces |
| `spawnpoint init` | Run interactive setup |
| `spawnpoint config` | View current config |
| `spawnpoint config --edit` | Edit config in $EDITOR |
| `spawnpoint config --reset` | Reset to defaults |
| `spawnpoint update` | Update to latest version |
| `spawnpoint --version` | Show version |

## Configuration

Config lives at `~/.spawnpoint/config.toml`:

```toml
# Directories to scan for git repos
scan_dirs = ['~/code', '~/projects']

# Where workspaces are created
worktree_dir = '~/.spawnpoint/workspaces'

# How deep to scan for repos (1-4)
scan_depth = 2

# Files/dirs to copy into new worktrees
copy_patterns_globs = ['.env*']
copy_patterns_files = ['AGENT.md', 'CLAUDE.md', 'GEMINI.md']
copy_patterns_dirs = ['.vscode', 'docs']

# Auto-install dependencies after worktree creation
auto_install_deps = true
```

When creating a new branch, Spawnpoint automatically detects the repo's default branch to use as the base. No configuration needed.

## Shell Integration

Add this to your `~/.zshrc` or `~/.bashrc` to automatically `cd` into the workspace after creation:

```sh
sp() {
  cd "$(spawnpoint create)"
}
```

Then use `sp` instead of `spawnpoint create`.

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
