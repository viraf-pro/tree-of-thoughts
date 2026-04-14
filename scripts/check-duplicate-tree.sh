#!/usr/bin/env bash
set -euo pipefail

# PreToolUse hook for create_tree: check if route_problem was called first.
# Reads hook input JSON from stdin.
# Exit 0 = allow, exit 2 = block with message.

TOT="${CLAUDE_PLUGIN_DATA}/bin/tot-mcp"
[ -x "$TOT" ] || exit 0

export TOT_DB_PATH="${TOT_DB_PATH:-${CLAUDE_PLUGIN_DATA}/tot.db}"
export TOT_NO_DASHBOARD=1

# Parse the tool input from stdin
INPUT=$(cat)
PROBLEM=$(echo "$INPUT" | python3 -c "
import json, sys
d = json.load(sys.stdin)
args = d.get('tool_input', {})
print(args.get('problem', ''))
" 2>/dev/null) || exit 0

[ -z "$PROBLEM" ] && exit 0

# Route the problem to check for duplicates
ROUTE=$("$TOT" route "$PROBLEM" 2>/dev/null) || exit 0
ACTION=$(echo "$ROUTE" | python3 -c "import json,sys; print(json.load(sys.stdin).get('action','create'))" 2>/dev/null) || exit 0

if [ "$ACTION" = "continue" ]; then
  TREE_ID=$(echo "$ROUTE" | python3 -c "import json,sys; print(json.load(sys.stdin).get('treeId',''))" 2>/dev/null)
  EXISTING=$(echo "$ROUTE" | python3 -c "import json,sys; print(json.load(sys.stdin).get('problem','')[:80])" 2>/dev/null)
  echo "Duplicate tree warning: A similar tree already exists."
  echo "  Existing: $EXISTING"
  echo "  Tree ID:  $TREE_ID"
  echo "  Consider using resume_tree instead of create_tree."
  # Don't block — just warn. The agent can decide.
  exit 0
fi

exit 0
