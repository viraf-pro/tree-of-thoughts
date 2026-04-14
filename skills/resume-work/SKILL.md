---
name: resume-work
description: Resume work on a paused or abandoned tree with full context loading. Use when the user says "continue", "pick up where I left off", "resume", "what was I working on", or starts a new session and wants to continue previous work.
---

# Resume Work

Pick up a previous tree of thoughts with progressive context loading.

## Steps

1. **Find the tree.** Either:
   - The user provides a tree ID → use it directly
   - The user describes the problem → call `route_problem` to find the matching tree
   - The user says "continue" with no context → call `suggest_next` to get the recommendation

2. **Load context.** Call `get_tree_context` with `detail: "summary"` first:
   - Problem statement
   - Current status (paused/active/abandoned)
   - Best path so far
   - Frontier nodes available
   - Node counts

   Present this to the user so they can orient.

3. **Resume the tree.** If status is "paused" or "abandoned", call `resume_tree` to reactivate it.

4. **Check for related knowledge.** Call `retrieve_context` with the tree's problem to surface any solutions stored since the tree was last active.

5. **Check cross-links.** Call `get_tree_links` to see if other trees inform this one.

6. **Load full context if needed.** If the user needs deeper context, call `get_tree_context` with `detail: "full"` which adds:
   - All pruned branches and why they were abandoned
   - Complete path comparison
   - Cross-tree dependencies

7. **Present the decision point.** Show the user:
   - Where they left off (frontier nodes)
   - What the best path looks like so far
   - What remains unexplored
   - Any new context from retrieval

8. **Continue exploration.** Use the `explore-tree` workflow from here — generate thoughts, evaluate, expand.

## For Abandoned Trees

If the tree was abandoned, ask the user:
- Do they want to resume it (reactivate)?
- Or harvest the valuable thoughts into a new tree?
- Or link it to a new tree with `link_trees` type "informs"?
