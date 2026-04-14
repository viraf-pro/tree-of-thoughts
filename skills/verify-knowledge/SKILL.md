---
name: verify-knowledge
description: Computational feedback sensor for knowledge store quality. Checks that stored solutions have tags, links, embeddings, and no orphans were created. Runs after librarian agent completes.
---

# Verify Knowledge (Feedback Sensor)

Fast check that knowledge operations maintained store quality.

## Checks

### 1. No new orphans
Call `lint_knowledge` and count orphan solutions.
- **PASS:** No orphans (all solutions have at least one link)
- **WARN:** 1-2 orphans (may be newly ingested, not yet linked)
- **FAIL:** 3+ orphans (linking step was skipped)

### 2. Tag coverage
From `knowledge_report`, check tag distribution:
- **PASS:** Newly stored solutions have tags
- **WARN:** Solutions stored with only generic tags ("ingested")
- **FAIL:** Solutions stored with no tags

### 3. Embedding presence
From `retrieval_stats`, check embedding ratio:
- **PASS:** Embedding ratio matches embedding provider availability
- **WARN:** Embedding ratio dropped after recent operations
- **FAIL:** New solutions missing embeddings when provider is active

### 4. No contradictions introduced
From `lint_knowledge`, check for new contradictions:
- **PASS:** No new contradiction warnings
- **WARN:** New potential contradictions detected (may be intentional)
- **INFO:** Existing contradictions resolved

### 5. Knowledge event logged
From `knowledge_log` (limit 5), check recent events:
- **PASS:** Recent store/link/compact events present
- **WARN:** Operations happened but no knowledge events logged
- **FAIL:** No recent knowledge events (logging broken)

### 6. Drift check
Call `drift_scan`:
- **PASS:** No new duplicate trees, no new never-retrieved solutions
- **WARN:** New never-retrieved solutions (may just be fresh)
- **FAIL:** Duplicate trees created (route_problem was skipped)

## Output

```
Knowledge Verification:
  Orphan check:         PASS (0 orphans)
  Tag coverage:         PASS (all tagged)
  Embedding presence:   PASS (95% have embeddings)
  Contradictions:       PASS (none introduced)
  Events logged:        PASS (3 recent events)
  Drift check:          PASS (no duplicates)

  Overall: PASS (6/6)
```

## Remediation

For each failure, include the specific fix:
- "FAIL: 3 orphans → call link_solutions for [id1], [id2], [id3]"
- "FAIL: No tags → update solutions with relevant tags"
- "FAIL: Duplicate trees → call link_trees with type 'supersedes'"
