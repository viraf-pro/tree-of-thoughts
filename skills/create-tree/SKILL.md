---
name: create-tree
description: Create a new Tree of Thoughts to explore a problem with structured multi-path reasoning. Use when the user wants to think deeply about a problem, compare approaches, or make a complex decision.
---

# Create a Tree of Thoughts

The user wants to explore a problem using Tree of Thoughts reasoning.

## Steps

1. Call the `route_problem` MCP tool first with the user's problem to check for existing trees
2. If `route_problem` returns `action: "continue"`, call `resume_tree` with the existing tree ID and show the user the current state
3. If `route_problem` returns `action: "create"`, call `create_tree` with the problem statement
4. After creating, confirm the tree ID and root node to the user
5. Ask if they'd like to start generating candidate thoughts

The default search strategy is "beam" (best-first). Use "bfs" for breadth-first exploration or "dfs" for depth-first when the user has a preference.
