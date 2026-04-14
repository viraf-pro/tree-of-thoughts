---
name: audit-trail
description: Trace the reasoning history behind a decision or tree. Shows what was tried, what was rejected, and why. Use when the user says "why did we decide", "what happened", "trace", "show history", "what was the reasoning", or needs to understand past decisions.
---

# Audit Trail

Reconstruct the reasoning history behind a decision, tree, or solution.

## Steps

1. **Find the target.** Either:
   - User provides a tree ID → use directly
   - User describes the topic → call `route_problem` to find the tree
   - User asks about a solution → call `retrieve_context` to find it, then look up its tree_id

2. **Load full context.** Call `get_tree_context` with `detail: "full"` to get:
   - Complete exploration history
   - All paths (including pruned branches)
   - Cross-tree links
   - Decision timeline

3. **Get the audit log.** Call `audit_log` with the tree_id to see:
   - Every tool call that touched this tree
   - Timestamps showing the sequence of exploration
   - What was generated, evaluated, pruned, and solved

4. **Get all paths.** Call `get_all_paths` to see every branch that was explored, including dead ends.

5. **Check knowledge connections.** Call `get_tree_links` to see if other trees informed this one. Call `get_solution_links` if there's a stored solution.

6. **Reconstruct the narrative.** Present the history as a story:
   - "First, we considered [N] approaches: [list]"
   - "[Branch X] was pruned because [evaluation reason]"
   - "[Branch Y] looked promising but scored [score] on [criterion]"
   - "The winning path was [path] with average score [score]"
   - "Trade-off accepted: [what was given up]"

7. **Highlight the key decision points.** For each node where multiple children were generated, show which child was chosen and why (based on scores).

## For Solutions

If the user asks about a specific stored solution:

1. Call `retrieve_context` to find it
2. Read its `rationale` field — this auto-generated text explains why it was chosen
3. Look up its source tree for the full exploration context
4. Call `get_solution_links` to show what it relates to, supersedes, or contradicts
