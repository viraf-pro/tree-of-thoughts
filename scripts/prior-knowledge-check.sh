#!/usr/bin/env bash
set -euo pipefail

# UserPromptSubmit hook: check if the user's prompt matches existing solutions.
# Injects prior knowledge as context so the agent doesn't re-research.
# Reads hook input JSON from stdin.

TOT="${CLAUDE_PLUGIN_DATA}/bin/tot-mcp"
[ -x "$TOT" ] || exit 0

export TOT_DB_PATH="${TOT_DB_PATH:-${CLAUDE_PLUGIN_DATA}/tot.db}"
export TOT_NO_DASHBOARD=1

# Extract the user's prompt
INPUT=$(cat)
PROMPT=$(echo "$INPUT" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(d.get('prompt', '')[:200])
" 2>/dev/null) || exit 0

# Skip short prompts, commands, and non-question prompts
[ ${#PROMPT} -lt 20 ] && exit 0
echo "$PROMPT" | grep -q '^/' && exit 0

# Search for matching solutions (quick keyword search)
RESULTS=$(echo "$PROMPT" | head -1 | xargs -I{} "$TOT" stats 2>/dev/null) || exit 0

# Only check if we have solutions in the store
SOL_COUNT=$(echo "$RESULTS" | python3 -c "import json,sys; print(json.load(sys.stdin).get('totalSolutions',0))" 2>/dev/null || echo "0")
[ "$SOL_COUNT" -eq 0 ] && exit 0

# Check for matching tree
ROUTE=$("$TOT" route "$PROMPT" 2>/dev/null) || exit 0
ACTION=$(echo "$ROUTE" | python3 -c "import json,sys; print(json.load(sys.stdin).get('action','create'))" 2>/dev/null) || exit 0

if [ "$ACTION" = "continue" ]; then
  TREE_ID=$(echo "$ROUTE" | python3 -c "import json,sys; print(json.load(sys.stdin).get('treeId',''))" 2>/dev/null)
  PROBLEM=$(echo "$ROUTE" | python3 -c "import json,sys; print(json.load(sys.stdin).get('problem','')[:80])" 2>/dev/null)
  echo "Prior knowledge: An existing tree matches this topic."
  echo "  Tree: $TREE_ID"
  echo "  Problem: $PROBLEM"
  echo "  Consider using /tree-of-thoughts:resume-work instead of starting fresh."
fi

exit 0
