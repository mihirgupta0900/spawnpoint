import tomllib
from dataclasses import dataclass, field
from pathlib import Path
from typing import List

SPAWNPOINT_DIR = Path.home() / ".spawnpoint"
CONFIG_PATH = SPAWNPOINT_DIR / "config.toml"

COMMON_CODE_DIRS = ["~/code", "~/projects", "~/repos", "~/src", "~/dev"]

DEFAULT_COPY_PATTERNS_GLOBS = [".env*"]
DEFAULT_COPY_PATTERNS_FILES = ["AGENT.md", "CLAUDE.md", "GEMINI.md"]
DEFAULT_COPY_PATTERNS_DIRS = [".vscode", "docs"]

@dataclass
class Config:
    scan_dirs: List[Path] = field(default_factory=list)
    worktree_dir: Path = field(default_factory=lambda: SPAWNPOINT_DIR / "workspaces")
    scan_depth: int = 2
    copy_patterns_globs: List[str] = field(default_factory=lambda: list(DEFAULT_COPY_PATTERNS_GLOBS))
    copy_patterns_files: List[str] = field(default_factory=lambda: list(DEFAULT_COPY_PATTERNS_FILES))
    copy_patterns_dirs: List[str] = field(default_factory=lambda: list(DEFAULT_COPY_PATTERNS_DIRS))
    additional_worktree_dirs: List[Path] = field(default_factory=list)
    auto_install_deps: bool = True


def expand_path(p: str) -> Path:
    return Path(p).expanduser().resolve()


def detect_scan_dirs() -> List[Path]:
    """Return common code directories that actually exist on this machine."""
    found = []
    for d in COMMON_CODE_DIRS:
        p = Path(d).expanduser()
        if p.is_dir():
            found.append(p)
    return found


def config_exists() -> bool:
    return CONFIG_PATH.is_file()


def load_config() -> Config:
    """Load config from TOML, merging with defaults."""
    if not CONFIG_PATH.is_file():
        return Config()

    with open(CONFIG_PATH, "rb") as f:
        data = tomllib.load(f)

    cfg = Config()

    if "scan_dirs" in data:
        cfg.scan_dirs = [expand_path(d) for d in data["scan_dirs"]]
    if "worktree_dir" in data:
        cfg.worktree_dir = expand_path(data["worktree_dir"])
    if "scan_depth" in data:
        cfg.scan_depth = int(data["scan_depth"])
    if "copy_patterns_globs" in data:
        cfg.copy_patterns_globs = list(data["copy_patterns_globs"])
    if "copy_patterns_files" in data:
        cfg.copy_patterns_files = list(data["copy_patterns_files"])
    if "copy_patterns_dirs" in data:
        cfg.copy_patterns_dirs = list(data["copy_patterns_dirs"])
    if "additional_worktree_dirs" in data:
        cfg.additional_worktree_dirs = [expand_path(d) for d in data["additional_worktree_dirs"]]
    if "auto_install_deps" in data:
        cfg.auto_install_deps = bool(data["auto_install_deps"])

    return cfg


def save_config(cfg: Config) -> Path:
    """Write config to TOML file. Returns the config path."""
    SPAWNPOINT_DIR.mkdir(parents=True, exist_ok=True)

    def path_str(p: Path) -> str:
        """Convert path to ~ notation for readability."""
        home = Path.home()
        try:
            return "~/" + str(p.relative_to(home))
        except ValueError:
            return str(p)

    lines = [
        "# Spawnpoint configuration",
        "# https://github.com/mihirgupta0900/spawnpoint",
        "",
        "# Directories to scan for git repos",
        f"scan_dirs = [{', '.join(repr(path_str(d)) for d in cfg.scan_dirs)}]",
        "",
        "# Where workspaces are created",
        f"worktree_dir = {repr(path_str(cfg.worktree_dir))}",
        "",
        "# Additional directories to scan during cleanup (for worktrees created at previous locations)",
        f"additional_worktree_dirs = [{', '.join(repr(path_str(d)) for d in cfg.additional_worktree_dirs)}]",
        "",
        "# How deep to scan for repos (1-4)",
        f"scan_depth = {cfg.scan_depth}",
        "",
        "# Glob patterns for files to copy into new worktrees",
        f"copy_patterns_globs = {repr(cfg.copy_patterns_globs)}",
        "",
        "# Specific files to copy",
        f"copy_patterns_files = {repr(cfg.copy_patterns_files)}",
        "",
        "# Directories to copy",
        f"copy_patterns_dirs = {repr(cfg.copy_patterns_dirs)}",
        "",
        "# Auto-install dependencies after worktree creation",
        f"auto_install_deps = {'true' if cfg.auto_install_deps else 'false'}",
        "",
    ]

    CONFIG_PATH.write_text("\n".join(lines))
    return CONFIG_PATH
