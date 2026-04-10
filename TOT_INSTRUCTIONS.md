# Tree of Thoughts — Agent Instructions

Add this to your project's AGENTS.md or CLAUDE.md to help coding agents use tot-mcp effectively.

## Quick Start (read this first)

```
suggest_next → route_problem → create_tree or resume_tree
→ generate_thoughts → evaluate_thought → search_step
→ mark_solution → store_solution
```

1. Call `suggest_next` — it tells you what to work on.
2. Call `route_problem` before creating a new tree — avoids duplicates.
3. Explore: `generate_thoughts` → `evaluate_thought` → `search_step` loop.
4. When solved: `mark_solution` → `get_best_path` → `store_solution`.
5. Resuming a tree? Call `get_tree_context` for full decision history.

## Detailed Workflow

### On session start

1. Call `suggest_next` to check if there's an existing tree to continue.
2. If it returns `action: "continue"` or `action: "resume"`, call `get_tree_context` for full context, then `resume_tree`.
3. If it returns `action: "create"`, proceed to create a new tree.

### Before creating a tree

1. Call `route_problem("your problem statement")` first.
2. If it returns `action: "continue"`, use the existing tree instead of creating a new one.
3. Only call `create_tree` if route_problem returns `action: "create"`.

### During exploration

1. Use `generate_thoughts` to create 3-4 candidate approaches per node.
2. Use `evaluate_thought` to score each: "sure" (promising), "maybe" (uncertain), "impossible" (dead end).
3. Use `search_step` to get the next node to expand (the server picks the best one based on the search strategy).
4. Use `backtrack` when a branch is clearly wrong. This prunes the subtree.
5. Use `retrieve_context` early to check if similar problems have been solved before.

### When solved

1. Call `mark_solution` on the winning node.
2. Call `get_best_path` to extract the full reasoning chain.
3. Call `store_solution` with descriptive tags to save it for future retrieval.
4. If this analysis informs another tree, call `link_trees` to create the cross-reference.

### Knowledge maintenance (periodic)

1. Call `lint_knowledge` to health-check the knowledge store. It returns **remediations** — specific tool calls you can execute to fix each issue.
2. Call `drift_scan` to detect entropy: duplicate trees, abandoned trees with valuable content, never-retrieved solutions.
3. Review similar pairs: compare the solutions and use `link_solutions` to record the relationship.
4. For unlinked solutions: use `link_solutions` to connect related entries (types: `related`, `supersedes`, `contradicts`, `extends`).
5. For stale solutions: consider `compact_analyze` or re-evaluation.
6. Call `knowledge_log` to review how the knowledge base evolved over time.

Note: `store_solution` automatically cross-references new solutions with similar existing ones. Manual linking is for connections the auto-linker misses.

### Compaction (periodic)

1. Call `compact_analyze` to find old solutions (30+ days) eligible for compression.
2. For each candidate, generate a 1-2 sentence summary preserving the key insight.
3. Call `compact_apply` with the summary. Original is archived and restorable.

## When to use Tree of Thoughts

Use ToT for problems where:
- There are 3+ plausible approaches and you're unsure which is best
- The wrong choice wastes significant time (market entry, architecture decisions, debugging)
- Evidence from one branch informs evaluation of another
- Past solutions from similar problems would accelerate the current one

Do NOT use ToT for:
- Simple tasks with obvious solutions
- Linear workflows with no branching
- Quick questions answerable from knowledge alone

## Search strategies

- **Beam search**: Best for most problems. Keeps the top-K scoring nodes in focus. Use when you want breadth + scoring.
- **BFS**: Use when the solution depth is known (e.g., "compare 4 options at depth 1, pick one").
- **DFS**: Use when going deep fast matters (e.g., debugging — drill into one hypothesis quickly).

## Tool reference (quick)

### Core reasoning
| Tool | When to call |
|---|---|
| `suggest_next` | Session start. "What should I work on?" |
| `route_problem` | Before `create_tree`. Check for existing matching tree. |
| `create_tree` | New problem, no existing tree matches. |
| `generate_thoughts` | Expand a node with candidate approaches. |
| `evaluate_thought` | Score a candidate as sure/maybe/impossible. |
| `search_step` | Get the next node to expand. |
| `backtrack` | Prune a dead-end branch. |
| `mark_solution` | Flag the winning node. |
| `get_best_path` | Extract the full reasoning chain. |
| `get_tree_context` | Get full context for resuming a tree (summary or full detail). |

### Knowledge store
| Tool | When to call |
|---|---|
| `retrieve_context` | Search past solutions for similar problems. |
| `store_solution` | Save a solution for future retrieval. |
| `link_solutions` | Cross-reference two solutions (related, supersedes, contradicts, extends). |
| `get_solution_links` | View what a solution is connected to in the knowledge graph. |
| `link_trees` | Connect two trees (depends_on, informs, etc). |

### Quality & maintenance
| Tool | When to call |
|---|---|
| `lint_knowledge` | Health-check. Returns remediations with specific tool calls to fix issues. |
| `drift_scan` | Detect entropy: duplicate trees, abandoned valuable trees, unused solutions. |
| `knowledge_log` | View the timeline of knowledge evolution. |
| `compact_analyze` | Find old solutions eligible for compression. |
| `compact_apply` | Compress a solution, archive the original. |
| `audit_log` | View the decision trail for debugging. |
| `open_dashboard` | Get the URL for the live visual dashboard. |

## CLI commands

Run `tot-mcp <command>` for lightweight CLI access (no MCP overhead):

```
tot-mcp lint      # Knowledge store health-check (JSON)
tot-mcp drift     # Entropy/drift scan (JSON)
tot-mcp health    # Machine-readable health summary (JSON)
tot-mcp suggest   # What should I work on next?
tot-mcp list      # List all trees
tot-mcp stats     # Retrieval store statistics
```
