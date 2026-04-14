---
name: deep-research
description: Conduct deep research on a topic using Tree of Thoughts. Explores multiple branches exhaustively before concluding. Use when the user needs thorough analysis, wants to compare all options, or says "research", "deep dive", "analyze thoroughly", or "compare approaches".
---

# Deep Research

Conduct exhaustive multi-path research on a topic. Unlike quick answers, deep research explores ALL viable branches before drawing conclusions.

## Protocol

1. **Check prior knowledge.** Call `retrieve_context` with the problem statement to find relevant past solutions. If a high-scoring match exists (score > 0.8), present it and ask if the user wants fresh research or to build on it.

2. **Route the problem.** Call `route_problem` to check for existing trees. If one exists, call `get_tree_context` with `detail: "full"` to understand what was already explored.

3. **Create or resume tree.** Either `create_tree` (strategy: "beam") or `resume_tree`.

4. **Generate breadth.** At the root, call `generate_thoughts` with 4-5 diverse candidate approaches. These should be genuinely different angles, not variations of the same idea.

5. **Evaluate all candidates.** Call `evaluate_thought` for each candidate with honest assessments:
   - "sure" (0.8-1.0) — strong evidence this direction works
   - "maybe" (0.4-0.7) — plausible but uncertain
   - "impossible" (0.0) — dead end, will be pruned

6. **Go deep on promising branches.** For each "sure" and "maybe" node:
   - Call `search_step` to get the next node
   - Generate 2-3 sub-thoughts with `generate_thoughts`
   - Evaluate each
   - Repeat until depth 3-4 or until a clear answer emerges

7. **Compare all paths.** Call `get_all_paths` to see every explored branch ranked by score. Call `get_frontier` to check if unexplored nodes remain.

8. **Do NOT conclude prematurely.** If frontier nodes with score > 0.5 remain unexplored, expand them before concluding.

9. **Mark solution.** Call `mark_solution` on the best terminal node.

10. **Store for future use.** Call `store_solution` with descriptive tags so this research compounds.

## Guidelines

- Spend at least 3 rounds of generate-evaluate-expand before concluding
- Always check `get_frontier` before declaring "done" — unexplored nodes may hold better answers
- Use `retrieve_context` mid-research if a sub-problem reminds you of prior work
- Generate thoughts that DISAGREE with each other — consensus too early means shallow research
