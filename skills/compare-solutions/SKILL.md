---
name: compare-solutions
description: Side-by-side comparison of stored solutions. Surfaces trade-offs, contradictions, and evolution across related solutions. Use when the user says "compare solutions", "which solution is better", "what changed", or wants to understand differences between stored knowledge.
---

# Compare Solutions

Compare 2 or more stored solutions side by side.

## Steps

1. **Find the solutions.** Either:
   - User provides solution IDs → use directly
   - User describes a topic → call `retrieve_context` with the topic, `top_k: 5` to find candidates
   - User asks about contradictions → call `lint_knowledge` and look for contradiction warnings

2. **Load each solution.** For each solution, note:
   - Problem statement
   - Solution text
   - Score
   - Tags
   - Source tree (if any)
   - Rationale

3. **Check links between them.** Call `get_solution_links` for each solution to see if they're already cross-referenced (related, supersedes, contradicts, extends).

4. **Compare dimensions.** Present a structured comparison:
   - **Agreement** — where solutions say the same thing
   - **Disagreement** — where they contradict or differ
   - **Evolution** — if one supersedes or extends another, what changed
   - **Context** — if solutions apply to different contexts, clarify when each is appropriate

5. **Score comparison.** If solutions have different scores, explain what drove the difference (examine their source trees if available).

6. **Recommend links.** If solutions aren't already linked, suggest appropriate links:
   - "related" — they address overlapping problems
   - "supersedes" — newer solution replaces older one
   - "contradicts" — they recommend opposite approaches
   - "extends" — one builds on the other

   Offer to create the links with `link_solutions`.

7. **Synthesize.** If the user wants a unified answer, combine the best parts of each solution into a new insight.

## For Contradiction Resolution

When solutions contradict each other:

1. Identify the specific point of disagreement
2. Check if the contradiction is context-dependent (both right in different situations)
3. If one is strictly better, suggest marking the weaker as superseded
4. If both are valid in different contexts, create "related" links with notes explaining when each applies
