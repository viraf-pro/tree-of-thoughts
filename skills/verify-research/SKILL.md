---
name: verify-research
description: Computational feedback sensor for research quality. Checks that a tree exploration was thorough — frontier explored, scores honest, solution stored. Runs automatically after researcher agent completes. Use after any research workflow to verify quality before accepting results.
---

# Verify Research (Feedback Sensor)

Fast, deterministic check that research was thorough. This is a SENSOR, not a skill — it checks, it doesn't explore.

## Checks (all computational, no inference needed)

Run these checks and report pass/fail for each:

### 1. Frontier exhaustion
Call `get_frontier` for the tree.
- **PASS:** No frontier nodes with score > 0.5 remain
- **WARN:** 1-2 frontier nodes with score > 0.5 unexplored
- **FAIL:** 3+ frontier nodes with score > 0.5 unexplored

### 2. Exploration depth
Call `get_tree_summary` for the tree.
- **PASS:** Max depth reached >= 3 (meaningful exploration)
- **WARN:** Max depth = 2 (shallow)
- **FAIL:** Max depth <= 1 (barely explored)

### 3. Branch diversity
Call `get_all_paths` for the tree.
- **PASS:** 3+ distinct paths explored
- **WARN:** 2 paths explored
- **FAIL:** Only 1 path explored (no comparison possible)

### 4. Pruning ratio
From tree summary, calculate: pruned / total nodes.
- **PASS:** Pruning ratio 10-50% (some paths rejected = honest evaluation)
- **WARN:** Pruning ratio < 10% (everything scored high = possible inflation)
- **WARN:** Pruning ratio > 50% (too much discarded = possible premature pruning)

### 5. Solution stored
Call `retrieve_context` with the tree's problem.
- **PASS:** A solution exists with matching tree_id
- **FAIL:** No solution stored (research done but not captured)

### 6. Score distribution
From all node evaluations, check distribution:
- **PASS:** Mix of sure/maybe/impossible scores (honest evaluation)
- **WARN:** All scores > 0.8 (suspiciously unanimous)
- **WARN:** All scores < 0.3 (nothing was promising — why continue?)

## Output

```
Research Verification: [tree_id]
  Frontier exhaustion:  PASS (0 nodes > 0.5 remaining)
  Exploration depth:    PASS (max depth 4)
  Branch diversity:     PASS (5 paths explored)
  Pruning ratio:        PASS (23% pruned)
  Solution stored:      PASS (sol-abc123)
  Score distribution:   PASS (3 sure, 4 maybe, 2 impossible)

  Overall: PASS (6/6)
```

If any check fails, include specific remediation:
- "FAIL: Frontier not exhausted → expand nodes [id1], [id2] before concluding"
- "FAIL: No solution stored → call store_solution with the best path"
