#!/bin/sh
set -e

# Spawnpoint installer
# Usage: curl -fsSL https://raw.githubusercontent.com/mihirgupta0900/spawnpoint/main/install.sh | sh

log()  { printf "\033[0;32m=>\033[0m %s\n" "$1" >&2; }
warn() { printf "\033[1;33mwarning:\033[0m %s\n" "$1" >&2; }
err()  { printf "\033[0;31merror:\033[0m %s\n" "$1" >&2; exit 1; }

PYTHON=""
INSTALLER=""

check_python() {
    for cmd in python3 python; do
        if command -v "$cmd" >/dev/null 2>&1; then
            ver=$("$cmd" -c "import sys; print(f'{sys.version_info.major}.{sys.version_info.minor}')")
            major=$(echo "$ver" | cut -d. -f1)
            minor=$(echo "$ver" | cut -d. -f2)
            if [ "$major" -ge 3 ] && [ "$minor" -ge 10 ]; then
                PYTHON="$cmd"
                log "Found $cmd ($ver)"
                return 0
            fi
        fi
    done
    err "Python 3.10+ is required. Install from https://python.org"
}

ensure_pipx() {
    if command -v pipx >/dev/null 2>&1; then
        INSTALLER="pipx"
        return 0
    fi
    log "pipx not found. Installing pipx..."
    "$PYTHON" -m pip install --user pipx 2>/dev/null && {
        # Ensure pipx is on PATH
        "$PYTHON" -m pipx ensurepath 2>/dev/null || true
        INSTALLER="pipx"
        return 0
    }
    warn "Could not install pipx. Falling back to pip."
    INSTALLER="pip"
}

install_spawnpoint() {
    check_python
    ensure_pipx

    if [ "$INSTALLER" = "pipx" ]; then
        log "Installing spawnpoint via pipx..."
        if command -v pipx >/dev/null 2>&1; then
            pipx install spawnpoint
        else
            "$PYTHON" -m pipx install spawnpoint
        fi
    else
        log "Installing spawnpoint via pip..."
        "$PYTHON" -m pip install --user spawnpoint
    fi
}

verify() {
    echo ""
    if command -v spawnpoint >/dev/null 2>&1; then
        log "spawnpoint $(spawnpoint --version 2>/dev/null || echo '') installed successfully!"
    else
        warn "spawnpoint installed but not found in PATH."
        warn "You may need to add ~/.local/bin to your PATH:"
        echo ""
        case "$SHELL" in
            */zsh)  echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.zshrc && source ~/.zshrc" ;;
            */bash) echo "  echo 'export PATH=\"\$HOME/.local/bin:\$PATH\"' >> ~/.bashrc && source ~/.bashrc" ;;
            */fish) echo "  fish_add_path ~/.local/bin" ;;
            *)      echo "  export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
        esac
        echo ""
    fi

    echo "  Next steps:"
    echo "    spawnpoint create    # spawn worktree workspaces"
    echo "    spawnpoint cleanup   # remove worktree workspaces"
    echo "    spawnpoint --help    # see all commands"
    echo ""
}

install_spawnpoint
verify
