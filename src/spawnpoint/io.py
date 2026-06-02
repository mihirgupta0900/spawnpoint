"""Helpers for non-interactive (agent-friendly) command execution.

Each command threads a ``no_input`` flag through its prompt sites. When set,
these helpers satisfy what would otherwise be an interactive prompt from flags,
and fail fast (non-zero exit, clear message) instead of hanging on a TTY.
"""

import json as _json
from typing import Dict, List, Optional

import typer
from rich.console import Console

# Plain stdout console for machine-readable / capturable output.
stdout_console = Console()


def parse_csv(value: Optional[str]) -> List[str]:
    """Split a comma-separated flag value into trimmed, non-empty parts."""
    if not value:
        return []
    return [part.strip() for part in value.split(",") if part.strip()]


def require(value: Optional[str], flag: str, err: Console) -> str:
    """Return ``value`` or exit with an error naming the missing flag."""
    if value is None or (isinstance(value, str) and not value.strip()):
        err.print(f"[bold red]Error:[/bold red] --no-input requires {flag}.")
        raise typer.Exit(code=1)
    return value


def resolve_names(
    requested: List[str],
    name_to_value: Dict[str, object],
    *,
    kind: str,
    err: Console,
    aliases: Optional[Dict[str, List[str]]] = None,
) -> List[object]:
    """Resolve requested names against available choices.

    ``name_to_value`` maps the canonical display name to its value. ``aliases``
    optionally maps an alternate name (e.g. a bare repo dir name) to the list of
    canonical names it could refer to — used to detect ambiguity.

    Exits non-zero on any unknown name or ambiguous bare name, listing the valid
    choices so an agent can correct its call.
    """
    resolved: List[object] = []
    for name in requested:
        if name in name_to_value:
            resolved.append(name_to_value[name])
            continue
        if aliases and name in aliases:
            matches = aliases[name]
            if len(matches) > 1:
                err.print(
                    f"[bold red]Error:[/bold red] {kind} '{name}' is ambiguous; "
                    f"matches: {', '.join(matches)}. Use the full name."
                )
                raise typer.Exit(code=1)
            resolved.append(name_to_value[matches[0]])
            continue
        err.print(
            f"[bold red]Error:[/bold red] {kind} '{name}' not found. "
            f"Valid: {', '.join(sorted(name_to_value)) or '(none)'}"
        )
        raise typer.Exit(code=1)
    return resolved


def emit_json(payload: object) -> None:
    """Print a JSON payload to stdout for machine consumption."""
    stdout_console.print_json(_json.dumps(payload, default=str))
