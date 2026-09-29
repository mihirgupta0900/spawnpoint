# Spawnpoint JSON output schemas & flag matrix

All commands accept `--json` to emit a single JSON document to stdout. Use `--no-input` (`-n`)
to disable interactive prompts.

## Flag matrix

| Command | Required in `--no-input` | Optional |
|---|---|---|
| `repos` | — | `--json` |
| `create` | `--branch`/`-b`, and one of `--repos` / `--template` | `--base`, `-y`/`--yes`, `--save-template`, `--json` |
| `add` (run inside a workspace) | `--repos` | `--base`, `--json` |
| `list` | (with `--cd`) `--workspace` | `--cd`/`-c`, `--json` |
| `cleanup` | `--workspaces`, `--delete-branches`/`--keep-branches` | `--json` |
| `template list` | — | `--json` |
| `template show <name>` | — | `--json` |
| `template save <name>` | `--repos` | `--base`, `--description`/`-d`, `--json` |
| `template delete <name>` | — (`--no-input` skips the confirm) | `--json` |
| `init` | n/a — interactive only, do not run from an agent | — |
| `config` | n/a — interactive/editor | `--edit`, `--reset` |

`--repos`, `--workspaces` are comma-separated lists of names. Names must match those returned
by `repos --json` / `list --json`. Unknown or ambiguous names exit non-zero with valid choices.
`--template` takes a single name from `template list --json`.

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
  "template": "image-gen",
  "repos": [
    {
      "name": "api",
      "branch": "feat/login",
      "base": "main",
      "action": "create | add",
      "status": "created | failed"
    }
  ]
}
```

`action` is `create` when the branch was new in that repo (branched from `base`) and `add` when it already existed locally or on origin (`base` is then `null`). `template` is the template the repo set came from, or `null` when repos were passed explicitly.

Without `--json` (but with `--no-input`): stdout is just the workspace path; status text is on stderr.

## `add --no-input --json --repos ...`

Same shape as create, but the per-repo list key is `added` instead of `repos`:

```json
{
  "workspace": "/Users/x/.spawnpoint/workspaces/feat-login",
  "branch": "feat/login",
  "added": [
    { "name": "worker", "branch": "feat/login", "base": "main", "action": "create", "status": "added" }
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

## `template list --json`

```json
[
  {
    "name": "image-gen",
    "repos": ["api", "web"],
    "base": "main",
    "description": "Image generation stack"
  }
]
```

`base` and `description` are `null` when unset; a `null` base means each repo's default branch is
detected at create time. `template show --json` returns a single object of the same shape, and
`template save --json` returns the template it wrote.

## `template delete <name> --no-input --json`

```json
{ "deleted": "image-gen" }
```

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
