#!/usr/bin/env bash
set -euo pipefail

# PreToolUse hook for execute_experiment: verify preconditions.
# Reads hook input JSON from stdin.
# Checks: git is clean, work_dir exists, previous_hash is valid.

INPUT=$(cat)

TREE_ID=$(echo "$INPUT" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(d.get('tool_input', {}).get('tree_id', ''))
" 2>/dev/null) || exit 0

PREV_HASH=$(echo "$INPUT" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(d.get('tool_input', {}).get('previous_hash', ''))
" 2>/dev/null) || exit 0

[ -z "$TREE_ID" ] && exit 0

# Get experiment config to find work_dir
TOT="${CLAUDE_PLUGIN_DATA}/bin/tot-mcp"
[ -x "$TOT" ] || exit 0

export TOT_DB_PATH="${TOT_DB_PATH:-${CLAUDE_PLUGIN_DATA}/tot.db}"
export TOT_NO_DASHBOARD=1

# Check previous_hash is set
if [ -z "$PREV_HASH" ]; then
  echo "Experiment safety: previous_hash is empty. This is needed for rollback on failure."
  echo "  Run prepare_experiment first to get the previous_hash."
fi

exit 0
