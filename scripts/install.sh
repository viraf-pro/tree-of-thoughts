#!/usr/bin/env bash
set -euo pipefail

# Install tot-mcp binary from GitHub releases or build from source.
# Called by SessionStart hook. Stores binary in CLAUDE_PLUGIN_DATA/bin/.

REPO="viraf-pro/tree-of-thoughts"
BINARY_NAME="tot-mcp"
INSTALL_DIR="${CLAUDE_PLUGIN_DATA}/bin"
PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT}"
VERSION_FILE="${CLAUDE_PLUGIN_DATA}/.installed-version"

mkdir -p "$INSTALL_DIR"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
esac

# Get latest release tag from GitHub
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/' || echo "")

# Check if already installed and up to date
if [ -f "$INSTALL_DIR/$BINARY_NAME" ] && [ -f "$VERSION_FILE" ]; then
  INSTALLED=$(cat "$VERSION_FILE")
  if [ "$INSTALLED" = "$LATEST" ] && [ -n "$LATEST" ]; then
    exit 0
  fi
fi

# Try downloading pre-built binary from GitHub releases
if [ -n "$LATEST" ]; then
  ARCHIVE="${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
  URL="https://github.com/${REPO}/releases/download/${LATEST}/${ARCHIVE}"

  if curl -fsSL "$URL" -o "/tmp/${ARCHIVE}" 2>/dev/null; then
    tar -xzf "/tmp/${ARCHIVE}" -C "$INSTALL_DIR" "$BINARY_NAME" 2>/dev/null && {
      chmod +x "$INSTALL_DIR/$BINARY_NAME"
      echo "$LATEST" > "$VERSION_FILE"
      rm -f "/tmp/${ARCHIVE}"
      exit 0
    }
    rm -f "/tmp/${ARCHIVE}"
  fi
fi

# Fallback: build from source (requires Go)
if command -v go &>/dev/null; then
  cd "$PLUGIN_ROOT"
  go build -ldflags "-s -w" -o "$INSTALL_DIR/$BINARY_NAME" .
  echo "built-from-source" > "$VERSION_FILE"
  exit 0
fi

echo "tot-mcp: could not download binary or build from source. Install Go or check https://github.com/${REPO}/releases" >&2
exit 1
