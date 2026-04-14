---
name: decide
description: Structured decision-making with forced trade-off analysis. Creates a pros/cons tree, scores each factor, and recommends a choice with explicit reasoning. Use when the user says "should I", "which is better", "compare", "decide between", "pros and cons", or faces any choice between 2+ options.
---

# Decide

Make a structured decision by exploring trade-offs across all options.

## Protocol

1. **Clarify the options.** If the user hasn't listed specific options, ask. Need at least 2 options to compare.

2. **Check prior decisions.** Call `retrieve_context` with the decision topic — a similar decision may have been made before.

3. **Create the decision tree.** Call `create_tree` with the decision framed as a question (e.g., "Should we use PostgreSQL or DynamoDB for the user service?"). Use strategy "beam" for balanced comparison.

4. **Generate option branches.** At the root, call `generate_thoughts` with one thought per option. Each thought should be a neutral statement of what the option IS, not an argument for it.

5. **For each option, generate evaluation criteria.** Expand each option node with `generate_thoughts` covering:
   - Performance / speed
   - Cost / resources
   - Complexity / learning curve
   - Risk / failure modes
   - Scalability / future-proofing
   - Team fit / existing expertise
   
   Not all criteria apply to every decision — generate only what's relevant.

6. **Score honestly.** Call `evaluate_thought` for each criterion:
   - "sure" with score 0.8-1.0 — this option is clearly strong here
   - "maybe" with score 0.4-0.7 — mixed or uncertain
   - "impossible" with score 0.0-0.2 — this option is weak here

7. **Force trade-off articulation.** For the top 2 options, generate a thought that explicitly states: "Choosing [A] over [B] means accepting [downside] in exchange for [upside]." Evaluate this trade-off thought.

8. **Compare all paths.** Call `get_all_paths` to see the scored breakdown across all options and criteria.

9. **Mark the recommendation.** Call `mark_solution` on the best option's deepest evaluated node.

10. **Store the decision.** Call `store_solution` with tags including "decision" and the domain. Include the trade-off statement in the solution text.

## Presentation

Present the decision as a table:

```
| Criterion     | Option A (score) | Option B (score) |
|---------------|-------------------|-------------------|
| Performance   | Fast (0.9)        | Moderate (0.6)    |
| Cost          | Expensive (0.3)   | Cheap (0.8)       |
| ...           | ...               | ...               |
| AVERAGE       | 0.65              | 0.72              |
```

Then state: "Recommendation: [Option]. Trade-off: [what you give up] for [what you gain]."

## Guidelines

- Never skip the trade-off articulation step — it's the most valuable part
- If scores are within 0.1 of each other, say so explicitly — the decision is close
- Contradict your own initial intuition at least once (generate a thought favoring the option you'd otherwise dismiss)
