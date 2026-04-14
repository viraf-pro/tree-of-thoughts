---
name: explore-tree
description: Explore and expand an existing Tree of Thoughts. Generates candidate thoughts, evaluates them, and navigates the reasoning tree. Use when the user wants to continue working on a tree or dive deeper into a branch.
---

# Explore a Tree of Thoughts

The user wants to continue exploring a reasoning tree.

## Steps

1. Call `get_tree_context` with the tree ID and `detail: "summary"` to understand current state
2. Call `get_frontier` to see all expandable nodes ranked by score
3. Present the frontier to the user and ask which branch to explore (or use `search_step` to pick automatically)
4. For the chosen node, generate 3-5 candidate thoughts using `generate_thoughts`
5. Evaluate each thought using `evaluate_thought` with "sure", "maybe", or "impossible"
6. Prune dead ends with `backtrack` if evaluation is "impossible"
7. Repeat: call `search_step` for the next node to expand
8. When a satisfactory answer is found, call `mark_solution` on the terminal node
9. Call `store_solution` to save it for future retrieval

## Guidelines

- Generate diverse thoughts at each step, not variations of the same idea
- Be honest in evaluations: mark "impossible" early to avoid wasting depth
- Use `get_all_paths` to compare branches before marking a solution
- Use `retrieve_context` to check if similar problems were solved before
