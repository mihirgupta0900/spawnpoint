"""Non-blocking update checker. Runs PyPI lookup in a background thread."""

from __future__ import annotations

import json
import threading
import time
import urllib.request
from pathlib import Path

from .config import SPAWNPOINT_DIR

_CACHE_PATH = SPAWNPOINT_DIR / ".version_cache.json"
_CACHE_TTL = 86400  # 24 hours
_PYPI_URL = "https://pypi.org/pypi/spawnpoint/json"

_thread: threading.Thread | None = None
_latest_version: str | None = None
_current_version: str | None = None


def _read_cache() -> str | None:
    """Return cached latest version if fresh, else None."""
    try:
        data = json.loads(_CACHE_PATH.read_text())
        if time.time() - data["checked_at"] < _CACHE_TTL:
            return data["latest_version"]
    except Exception:
        pass
    return None


def _write_cache(latest: str) -> None:
    try:
        SPAWNPOINT_DIR.mkdir(parents=True, exist_ok=True)
        _CACHE_PATH.write_text(json.dumps({
            "latest_version": latest,
            "checked_at": time.time(),
        }))
    except Exception:
        pass


def _fetch_latest() -> str | None:
    try:
        req = urllib.request.Request(_PYPI_URL, headers={"Accept": "application/json"})
        with urllib.request.urlopen(req, timeout=3) as resp:
            data = json.loads(resp.read())
        return data["info"]["version"]
    except Exception:
        return None


def _check() -> None:
    global _latest_version
    cached = _read_cache()
    if cached:
        _latest_version = cached
        return
    latest = _fetch_latest()
    if latest:
        _latest_version = latest
        _write_cache(latest)


def start_check(current_version: str) -> None:
    """Kick off background version check. Call early in CLI startup."""
    global _thread, _current_version
    _current_version = current_version
    _thread = threading.Thread(target=_check, daemon=True)
    _thread.start()


def get_update_notice() -> str | None:
    """Join bg thread (short timeout) and return a notice string, or None."""
    if _thread is None:
        return None
    _thread.join(timeout=0.5)
    if _latest_version is None or _current_version is None:
        return None
    if _latest_version == _current_version:
        return None
    # Compare versions to avoid false positives on dev/pre-release builds
    try:
        cur = tuple(int(x) for x in _current_version.split(".")[:3])
        lat = tuple(int(x) for x in _latest_version.split(".")[:3])
        if lat <= cur:
            return None
    except (ValueError, AttributeError):
        pass
    return (
        f"[dim]Update available: {_current_version} → {_latest_version}"
        f"  (run [bold]sp update[/bold] to upgrade)[/dim]"
    )
