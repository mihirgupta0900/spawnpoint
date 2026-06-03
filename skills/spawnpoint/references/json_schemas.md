# Spawnpoint JSON output schemas & flag matrix

All commands accept `--json` to emit a single JSON document to stdout. Use `--no-input` (`-n`)
to disable interactive prompts.

## Flag matrix

| Command | Required in `--no-input` | Optional |
|---|---|---|
| `repos` | — | `--json` |
| `create` | `--repos`, `--branch`/`-b` | `--base`, `-y`/`--yes`, `--json` |
| `add` (run inside a workspace) | `--repos` | `--base`, `--json` |
| `list` | (with `--cd`) `--workspace` | `--cd`/`-c`, `--json` |
| `cleanup` | `--workspaces`, `--delete-branches`/`--keep-branches` | `--json` |
| `init` | n/a — interactive only, do not run from an agent | — |
| `config` | n/a — interactive/editor | `--edit`, `--reset` |

`--repos`, `--workspaces` are comma-separated lists of names. Names must match those returned
by `repos --json` / `list --json`. Unknown or ambiguous names exit non-zero with valid choices.

## `repos --json`

```json
[
  { "name": "api", "path": "/Users/x/code/api" },
  { "name": "web", "path": "/Users/x/code/web" }
]
```

## `create --no-input --json --repos ... --branch ...`

```json
{
  "workspace": "/Users/x/.spawnpoint/workspaces/feat-login",
  "branch": "feat/login",
  "repos": [
    {
      "name": "api",
      "branch": "feat/login",
      "base": "main",
      "action": "new_branch | existing_branch | checkout",
      "status": "created | success | failed"
    }
  ]
}
```

Without `--json` (but with `--no-input`): stdout is just the workspace path; status text is on stderr.

## `add --no-input --json --repos ...`

Same shape as create, but the per-repo list key is `added` instead of `repos`:

```json
{
  "workspace": "/Users/x/.spawnpoint/workspaces/feat-login",
  "branch": "feat/login",
  "added": [
    { "name": "worker", "branch": "feat/login", "base": "main", "action": "new_branch", "status": "created" }
  ]
}
```

## `list --json`

```json
[
  {
    "name": "feat-login",
    "path": "/Users/x/.spawnpoint/workspaces/feat-login",
    "repos": 2,
    "branches": ["feat/login"],
    "dirty": false
  }
]
```

`dirty` is true if any worktree in the workspace has uncommitted changes — check before cleanup.

## `cleanup --no-input --json --workspaces ... (--delete-branches|--keep-branches)`

```json
{
  "removed": [
    {
      "workspace": "feat-login",
      "worktrees": [
        { "repo": "api", "branch": "feat/login", "branch_deleted": true }
      ]
    }
  ]
}
```
