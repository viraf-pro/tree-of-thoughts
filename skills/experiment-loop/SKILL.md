---
name: experiment-loop
description: Hypothesis-driven experiment pipeline. Dispatches scout → researcher (hypothesis) → experimenter (test) → researcher (interpret) → librarian (store). Use when the user wants to systematically test code hypotheses, says "run experiments", "optimize", "find the best approach by testing", or needs empirical validation of ideas.
---

# Experiment Loop Workflow

A multi-agent pipeline that generates hypotheses from tree reasoning, tests them empirically, interprets results, and stores findings.

## Pipeline

```
Scout (route) → Researcher (hypothesize) → Experimenter (test)
                      ↑                          ↓
                      ↑                     Results back
                      ↑                          ↓
                 Researcher (interpret) ← ───────┘
                      ↓
                 More hypotheses?
                  ↙        ↘
                Yes         No
                 ↓           ↓
            Experimenter   Librarian (store)
            (next test)
```

## Execution

### Stage 1: Scout (routing + config check)

Dispatch `tree-of-thoughts:scout`:

```
## Context Baton
- Workflow: experiment-loop
- Stage: 1 of 5
- Your task: Find the tree for this problem. Check if experiment_history exists (is the experiment runner configured?). Load baseline metric if available.
- Success criteria: Tree ID, experiment config status, baseline metric
```

### Stage 2: Researcher (generate hypotheses)

Dispatch `tree-of-thoughts:researcher`:

```
## Context Baton
- Workflow: experiment-loop
- Stage: 2 of 5
- Tree ID: [from scout]
- Previous agent: scout
- Their findings: [tree state, experiment config, baseline metric]
- Your task: Generate 3-5 hypotheses for improving the metric. Each hypothesis should be a specific, testable code change. Evaluate each for likelihood of improvement. Rank by expected impact.
- Success criteria: Hypotheses as tree nodes, ranked by score, each with a clear code change description
```

### Stage 3: Experimenter (test top hypothesis)

Dispatch `tree-of-thoughts:experimenter`:

```
## Context Baton
- Workflow: experiment-loop
- Stage: 3 of 5
- Tree ID: [same]
- Previous agent: researcher
- Their findings: [ranked hypotheses with scores and code change descriptions]
- Your task: Test the top-ranked hypothesis. Read the target file, apply the change, run the experiment. Report the metric result.
- Success criteria: Experiment result with status (improved/regressed/crashed), metric delta, kept/discarded
```

### Stage 4: Researcher (interpret results)

Dispatch `tree-of-thoughts:researcher` again:

```
## Context Baton
- Workflow: experiment-loop
- Stage: 4 of 5
- Tree ID: [same]
- Previous agent: experimenter
- Their findings: [experiment result: status, metric, delta, kept/discarded]
- Your task: Interpret the result. If improved, why did it work? If regressed, why? Update evaluations on the tree. Decide: should we test the next hypothesis or are we done?
- Success criteria: Tree nodes updated with experiment-informed evaluations. Clear recommendation: continue testing or conclude.
```

If more hypotheses should be tested, loop back to Stage 3 with the next hypothesis.

### Stage 5: Librarian (store findings)

After experiments are done:

```
## Context Baton
- Workflow: experiment-loop
- Stage: 5 of 5
- Tree ID: [same]
- Previous agent: researcher
- Their findings: [all experiment results, best approach, metric improvements]
- Your task: Store the findings. Include experiment metrics in the solution. Tag with experiment-related tags. Cross-reference with any similar optimization work.
- Success criteria: Solution stored with experiment results, tags include metric direction and domain
```

## Guidelines

- Maximum 5 experiment iterations per workflow run
- If 3 consecutive experiments regress, the researcher should reconsider the approach entirely
- Always store findings even if no improvement was found — "X doesn't work" is valuable knowledge
