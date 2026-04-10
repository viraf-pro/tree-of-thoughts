# v0.2.0 — Knowledge Graph, Web Research, Harness Engineering

This release transforms tot-mcp from a tree reasoning tool into a **compounding knowledge system**. Solutions cross-reference each other in a graph with confidence-scored links, web research can be ingested directly, and computational feedback sensors maintain knowledge quality automatically.

**39 MCP tools** (was 30). **108 tests** (was 56). **8 packages** (was 6). **16 CLI commands** (was 10).

## Highlights

### Compounding Knowledge Graph
Solutions are no longer isolated rows in a database. When you store a solution, it **automatically cross-references** similar existing solutions with confidence-scored links. The knowledge graph supports four relationship types (related, supersedes, contradicts, extends) and tracks whether each link was created manually or auto-generated.

Graph topology analysis identifies **god nodes** (most connected solutions), **communities** (clusters of related knowledge), and **bridge edges** (surprising connections across domains) — inspired by [Graphify](https://graphify.net/)'s approach to knowledge graph analysis.

### Web Research Ingestion
New `ingest_url` tool fetches web pages and stores their content as solutions. Secure by design: HTTP/HTTPS only, localhost/metadata endpoints blocked, 1MB size cap, redirect validation. HTML is stripped to clean text with title extraction.

```
tot-mcp ingest https://arxiv.org/abs/2305.10601 --tags paper,tot
```

### Harness Engineering Sensors
Three computational feedback sensors that output structured JSON — usable in CI pipelines, pre-commit hooks, or background monitoring:

- **`lint`** — Structural health-check with **remediation instructions** (specific tool calls to fix each issue)
- **`drift`** — Entropy detection: duplicate trees, abandoned trees with valuable content, never-retrieved solutions
- **`report`** — Knowledge base overview: top solutions, tag coverage, graph shape, suggested queries

### Progressive Disclosure
New `get_tree_context` tool provides tiered context for resuming trees:
- **Summary tier**: problem, status, best path, frontier nodes
- **Full tier**: adds pruned branches, all paths, cross-tree links

### Design Rationale Capture
`store_solution` now auto-generates a rationale from the tree exploration: *"Evaluated 3 branches. Selected: X (score 0.85). Considered: Y (score 0.62)."* The rationale is included in retrieval results so agents understand *why* a solution was chosen.

### Token-Budgeted Retrieval
`retrieve_context` now accepts `max_tokens` to cap response size. Solutions are truncated to fit within the budget (~4 chars/token estimate), reducing context window consumption.

### Obsidian Vault Export
Export the entire knowledge base as an Obsidian vault:
```
tot-mcp export --obsidian ./my-vault
```
Generates interlinked markdown files with YAML frontmatter, `[[wiki-links]]` between solutions, and a tag-grouped index.

## New Tools (+9)

| Tool | Category | Description |
|---|---|---|
| `ingest_url` | Knowledge store | Fetch web page and store as solution |
| `link_solutions` | Knowledge graph | Cross-reference solutions with confidence scores |
| `get_solution_links` | Knowledge graph | View solution connections |
| `knowledge_graph` | Graph analysis | Topology: god nodes, communities, bridges |
| `knowledge_report` | Graph analysis | Structured knowledge base overview |
| `lint_knowledge` | Quality | Health-check with remediation instructions |
| `drift_scan` | Quality | Entropy and drift detection |
| `knowledge_log` | Quality | Knowledge evolution timeline |
| `get_tree_context` | Deep research | Progressive-disclosure tree resumption |

## New CLI Commands (+6)

| Command | Description |
|---|---|
| `tot-mcp lint` | Knowledge store health-check (JSON) |
| `tot-mcp drift` | Entropy/drift scan (JSON) |
| `tot-mcp report` | Knowledge base overview (JSON) |
| `tot-mcp health` | Machine-readable health summary (JSON) |
| `tot-mcp ingest <url>` | Fetch URL and store as solution |
| `tot-mcp export --obsidian <dir>` | Export as Obsidian vault |

## Bug Fixes

- **Fixed panic** on short IDs in LinkSolutions (`sourceID[:8]` crash)
- **Fixed duplicate links** — added UNIQUE constraint on solution_links
- **Fixed silent error swallowing** in LogKnowledgeEvent, autoLinkRelated, rows.Scan
- **Fixed orphan query** for NULL/empty tree_id handling
- **Fixed nil slice serialization** — JSON `[]` instead of `null` for empty collections
- **Fixed test ordering dependencies** — all tests are now self-contained

## Database Changes

New tables (additive — existing databases migrate automatically):
- `solution_links` — Cross-references with confidence, source, UNIQUE constraint
- `knowledge_log` — Knowledge evolution events

New columns (additive migrations):
- `solutions.rationale` — Design rationale from tree exploration
- `solution_links.confidence` — Link confidence score (0.0-1.0)
- `solution_links.source` — Link origin ("manual" or "auto")

## Design Influences

- **[Karpathy's LLM Wiki](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)** — Persistent compounding knowledge, solution cross-references, lint operations
- **[Harness Engineering](https://martinfowler.com/articles/harness-engineering.html)** — Feedforward guides, computational sensors, remediation instructions, progressive disclosure
- **[Graphify](https://graphify.net/)** — Graph topology analysis, confidence labels, token budgets, Obsidian export

## Breaking Changes

None. All changes are additive. Existing databases migrate automatically on startup.

## Full Changelog

29 commits since v0.1.0. See `git log v0.1.0..v0.2.0` for details.
