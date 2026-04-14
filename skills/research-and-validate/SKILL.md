---
name: research-and-validate
description: Full research pipeline with adversarial validation. Dispatches scout → researcher → critic → revision loop → librarian. Use when the user needs a thoroughly researched AND validated answer, says "research and verify", "I need a solid answer", or when the stakes are high enough to warrant adversarial review.
---

# Research and Validate Workflow

A multi-agent pipeline that researches a problem deeply, then validates the conclusion through adversarial review before storing it.

## Pipeline

```
Scout (route) → Researcher (explore) → Critic (challenge)
    ↓                                       ↓
    ↓                                  Issues found?
    ↓                                   ↙        ↘
    ↓                                 Yes         No
    ↓                                  ↓           ↓
    ↓                            Researcher      Librarian
    ↓                            (revise)        (store)
    ↓                                ↓
    ↓                            Critic (re-verify)
    ↓                                ↓
    ↓                          Max 2 loops, then human
```

## Execution

### Stage 1: Scout (routing)

Dispatch the `tree-of-thoughts:scout` agent:

```
## Context Baton
- Workflow: research-and-validate
- Stage: 1 of 5
- Your task: Find or create a tree for this problem. Load any prior context.
- Success criteria: Return tree ID, status, and any existing best path
```

### Stage 2: Researcher (exploration)

Dispatch the `tree-of-thoughts:researcher` agent with the scout's findings:

```
## Context Baton
- Workflow: research-and-validate
- Stage: 2 of 5
- Tree ID: [from scout]
- Previous agent: scout
- Their findings: [tree status, existing paths, prior solutions]
- Your task: Deep multi-path research. Generate diverse candidates, evaluate honestly, explore all promising branches.
- Success criteria: Solution marked, all frontier nodes with score > 0.5 explored, get_all_paths shows comparison
```

### Stage 3: Critic (adversarial review)

Dispatch the `tree-of-thoughts:critic` agent:

```
## Context Baton
- Workflow: research-and-validate
- Stage: 3 of 5
- Tree ID: [same]
- Previous agent: researcher
- Their findings: [best path, solution node, scores, explored branches]
- Your task: Challenge this conclusion. Check pruned branches for premature dismissal. Search for counterevidence. Rate robustness.
- Success criteria: Verdict (ROBUST/WEAK/NEEDS WORK) with specific issues or approval
```

### Stage 4: Revision loop (if needed)

If critic verdict is WEAK or NEEDS WORK, dispatch researcher again:

```
## Context Baton
- Workflow: research-and-validate
- Stage: 4 of 5 (revision round [N])
- Tree ID: [same]
- Previous agent: critic
- Their findings: [specific weaknesses, blind spots, unexplored angles]
- Your task: Address these specific issues. Expand the branches the critic identified. DO NOT re-explore what was already validated.
- Success criteria: Each critic issue addressed with new thoughts/evaluations
```

Then dispatch critic again to re-verify. Maximum 2 revision rounds.

### Stage 5: Librarian (storage)

After critic approves (or after 2 revision rounds):

```
## Context Baton
- Workflow: research-and-validate
- Stage: 5 of 5
- Tree ID: [same]
- Previous agent: critic
- Their findings: [final verdict, any remaining caveats]
- Your task: Store the validated solution. Tag appropriately. Cross-reference with existing knowledge. Include the critic's verdict in the solution rationale.
- Success criteria: Solution stored with tags, cross-references created, knowledge event logged
```

## Verification

After the pipeline completes, run the `verify-research` sensor to confirm:
- Frontier is empty or all remaining nodes are low-score
- Solution is stored with tags
- Critic verdict is recorded
- No orphan solutions created
