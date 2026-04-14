#!/usr/bin/env bash
set -euo pipefail

# PostToolUse hook for store_solution / ingest_url: check for new orphans.
# Reads hook input JSON from stdin.

TOT="${CLAUDE_PLUGIN_DATA}/bin/tot-mcp"
[ -x "$TOT" ] || exit 0

export TOT_DB_PATH="${TOT_DB_PATH:-${CLAUDE_PLUGIN_DATA}/tot.db}"
export TOT_NO_DASHBOARD=1

# Run lint
LINT=$("$TOT" lint 2>/dev/null) || exit 0

ORPHANS=$(echo "$LINT" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(d.get('orphanSolutions', 0))
" 2>/dev/null) || exit 0

if [ "$ORPHANS" -gt 0 ]; then
  echo "Knowledge lint: $ORPHANS orphan solution(s) with no cross-references."
  echo "  Run retrieve_context on their problems to find connections, then link_solutions."
fi

exit 0
