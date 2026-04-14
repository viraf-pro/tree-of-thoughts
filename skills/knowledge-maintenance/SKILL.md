---
name: knowledge-maintenance
description: Knowledge quality pipeline with automated remediation. Dispatches librarian (detect) → synthesizer (consolidate) → critic (validate) → librarian (apply fixes). Use periodically, when knowledge base has grown, or when the user says "maintain knowledge", "consolidate", "clean up everything".
---

# Knowledge Maintenance Workflow

A multi-agent pipeline that detects quality issues, consolidates knowledge, validates the consolidation, and applies fixes.

## Pipeline

```
Librarian (detect issues) → Synthesizer (consolidate clusters)
         ↓                           ↓
    Issues found               Synthesis created
         ↓                           ↓
         └──────────→ Critic (validate both) ←──┘
                           ↓
                      Issues with fixes?
                       ↙        ↘
                     Yes         No
                      ↓           ↓
                 Librarian      Done
                 (apply fixes)
```

## Execution

### Stage 1: Librarian (detect)

Dispatch `tree-of-thoughts:librarian`:

```
## Context Baton
- Workflow: knowledge-maintenance
- Stage: 1 of 4
- Your task: Run comprehensive health check. Call lint_knowledge, drift_scan, and knowledge_report. Categorize findings into: (1) structural issues with remediations, (2) entropy/drift, (3) consolidation opportunities (clusters with 3+ unlinked solutions on the same topic).
- Success criteria: Categorized list of issues with specific remediations. List of solution clusters ripe for synthesis.
```

### Stage 2: Synthesizer (consolidate)

Dispatch `tree-of-thoughts:synthesizer` with the librarian's cluster list:

```
## Context Baton
- Workflow: knowledge-maintenance
- Stage: 2 of 4
- Previous agent: librarian
- Their findings: [categorized issues, solution clusters for consolidation]
- Your task: For each cluster of 3+ related solutions, create a synthesis that extracts the higher-order insight. Look for cross-domain patterns. Create link_solutions connections. Tag synthesis outputs with "synthesis".
- Success criteria: Each cluster has a synthesis solution. Cross-domain bridges identified. All new solutions tagged and linked.
```

### Stage 3: Critic (validate)

Dispatch `tree-of-thoughts:critic` to validate both the issues and syntheses:

```
## Context Baton
- Workflow: knowledge-maintenance
- Stage: 3 of 4
- Previous agents: librarian (issues), synthesizer (consolidations)
- Their findings: [issues list with remediations, new synthesis solutions with links]
- Your task: Validate the proposed changes. For each remediation: is it safe? For each synthesis: does it add genuine insight or just summarize? For each proposed link: is the relationship type correct? Flag any changes that might lose information.
- Success criteria: Approved list of safe changes. Flagged list of risky changes needing human review.
```

### Stage 4: Librarian (apply)

Dispatch `tree-of-thoughts:librarian` with the approved changes:

```
## Context Baton
- Workflow: knowledge-maintenance
- Stage: 4 of 4
- Previous agent: critic
- Their findings: [approved changes list, flagged changes for human review]
- Your task: Apply all approved changes. For each approved remediation, execute the tool call. Skip flagged changes and report them to the user. Run lint_knowledge again to verify improvements.
- Success criteria: All approved changes applied. Lint re-run shows fewer issues. Flagged items reported to user.
```

## When to Run

- After ingesting 10+ new items
- Weekly as a maintenance routine
- When `drift_scan` reports high entropy
- Before major decisions that rely on the knowledge base
