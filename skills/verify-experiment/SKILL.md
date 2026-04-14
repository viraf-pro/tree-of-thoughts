---
name: verify-experiment
description: Computational feedback sensor for experiment quality. Checks that experiments were properly configured, metrics parsed, baselines updated, and git state is clean. Runs after experimenter agent completes.
---

# Verify Experiment (Feedback Sensor)

Fast check that experiments ran correctly and state is consistent.

## Checks

### 1. Experiment count
Call `experiment_history` for the tree.
- **PASS:** At least 1 experiment completed
- **FAIL:** No experiments recorded (experimenter may have failed silently)

### 2. Metric parsed
From experiment history, check each result:
- **PASS:** All non-crashed experiments have a metric value
- **WARN:** Some experiments have null metric (regex may be wrong)
- **FAIL:** No experiments have metrics (metric_regex likely misconfigured)

### 3. Baseline tracking
From experiment history, check if improved experiments updated the baseline:
- **PASS:** Baseline exists and reflects the best result
- **WARN:** No improvements found (all regressed/crashed)
- **FAIL:** Improvement found but baseline not updated

### 4. Tree node evaluation consistency
For each experiment result, check the corresponding tree node:
- **PASS:** Improved experiments → node scored "sure"; crashed → "impossible"
- **WARN:** Mismatch between experiment status and node evaluation
- **FAIL:** Experiment ran but node was never evaluated

### 5. Git state
Execute `git status` in the work_dir (via Bash tool):
- **PASS:** Working tree is clean (no uncommitted changes)
- **WARN:** Uncommitted changes present (rollback may have left debris)
- **FAIL:** On wrong branch (should be on experiment branch or main)

### 6. Success rate
From experiment history stats:
- **PASS:** Success rate > 0% (at least one improvement found)
- **INFO:** Success rate = 0% after 3+ experiments (approach may be wrong)
- **WARN:** Success rate = 100% (suspiciously good — check metric regex)

## Output

```
Experiment Verification: [tree_id]
  Experiment count:     PASS (5 experiments)
  Metric parsed:        PASS (5/5 have metrics)
  Baseline tracking:    PASS (baseline: 0.42 → 0.38)
  Node eval consistency: PASS (all nodes match)
  Git state:            PASS (clean, on main)
  Success rate:         PASS (40% — 2/5 improved)

  Overall: PASS (6/6)
```
