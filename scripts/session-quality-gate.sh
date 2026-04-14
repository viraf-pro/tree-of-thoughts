#!/usr/bin/env bash
set -euo pipefail

# Stop hook: end-of-session quality gate.
# Checks if solutions were stored during this session and runs verification.

TOT="${CLAUDE_PLUGIN_DATA}/bin/tot-mcp"
[ -x "$TOT" ] || exit 0

export TOT_DB_PATH="${TOT_DB_PATH:-${CLAUDE_PLUGIN_DATA}/tot.db}"
export TOT_NO_DASHBOARD=1

# Check recent knowledge events (last 5 minutes worth)
EVENTS=$("$TOT" report 2>/dev/null) || exit 0

# Quick lint check
LINT=$("$TOT" lint 2>/dev/null) || exit 0
ORPHANS=$(echo "$LINT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('orphanSolutions',0))" 2>/dev/null || echo "0")
ISSUES=$(echo "$LINT" | python3 -c "import json,sys; print(json.load(sys.stdin).get('totalIssues',0))" 2>/dev/null || echo "0")

if [ "$ISSUES" -gt 0 ]; then
  echo "Session quality gate: $ISSUES knowledge issue(s) detected ($ORPHANS orphans)."
  echo "  Run /tree-of-thoughts:knowledge-health for details and fixes."
fi

exit 0
