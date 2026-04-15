#!/usr/bin/env bash
set -euo pipefail

# Launcher for tot-mcp MCP server.
# Downloads the binary on first run, then exec's it.
# Used as the mcpServers command in plugin.json so the MCP server
# is available immediately after plugin install — no restart needed.

BINARY="${CLAUDE_PLUGIN_DATA}/bin/tot-mcp"

# If binary doesn't exist, run the install script
if [ ! -f "$BINARY" ]; then
  "${CLAUDE_PLUGIN_ROOT}/scripts/install.sh"
fi

# Exec replaces this process with tot-mcp
exec "$BINARY" "$@"
