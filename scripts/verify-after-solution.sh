#!/usr/bin/env bash
set -euo pipefail

# PostToolUse hook for mark_solution: run verification checks on the tree.
# Reads hook input JSON from stdin.

TOT="${CLAUDE_PLUGIN_DATA}/bin/tot-mcp"
[ -x "$TOT" ] || exit 0

export TOT_DB_PATH="${TOT_DB_PATH:-${CLAUDE_PLUGIN_DATA}/tot.db}"
export TOT_NO_DASHBOARD=1

# Parse tree_id from the tool input
INPUT=$(cat)
TREE_ID=$(echo "$INPUT" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(d.get('tool_input', {}).get('tree_id', ''))
" 2>/dev/null) || exit 0

[ -z "$TREE_ID" ] && exit 0

# Get tree summary for verification
SUMMARY=$("$TOT" show "$TREE_ID" 2>/dev/null) || exit 0

# Extract stats
DEPTH=$(echo "$SUMMARY" | python3 -c "import json,sys; print(json.load(sys.stdin).get('stats',{}).get('maxDepthReached',0))" 2>/dev/null || echo "0")
TOTAL=$(echo "$SUMMARY" | python3 -c "import json,sys; print(json.load(sys.stdin).get('stats',{}).get('totalNodes',0))" 2>/dev/null || echo "0")
PRUNED=$(echo "$SUMMARY" | python3 -c "import json,sys; print(json.load(sys.stdin).get('stats',{}).get('prunedNodes',0))" 2>/dev/null || echo "0")
FRONTIER=$(echo "$SUMMARY" | python3 -c "import json,sys; print(json.load(sys.stdin).get('stats',{}).get('frontierSize',0))" 2>/dev/null || echo "0")

ISSUES=""

# Check depth
if [ "$DEPTH" -lt 2 ]; then
  ISSUES="${ISSUES}Shallow research (depth $DEPTH < 2). "
fi

# Check total nodes (at least some exploration)
if [ "$TOTAL" -lt 4 ]; then
  ISSUES="${ISSUES}Few nodes explored ($TOTAL < 4). "
fi

# Check frontier (unexplored nodes remaining)
if [ "$FRONTIER" -gt 3 ]; then
  ISSUES="${ISSUES}$FRONTIER frontier nodes still unexplored. "
fi

# Check pruning (no pruning = possibly no honest evaluation)
if [ "$TOTAL" -gt 5 ] && [ "$PRUNED" -eq 0 ]; then
  ISSUES="${ISSUES}No branches pruned — evaluations may be inflated. "
fi

if [ -n "$ISSUES" ]; then
  echo "Research verification warning for tree $TREE_ID:"
  echo "  $ISSUES"
  echo "  Consider expanding more branches before accepting this solution."
fi

exit 0
