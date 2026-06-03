---
name: spawnpoint
description: This skill should be used when an agent needs to create, list, extend, or remove multi-repo git worktree workspaces using the spawnpoint CLI (binary `spawnpoint`, shell wrapper `sp`). Use it when a task involves spinning up isolated worktrees for a feature branch across one or more repos, finding existing workspaces, adding repos to the current workspace, or cleaning up worktrees. Always invoke spawnpoint in non-interactive mode with `--no-input --json` so commands never block on a TTY prompt.
---

# Spawnpoint (agent usage)

Spawnpoint spawns git worktree workspaces for feature development. A "workspace" is a
single folder under the configured worktree dir holding one worktree per selected repo,
all on the same feature branch. This skill covers driving it **non-interactively** from
an agent.

## Critical rule: never run interactively

The interactive `sp` shell wrapper and bare commands prompt via a TTY and will **hang** an
agent. Always:

- Call the underlying binary **`spawnpoint`**, not the `sp` shell function.
- Pass **`--no-input`** (alias `-n`) to disable prompts, and **`--json`** for parseable output.
- Supply every required value via flags. In `--no-input`, a missing required flag exits
  non-zero with a message naming the flag (e.g. `--no-input requires --repos`); read that
  message and retry with the flag set.

## Workflow

1. **Discover available repos** before creating/adding — repo names come from this list:
   ```bash
   spawnpoint repos --json
   ```
   Returns `[{ "name": "...", "path": "..." }]`. Use the `name` values verbatim for `--repos`.
   If it errors with "No scan directories configured", the user must run `spawnpoint init`
   (interactive) first — surface that, do not attempt init from an agent.

2. **Create a workspace** for a feature branch:
   ```bash
   spawnpoint create --no-input --json --repos "api,web" --branch "feat/login" [--base main]
   ```
   - `--repos` comma-separated repo names from step 1 (required).
   - `--branch` / `-b` feature branch name (required).
   - `--base` base branch for newly-created branches; omit to use each repo's detected default.
   - `-y` / `--yes` auto-selects the default base without `--base` (single-repo convenience).
   Returns the new `workspace` path and per-repo `status`. `cd` into `workspace` for follow-up work.

3. **Add repos to the current workspace** — run from **inside** an existing workspace dir:
   ```bash
   cd <workspace-path> && spawnpoint add --no-input --json --repos "worker" [--base main]
   ```

4. **List workspaces**:
   ```bash
   spawnpoint list --json
   ```
   Returns an array of workspaces with `path`, `repos` count, `branches`, and `dirty` flag.
   To resolve a single workspace path non-interactively: `spawnpoint list --cd --no-input --json --workspace "<name>"`.

5. **Clean up workspaces**:
   ```bash
   spawnpoint cleanup --no-input --json --workspaces "feat/login,feat/old" --delete-branches
   ```
   - `--workspaces` comma-separated names (required; get names from `list`).
   - `--delete-branches` / `--keep-branches` required in `--no-input` — choose explicitly.
   Returns a `removed` report. This is destructive; only run with explicit user intent and
   confirm the target names against `spawnpoint list --json` first.

## Output handling

- With `--json`, stdout is a single JSON document — parse it; do not also pass non-JSON flags
  that would pollute stdout.
- Without `--json`, rich/status output goes to **stderr** and only the workspace path goes to
  stdout (so `path=$(spawnpoint create --no-input --repos ... --branch ...)` captures it).
- Exit code 0 = success. Non-zero = a required flag was missing or a name was unknown/ambiguous;
  the stderr message lists valid choices — correct and retry.

## Reference

See `references/json_schemas.md` for exact JSON output shapes of `repos`, `create`, `add`,
`list`, and `cleanup`, plus the full flag matrix.
