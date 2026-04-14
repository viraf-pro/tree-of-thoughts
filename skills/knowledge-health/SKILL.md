---
name: knowledge-health
description: Run a comprehensive health check on the knowledge base. Finds quality issues, entropy, orphans, and suggests fixes. Use when the user says "check knowledge", "lint", "health check", "clean up knowledge", or periodically to maintain quality.
---

# Knowledge Health Check

Run all three knowledge quality sensors and present a unified report with actionable fixes.

## Steps

1. **Lint.** Call `lint_knowledge` to find structural issues:
   - Orphan solutions (no cross-references)
   - Missing embeddings
   - Stale entries
   - Potential contradictions
   
   Each issue comes with a specific remediation (exact tool call to fix it).

2. **Drift scan.** Call `drift_scan` to detect entropy:
   - Duplicate tree pairs (same problem explored twice)
   - Abandoned trees with valuable content (should be resumed or harvested)
   - Never-retrieved solutions (stored but never used)

3. **Report.** Call `knowledge_report` for the overview:
   - Top solutions (god nodes)
   - Tag coverage
   - Graph summary
   - Recent events

4. **Present findings.** Organize into three sections:
   - **Issues to fix now** — lint problems with remediations
   - **Entropy to address** — duplicates, abandoned value, unused solutions
   - **Knowledge overview** — stats, top solutions, tag coverage

5. **Offer to fix.** For each issue, offer to execute the remediation:
   - Link orphan solutions with `link_solutions`
   - Compact old solutions with `compact_analyze` + `compact_apply`
   - Merge duplicate trees by linking with `link_trees` type "supersedes"
