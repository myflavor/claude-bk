#!/bin/bash
# Install claude-bk from GitHub Releases
# Usage: curl -sSL https://raw.githubusercontent.com/myflavor/claude-bk/main/scripts/install.sh | bash
#
# Downloads the per-platform binary (claude-bk-<os>-<arch>) and places it next
# to claude.
set -e

REPO="myflavor/claude-bk"

# Detect OS and arch, map to the release binary name
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)  ARCH=amd64 ;;
    aarch64|arm64) ARCH=arm64 ;;
    *) echo "Unsupported arch: $ARCH"; exit 1 ;;
esac

case "$OS" in
    linux|darwin) BIN="claude-bk-$OS-$ARCH" ;;
    *) echo "Unsupported platform: $OS-$ARCH"; exit 1 ;;
esac

# Find claude location (binary installs next to it)
CLAUDE_PATH=$(command -v claude)
if [ -z "$CLAUDE_PATH" ]; then
    echo "Error: claude command not found in PATH"
    echo "Please install Claude Code first: https://code.claude.com"
    exit 1
fi

CLAUDE_DIR=$(cd "$(dirname "$CLAUDE_PATH")" && pwd)
BK_TARGET="$CLAUDE_DIR/claude-bk"

# Check write permission on any existing binary (e.g. previously installed with sudo)
if [ -f "$BK_TARGET" ] && [ ! -w "$BK_TARGET" ]; then
    echo "Error: no write permission to $BK_TARGET"
    echo "Try running with sudo, or install claude to a user-writable location"
    exit 1
fi

TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

echo "Downloading $BIN from $REPO..."
if ! curl -sSL -f -o "$TMP_DIR/claude-bk" "https://github.com/$REPO/releases/latest/download/$BIN"; then
    echo "Error: download failed for $BIN"
    exit 1
fi
if [ ! -s "$TMP_DIR/claude-bk" ]; then
    echo "Error: downloaded file is empty: $BIN"
    exit 1
fi
chmod +x "$TMP_DIR/claude-bk"

# Atomic install: stage in the target dir, then rename (same filesystem).
tmp="$BK_TARGET.new.$$"
cp -f "$TMP_DIR/claude-bk" "$tmp" || { rm -f "$tmp"; echo "Error: failed to stage $BK_TARGET"; exit 1; }
mv -f "$tmp" "$BK_TARGET" || { rm -f "$tmp"; echo "Error: failed to install $BK_TARGET"; exit 1; }

HOME_DIR="${HOME:-$(eval echo ~$(id -un))}"
PROVIDERS_DIR="$HOME_DIR/.claude-bk"

if [ ! -d "$PROVIDERS_DIR" ]; then
    mkdir -p "$PROVIDERS_DIR"
    chmod 700 "$PROVIDERS_DIR"
    echo ""
    echo "Created providers directory: $PROVIDERS_DIR"
    echo "Add a provider by placing a config file at: $PROVIDERS_DIR/<name>.json"
fi

echo ""
echo "Installed to: $BK_TARGET"
echo "Test it with: cd <your-project> && claude-bk <provider-name> push"
