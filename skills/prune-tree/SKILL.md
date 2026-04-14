---
name: prune-tree
description: Systematic cleanup of a tree — prune low-scoring branches, remove dead weight, and tighten the reasoning structure. Use when the user says "clean up", "prune", "simplify tree", or when a tree has too many branches and needs focus.
---

# Prune Tree

Systematically clean up a reasoning tree by removing low-value branches.

## Steps

1. **Load tree state.** Call `get_tree_context` with `detail: "full"` to see all branches.

2. **Identify prune candidates.** Call `get_all_paths` and `get_frontier` to find:
   - Branches with average score < 0.3 (clearly weak)
   - Frontier nodes with score < 0.3 that haven't been expanded
   - Branches marked "maybe" that were never expanded (stale exploration)
   - Duplicate branches (same idea explored twice)

3. **Present the prune plan.** Show the user what will be removed:
   ```
   Prune candidates:
   1. [node-id] "Python approach" (score 0.2) — 3 descendants
   2. [node-id] "TypeScript/Deno" (score 0.15) — 1 descendant
   3. [node-id] "Unexplored maybe" (score 0.5, no children, stale)
   
   This will remove 5 nodes, keeping 8 of 13.
   ```

4. **Confirm with user.** Never auto-prune without confirmation.

5. **Execute pruning.** For each confirmed candidate, call `backtrack` with the node ID. This recursively removes the node and all descendants.

6. **Verify results.** Call `get_tree_summary` to show the cleaned-up state:
   - Before: X nodes, Y frontier
   - After: X' nodes, Y' frontier
   - Removed: Z nodes across N branches

## Guidelines

- Never prune branches with score > 0.5 without explicit user approval
- Never prune the only remaining branch (leave at least one path)
- If a branch has a terminal/solution node, warn before pruning — this deletes a found solution
- Stale frontier nodes (unexpanded for a long time) are good prune candidates
- After pruning, the tree should be more focused with a clearer best path
