#!/bin/sh
# Install spawnpoint from GitHub releases.
#
#   curl -fsSL https://raw.githubusercontent.com/mihirgupta0900/spawnpoint/main/install.sh | sh
#
# Environment:
#   SPAWNPOINT_VERSION      version to install (default: latest), e.g. 1.0.0
#   SPAWNPOINT_INSTALL_DIR  where to put the binary (default: ~/.local/bin)
#   SPAWNPOINT_RELEASES_URL release base URL (default: GitHub; for mirrors/tests)
set -eu

REPO="mihirgupta0900/spawnpoint"
BASE="${SPAWNPOINT_RELEASES_URL:-https://github.com/$REPO/releases}"
DIR="${SPAWNPOINT_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*" >&2; }
die() { say "error: $*"; exit 1; }

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) die "unsupported OS $(uname -s); download a build from https://github.com/$REPO/releases" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) die "unsupported CPU $(uname -m)" ;;
esac

if [ -n "${SPAWNPOINT_VERSION:-}" ]; then
  url="$BASE/download/v${SPAWNPOINT_VERSION#v}"
else
  url="$BASE/latest/download"
fi
asset="spawnpoint_${os}_${arch}.tar.gz"

# An existing pipx/uv install owns ~/.local/bin/spawnpoint; upgrading it in
# place keeps a single install (it gets the same Go binary).
existing="$DIR/spawnpoint"
if [ -L "$existing" ] && [ -z "${SPAWNPOINT_FORCE:-}" ]; then
  env_dir=$(dirname "$(dirname "$(readlink "$existing")")")
  if [ -f "$env_dir/pipx_metadata.json" ]; then
    die "spawnpoint is installed with pipx. Upgrade it with: pipx upgrade spawnpoint
       (or run 'pipx uninstall spawnpoint' first, or set SPAWNPOINT_FORCE=1)"
  elif [ -f "$env_dir/uv-receipt.toml" ]; then
    die "spawnpoint is installed with uv. Upgrade it with: uv tool upgrade spawnpoint
       (or run 'uv tool uninstall spawnpoint' first, or set SPAWNPOINT_FORCE=1)"
  fi
fi

if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -qO "$2" "$1"; }
else
  die "need curl or wget"
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

say "Downloading $asset..."
fetch "$url/$asset" "$tmp/$asset" || die "download failed: $url/$asset"
fetch "$url/checksums.txt" "$tmp/checksums.txt" || die "download failed: $url/checksums.txt"

want=$(awk -v f="$asset" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || die "no checksum for $asset"
if command -v sha256sum >/dev/null 2>&1; then
  got=$(sha256sum "$tmp/$asset" | awk '{ print $1 }')
else
  got=$(shasum -a 256 "$tmp/$asset" | awk '{ print $1 }')
fi
[ "$want" = "$got" ] || die "checksum mismatch for $asset"

tar -xzf "$tmp/$asset" -C "$tmp" spawnpoint
mkdir -p "$DIR"
mv -f "$tmp/spawnpoint" "$DIR/spawnpoint"
chmod 755 "$DIR/spawnpoint"
ln -sf spawnpoint "$DIR/sp"

say "Installed $("$DIR/spawnpoint" --version 2>/dev/null || echo spawnpoint) to $DIR (also as 'sp')"
case ":$PATH:" in
  *":$DIR:"*) ;;
  *) say "Add $DIR to your PATH, e.g.: echo 'export PATH=\"$DIR:\$PATH\"' >> ~/.zshrc" ;;
esac
say "Get started: sp init"
