#!/usr/bin/env bash
set -euo pipefail

# Install tot-mcp binary from GitHub releases or build from source.
# Called by SessionStart hook and launch.sh. Stores binary in CLAUDE_PLUGIN_DATA/bin/.
#
# The binary version is pinned to the plugin version in plugin.json to keep
# the binary, skills, agents, and hooks in sync. When the marketplace updates
# the plugin files, the next session start downloads the matching binary.

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

# Read target version from plugin.json to stay in sync with plugin files
PLUGIN_JSON="${PLUGIN_ROOT}/.claude-plugin/plugin.json"
if [ -f "$PLUGIN_JSON" ]; then
  TARGET=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/v\1/p' "$PLUGIN_JSON" | head -1)
fi

# Fallback to latest release if plugin.json is missing or unparseable
if [ -z "${TARGET:-}" ]; then
  TARGET=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1 || echo "")
fi

# Check if already installed and at the correct version
if [ -f "$INSTALL_DIR/$BINARY_NAME" ] && [ -f "$VERSION_FILE" ]; then
  INSTALLED=$(cat "$VERSION_FILE")
  if [ "$INSTALLED" = "$TARGET" ] && [ -n "$TARGET" ]; then
    exit 0
  fi
fi

# Try downloading pre-built binary from GitHub releases
if [ -n "$TARGET" ]; then
  ARCHIVE="${BINARY_NAME}-${OS}-${ARCH}.tar.gz"
  URL="https://github.com/${REPO}/releases/download/${TARGET}/${ARCHIVE}"

  if curl -fsSL "$URL" -o "/tmp/${ARCHIVE}" 2>/dev/null; then
    tar -xzf "/tmp/${ARCHIVE}" -C "$INSTALL_DIR" "$BINARY_NAME" 2>/dev/null && {
      chmod +x "$INSTALL_DIR/$BINARY_NAME"
      echo "$TARGET" > "$VERSION_FILE"
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
