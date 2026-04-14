#!/usr/bin/env bash
set -euo pipefail

# SessionStart hook: briefing with knowledge health + priority work + compaction alerts.
# Output is injected as context at the start of every session.

TOT="${CLAUDE_PLUGIN_DATA}/bin/tot-mcp"
[ -x "$TOT" ] || exit 0

export TOT_DB_PATH="${TOT_DB_PATH:-${CLAUDE_PLUGIN_DATA}/tot.db}"
export TOT_NO_DASHBOARD=1

# Collect data (any failure is non-fatal — don't block session start)
SUGGEST=$("$TOT" suggest 2>/dev/null) || SUGGEST=""
HEALTH=$("$TOT" health 2>/dev/null) || HEALTH=""
DRIFT=$("$TOT" drift 2>/dev/null) || DRIFT=""
COMPACT=$("$TOT" compact 2>/dev/null) || COMPACT=""

# Build briefing
echo "--- Tree of Thoughts Session Briefing ---"

if [ -n "$SUGGEST" ]; then
  ACTION=$(echo "$SUGGEST" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('action','none'))" 2>/dev/null || echo "none")
  if [ "$ACTION" != "none" ]; then
    PROBLEM=$(echo "$SUGGEST" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('problem','')[:80])" 2>/dev/null || echo "")
    echo "Priority: $ACTION — $PROBLEM"
  else
    echo "Priority: No active trees. Ready for new work."
  fi
fi

if [ -n "$HEALTH" ]; then
  TREE_COUNT=$(echo "$HEALTH" | python3 -c "import json,sys; d=json.load(sys.stdin); t=d.get('trees',{}); print(f\"{t.get('active',0)} active, {t.get('paused',0)} paused, {t.get('solved',0)} solved\")" 2>/dev/null || echo "unknown")
  SOL_COUNT=$(echo "$HEALTH" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('solutions',{}).get('totalSolutions',0))" 2>/dev/null || echo "0")
  echo "Trees: $TREE_COUNT | Solutions: $SOL_COUNT"
fi

if [ -n "$DRIFT" ]; then
  DUPES=$(echo "$DRIFT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('duplicateTreePairs',[])))" 2>/dev/null || echo "0")
  NEVER=$(echo "$DRIFT" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('neverRetrievedSolutions',[])))" 2>/dev/null || echo "0")
  if [ "$DUPES" != "0" ] || [ "$NEVER" != "0" ]; then
    echo "Drift: $DUPES duplicate tree pairs, $NEVER never-retrieved solutions"
  fi
fi

if echo "$COMPACT" | grep -q "eligible"; then
  COUNT=$(echo "$COMPACT" | head -1 | grep -o '[0-9]*' | head -1)
  if [ -n "$COUNT" ] && [ "$COUNT" -gt 0 ]; then
    echo "Compaction: $COUNT solutions eligible (>30 days old)"
  fi
fi

echo "---"
