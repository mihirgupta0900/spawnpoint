<div align="center">

# Spawnpoint

**One branch. Every repo. Ready to code.**

Spawnpoint gives every task its own folder: a git worktree for each repo it touches, all on the same branch, with `.env` files copied and dependencies installed.<br>
Point Claude Code, Codex, Cursor, Gemini CLI, or yourself at it and start working.

[![Watch the one-minute overview](./assets/spawnpoint.gif)](./assets/spawnpoint.mp4)

<sub>▶ <a href="./assets/spawnpoint.mp4">Watch in HD with sound (MP4)</a></sub>

</div>

```sh
brew install mihirgupta0900/tap/spawnpoint   # or: curl -fsSL https://raw.githubusercontent.com/mihirgupta0900/spawnpoint/main/install.sh | sh
sp create                                    # pick repos, name a branch, done
```

One native binary, about 20 ms to start, and `sp update` keeps it current however you installed it ([more options](#install)).

## Why Spawnpoint

Worktrees are how you run agents in parallel without them tripping over each other. They don't make a fresh checkout usable, though, and they only cover one repo at a time.

- **Your feature spans repos.** Billing touches `api`, `web`, and `worker`. Most worktree tools handle one repo at a time. Spawnpoint puts all three in one folder on one branch, so you and your agent see the whole change.
- **A fresh worktree won't run yet.** It has no `.env`, no `node_modules`, no `.venv`, and no submodules, so your agent's first ten minutes go to setup. Spawnpoint copies env and agent files (`.env*`, `CLAUDE.md`, `AGENT.md`) and installs dependencies with whatever each repo uses: npm, pnpm, yarn, bun, uv, poetry, pip, bundler, or go.
- **Parallel agents need separate checkouts.** Two agents in one checkout overwrite each other's edits and branch switches. With a workspace per task, each agent gets its own branch, files, and dependencies.
- **Old worktrees fill your disk.** Ten worktrees with `node_modules` add up to gigabytes. `sp light-cleanup` deletes the reinstallable directories and keeps your code.

```
~/.spawnpoint/workspaces/feat-billing/
├── api/      ⎇ feat/billing   .env  CLAUDE.md  .venv
├── web/      ⎇ feat/billing   .env.local  CLAUDE.md  node_modules
└── worker/   ⎇ feat/billing   .env  submodules  node_modules
```

The checkouts in `~/code` are never touched.

## Built for coding agents

Spawnpoint builds the workspace and leaves the rest to you. It isn't an agent GUI, so any agent, editor, or terminal can use it, and an agent can create its own workspaces.

**Install the agent skill** ([skills.sh](https://www.skills.sh)). Claude Code, Codex, and other skill-aware agents learn to call Spawnpoint headlessly:

```sh
npx skills add mihirgupta0900/spawnpoint
```

**Or drive it from any script or agent.** Every command runs with `--no-input --json`, never waits on a prompt, and prints JSON you can parse:

```sh
$ spawnpoint create --no-input --json --repos api,web --branch feat/billing
{
  "workspace": "/Users/you/.spawnpoint/workspaces/feat-billing",
  "branch": "feat/billing",
  "template": null,
  "repos": [
    { "name": "api", "branch": "feat/billing", "base": "main", "action": "create", "status": "created" },
    { "name": "web", "branch": "feat/billing", "base": "main", "action": "create", "status": "created" }
  ]
}
```

A typical parallel setup runs one workspace per agent:

```sh
sp create --no-input --repos api,web        --branch feat/billing      # → Claude Code
sp create --no-input --repos api,worker     --branch fix/auth-timeout  # → Codex
sp create --no-input --template search-stack --branch feat/search      # → Gemini CLI
```

See [Non-Interactive Mode](#non-interactive-mode-for-agents--scripts) for every flag.

## Install

### macOS app

Download the latest `.dmg` from [Releases](https://github.com/mihirgupta0900/spawnpoint/releases?q=mac) and drag Spawnpoint into Applications. It lives in your menu bar, so you don't need a terminal.

Source: [`mac/`](./mac).

### CLI

Spawnpoint is a single native binary for macOS, Linux and Windows. Nothing else is needed at runtime besides `git`. Pick whichever installer you already use:

```sh
# Homebrew (macOS, Linux)
brew install mihirgupta0900/tap/spawnpoint

# Install script: downloads the release binary and verifies its checksum
curl -fsSL https://raw.githubusercontent.com/mihirgupta0900/spawnpoint/main/install.sh | sh

# pipx / uv / pip: same native binary, packaged as a wheel
pipx install spawnpoint
uv tool install spawnpoint

# Go
go install github.com/mihirgupta0900/spawnpoint@latest
```

Or download a build from [Releases](https://github.com/mihirgupta0900/spawnpoint/releases). The install script puts the binary in `~/.local/bin`. Set `SPAWNPOINT_INSTALL_DIR` to change that, or `SPAWNPOINT_VERSION` to pin a version.

This installs both `spawnpoint` and `sp` as commands (`go install` only provides `spawnpoint`; the [shell integration](#shell-integration) adds `sp`). All examples below use `sp` for brevity.

## Updating

```sh
sp update          # upgrade to the latest release
sp update --check  # just report whether one is available
```

`sp update` detects how Spawnpoint was installed and upgrades it the same way, so it never ends up with two copies:

| Installed with | `sp update` runs |
|---|---|
| Homebrew | `brew upgrade spawnpoint` |
| pipx | `pipx upgrade spawnpoint` |
| uv | `uv tool upgrade spawnpoint` |
| pip | `python -m pip install --upgrade spawnpoint` |
| `go install` | `go install github.com/mihirgupta0900/spawnpoint@latest` |
| Install script / manual download | downloads the new release, verifies its checksum, and swaps the binary in place |

Spawnpoint checks for new releases in the background at most once a day, then prints a one-line notice after a command. The check is skipped in `--json` / `--no-input` mode, and you can turn it off with `check_updates = false`.

### Coming from the Python version (0.11 and earlier)

You don't have to do anything special. Run `sp update` (or `pipx upgrade spawnpoint`) as usual, and pipx swaps the Python package for the native binary. Everything else stays as it is:

- `~/.spawnpoint/config.toml` and `templates.toml` are read as-is
- existing workspaces, the `sp()` shell function and the agent skill keep working
- every `--no-input` / `--json` flag and output shape is unchanged

What you get: startup in about 20 ms instead of about 90 ms, `list` roughly twice as fast, repos fetched in parallel, and no Python install to maintain. `--json` output is now always clean JSON. The Python release could mix progress output into stdout.

If you'd rather switch to Homebrew or the install script, remove the pipx copy first: `pipx uninstall spawnpoint`. The install script refuses to overwrite a pipx or uv install, so you won't end up with two copies.

## Quick Start

```
sp create     # select repos, name a branch, spawn worktrees
sp create -y  # auto-select default base branches (skip base branch prompts)
sp list       # view all workspaces
sp add        # add repos to the current workspace
sp cleanup        # select and remove worktree workspaces
sp light-cleanup  # free space by deleting node_modules, .venv, etc. (keeps code)

sp template save image-gen   # remember a set of repos you spawn together
sp create -t image-gen       # spawn that set again
```

On first run, Spawnpoint asks you to configure your scan directories and workspace location.

## How It Works

1. **Select repos.** Spawnpoint scans your code directories and shows a fuzzy-searchable list of git repos.
2. **Name a branch.** Enter a branch name for your feature.
3. **Spawn.** For each repo, Spawnpoint:
   - Creates a git worktree, and a new branch if needed, from the latest remote tip
   - Initializes submodules
   - Copies `.env` files, `CLAUDE.md`, and other config files from the original repo
   - Installs dependencies (detects npm/pnpm/yarn/bun, pip/uv/poetry, bundler, go modules)

All worktrees land in a single folder (`~/.spawnpoint/workspaces/<branch-name>/`). Open the whole workspace in your editor or start an AI coding session there.

## Commands

| Command | Description |
|---|---|
| `sp create` | Spawn worktree workspaces |
| `sp create -y` | Auto-select default base branches |
| `sp create -t <name>` | Spawn from a saved template |
| `sp list` | List all workspaces |
| `sp list --cd` | Interactively select a workspace to cd into |
| `sp repos` | List repositories available to select |
| `sp add` | Add repos to the current workspace |
| `sp template list` | List saved templates |
| `sp template save <name>` | Create or update a template |
| `sp template show <name>` | Show one template |
| `sp template delete <name>` | Delete a template |
| `sp cleanup` | Remove worktree workspaces |
| `sp light-cleanup` | Free space by deleting reinstallable dirs (node_modules, .venv, etc.) |
| `sp init` | Run interactive setup |
| `sp config` | View current config |
| `sp config --edit` | Edit config in $EDITOR |
| `sp config --reset` | Reset to defaults |
| `sp update` | Update to the latest version (via brew/pipx/uv/pip when that's how it was installed) |
| `sp update --check` | Check for an update without installing it |
| `sp --version` | Show version |
| `sp completion <shell>` | Print a shell completion script (bash, zsh, fish, powershell) |

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

## Templates

If the same repos keep coming up together — an image generation stack, a billing surface — save them once as a template and spawn them by name:

```sh
sp template save image-gen                      # pick repos interactively
sp template save image-gen --repos chottu,backend,daily-prophet --base staging
```

Then use it:

```sh
sp create -t image-gen        # or --template image-gen
```

A template stores the repo set, plus an optional base branch and description. `sp create` uses it as a starting point: the repo picker opens with those repos already selected, so you can still add or drop one for this workspace only. With `--no-input` the template's repos are used as-is.

### Picking a template in `sp create`

Run `sp create` with no flags and saved templates are offered first, ahead of the repo picker:

```
? Start from a template?
❯ Pick repos manually
  billing  (backend, dashboard)
  image-gen  (Image generation stack)
```

"Pick repos manually" is the default and comes first, so pressing Enter gives you exactly the old behaviour — and the prompt doesn't appear at all until you save your first template. Templates are listed by name with their description, or their repos if they have none.

Pick one and the repo picker opens with those repos already toggled on:

```
? Select repositories (type to search):
❯   4/4 (3)  Selected: backend, chottu, daily-prophet
❯❯backend
 ❯chottu
 ❯daily-prophet
  dashboard
```

So a template is two keystrokes (arrow, Enter) and then Enter again to accept the set, or a few TABs to adjust it for this one workspace. Skip the prompt entirely with `sp create -t image-gen`.

### Saving a template

After you confirm the plan, `sp create` offers to remember the set:

```
? Save these 3 repos as a template for next time? (y/N)
? Template name: image-gen
```

The default is No, and the offer is skipped when you picked a single repo or when the set still matches the template you started from. To name it up front instead:

```sh
sp create --save-template image-gen
```

That also works on an existing template, so `sp create -t image-gen --save-template image-gen` spawns the set and folds in whatever you tweaked in the picker.

Templates live in `~/.spawnpoint/templates.toml` and are safe to edit or commit as a dotfile:

```toml
[templates."image-gen"]
description = "Image generation stack"
repos = ["chottu", "backend", "daily-prophet"]
base = "staging"
```

Repo names must match the names from `sp repos`. Deleting a template never touches workspaces you already spawned.

## Non-Interactive Mode (for agents & scripts)

Every interactive command can run fully non-interactively with `--no-input` (`-n`), so coding agents and scripts can drive Spawnpoint without prompts. In this mode, every selection must be supplied via flags — a missing required flag exits non-zero with a clear error instead of hanging.

Add `--json` to any command for machine-readable output on stdout (human-readable text stays on stderr).

### Discover what's available

```sh
sp repos --json           # repos you can pass to --repos
sp list --json            # existing workspaces (names usable as --workspaces / --workspace)
sp template list --json   # templates you can pass to --template
```

### Create a workspace

```sh
sp create --no-input --repos api,web --branch feat-x --base main --json
sp create --no-input --template image-gen --branch feat-x --json
```

- `--repos` — comma-separated repo names (match the names from `sp repos`). Required unless `--template` is given.
- `--template` / `-t` — take the repo set from a saved template. An explicit `--repos` overrides it.
- `--branch` — branch name (required).
- `--base` — base branch for branches that don't exist yet. Optional; defaults to the template's base, then to each repo's detected default branch. Required only if no default can be detected.

On success it prints the workspace path to stdout (capture with `$(...)`), or full JSON with `--json`.

### Manage templates

```sh
sp template save image-gen --no-input --repos api,web --base main --json
sp template show image-gen --json
sp template delete image-gen --no-input --json
```

- `template save` requires `--repos` in `--no-input`; `--base` and `--description` are optional and keep their saved values when omitted.
- A template naming a repo that no longer exists is an error in `--no-input`; interactively it is a warning and the remaining repos are pre-selected.

### Add repos to the current workspace

Run from inside a workspace:

```sh
sp add --no-input --repos api --base main --json
```

### Remove workspaces

```sh
sp cleanup --no-input --workspaces feat-x,bug-y --delete-branches --json
```

- `--workspaces` — comma-separated workspace names (from `sp list`).
- `--delete-branches` / `--keep-branches` — required; whether to delete the branches from parent repos.

### Free space without deleting code (light cleanup)

```sh
sp light-cleanup
```

Scans selected workspaces for reinstallable artifact directories (`node_modules`, `.venv`, `venv`, `__pycache__`, `.next`, `target`, etc.), shows sizes, lets you pick which types to delete, and removes them. Your code is untouched.

Non-interactive:

```sh
sp light-cleanup --no-input --workspaces feat-x,bug-y --json
# delete only specific artifact types:
sp light-cleanup --no-input --workspaces feat-x --artifact-types node_modules,.venv --json
```

- `--workspaces` — comma-separated workspace names.
- `--artifact-types` — comma-separated artifact dir names to delete (default: all found).

### cd into a workspace

```sh
sp list --cd --no-input --workspace feat-x
```

Prints the workspace path to stdout (and writes the cd-path file used by shell integration).

> If no config exists yet, non-interactive commands auto-create one from detected defaults instead of prompting.

### Agent skill

A bundled [Claude Code / agent skill](skills/spawnpoint/SKILL.md) teaches agents to drive Spawnpoint non-interactively. Install it via [skills.sh](https://www.skills.sh):

```sh
npx skills add mihirgupta0900/spawnpoint
```

This adds the `spawnpoint` skill so agents automatically know to use `--no-input --json` and the correct flags for each command.

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

# Check for new versions on startup
check_updates = true
```

Templates are stored separately in `~/.spawnpoint/templates.toml`, so `sp init` and `sp config --reset` leave them alone. See [Templates](#templates).

### Additional worktree dirs

If you change `worktree_dir`, workspaces created at the old location won't be found during cleanup. Add the old path to `additional_worktree_dirs` so cleanup and list can still find them:

```toml
worktree_dir = '~/new-location/workspaces'
additional_worktree_dirs = ['~/.spawnpoint/workspaces']
```

When creating a new branch, Spawnpoint automatically detects the repo's default branch to use as the base. No configuration needed.

Set `SPAWNPOINT_DIR` to use a config directory other than `~/.spawnpoint`, e.g. for a throwaway setup in tests. The shell integration's auto-cd always reads `~/.spawnpoint/.cd_path`.

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

- git
- macOS 12+, Linux (x86_64 / arm64, glibc or musl), or Windows (x86_64)

## Development

```sh
go test ./...      # unit + end-to-end tests (real git repos in a temp $HOME)
go run . --help
GORELEASER_CURRENT_TAG=v1.2.3 goreleaser release --snapshot --clean   # local release build
python scripts/build_wheels.py --version 1.2.3                         # PyPI wheels from that build
```

The e2e suite in `e2e/` pins the `--no-input` / `--json` contract. Point it at another build with `SPAWNPOINT_BIN=/path/to/spawnpoint go test ./e2e`.

## Uninstall

Remove it with the tool you installed it with (`brew uninstall spawnpoint`, `pipx uninstall spawnpoint`, `uv tool uninstall spawnpoint`), or delete `~/.local/bin/spawnpoint` and `~/.local/bin/sp` for the install script. Then remove your config and templates:

```
rm -rf ~/.spawnpoint
```

## License

MIT
