---
name: suggest-next
description: Get a recommendation for what to work on next. Shows the most promising active tree and frontier nodes. Use when starting a new session or when the current task is done.
---

# Suggest Next Work

The user wants to know what to work on next.

## Steps

1. Call the `suggest_next` MCP tool to get the recommendation
2. If it suggests resuming a tree, call `get_tree_context` with the tree ID to get full context
3. Present the suggestion clearly: the problem, current state, and what to do next
4. If there's a frontier node to expand, offer to generate thoughts for it
