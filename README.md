# Tree of Thoughts MCP

A single-binary MCP server for tree-structured reasoning. Persistent storage, hybrid retrieval, compounding knowledge graph, experiment runner, live dashboard, and plugins for Claude Code and OpenAI Codex with 21 skills, 7 agents, and harness engineering hooks.

Built on the [Tree of Thoughts](https://arxiv.org/abs/2305.10601) framework (Yao et al., 2023).

## Table of Contents

- [How it works](#how-it-works)
- [Install](#install)
  - [Download binary](#download-binary)
  - [Build from source](#build-from-source)
- [Connect to an LLM client](#connect-to-an-llm-client)
  - [Claude Code plugin (recommended)](#claude-code-plugin-recommended)
  - [OpenAI Codex plugin](#openai-codex-plugin)
  - [Claude Code MCP only](#claude-code-mcp-only)
  - [Claude Desktop](#claude-desktop)
  - [ChatGPT Desktop](#chatgpt-desktop)
- [CLI mode](#cli-mode)
- [Configuration](#configuration)
- [Tools (39)](#tools-39)
- [Workflows](#workflows)
- [Live dashboard](#live-dashboard)
- [Architecture](#architecture)
- [Contributing](#contributing)

## How it works

Instead of linear chain-of-thought, the AI explores a **tree** of ideas:

1. **Branch out** — generate 3-5 candidate ideas at each step
2. **Evaluate** — score each as "sure", "maybe", or "impossible" (dead ends pruned)
3. **Go deep** — expand promising branches with sub-ideas
4. **Compare** — rank all explored paths and pick the best one

Everything gets saved in a **compounding knowledge graph** — solutions are cross-referenced, tagged with confidence scores, and analyzed for topology. Next time a similar problem comes up, past solutions are retrieved and built upon.

### Key capabilities

| Capability | What it does |
|---|---|
| **Tree reasoning** | BFS/DFS/Beam search with evaluation, backtracking, and frontier management |
| **Deep research** | Exhaustive multi-branch exploration before concluding |
| **Knowledge graph** | Auto-linked solutions with confidence scores, topology analysis (god nodes, communities, bridges) |
| **Hybrid retrieval** | Vector similarity (cosine) + FTS5 keyword search, merged with 20% dual-match boost |
| **Smart routing** | Prevents duplicate trees using embedding similarity + Jaccard overlap |
| **Web ingestion** | Fetch URLs and store as searchable solutions |
| **Experiment runner** | Modify code, run, parse metrics, keep or discard — guided by the tree |
| **Knowledge maintenance** | Lint, drift scan, compaction, and Obsidian vault export |
| **Agent workflows** | Multi-agent pipelines: research-and-validate, experiment-loop, knowledge-maintenance |
| **Live dashboard** | Real-time D3 tree visualization, solution store, experiment charts at `localhost:4545` |

## Install

### Download binary

Prebuilt binaries for all platforms from [GitHub Releases](https://github.com/viraf-pro/tree-of-thoughts/releases):

```bash
# macOS Apple Silicon
curl -L https://github.com/viraf-pro/tree-of-thoughts/releases/latest/download/tot-mcp-darwin-arm64.tar.gz | tar xz

# macOS Intel
curl -L https://github.com/viraf-pro/tree-of-thoughts/releases/latest/download/tot-mcp-darwin-amd64.tar.gz | tar xz

# Linux x86_64
curl -L https://github.com/viraf-pro/tree-of-thoughts/releases/latest/download/tot-mcp-linux-amd64.tar.gz | tar xz

# Linux ARM64
curl -L https://github.com/viraf-pro/tree-of-thoughts/releases/latest/download/tot-mcp-linux-arm64.tar.gz | tar xz

# Windows (PowerShell)
Invoke-WebRequest https://github.com/viraf-pro/tree-of-thoughts/releases/latest/download/tot-mcp-windows-amd64.exe -OutFile tot-mcp.exe
```

**Add to PATH:**
- macOS/Linux: `mv tot-mcp /usr/local/bin/`
- Windows: move `tot-mcp.exe` to a directory in your `%PATH%`, or add its location to `%PATH%`

### Build from source

```bash
git clone https://github.com/viraf-pro/tree-of-thoughts.git
cd tree-of-thoughts
go build -o tot-mcp .
```

Cross-compile all platforms: `make all`

## Connect to an LLM client

### Claude Code plugin (recommended)

The full plugin experience with skills, agents, hooks, and auto-updates:

```bash
# Add the marketplace
/plugin marketplace add viraf-pro/tree-of-thoughts

# Install the plugin
/plugin install tree-of-thoughts@tree-of-thoughts
```

The plugin auto-downloads the binary on first use (no restart needed) and includes:

- **39 tools** — tree reasoning, deep research, knowledge graph, experiment runner
- **21 skills** — `/tree-of-thoughts:create-tree`, `:deep-research`, `:decide`, `:run-experiment`, etc.
- **7 agents** — researcher, experimenter, librarian, critic, synthesizer, conductor, scout
- **7 hooks** — session briefing, duplicate prevention, research verification, knowledge lint
- **Auto-updates** — session briefing notifies when a new version is available
- **Dashboard** — live web UI at `http://127.0.0.1:4545` (shown in session briefing)

The binary version is pinned to the plugin version — updating the marketplace keeps skills, agents, hooks, and the MCP server in sync.

**Updating:**

```
/plugin marketplace update tree-of-thoughts
```

**Local development:**

```bash
claude --plugin-dir /path/to/tree-of-thoughts
```

### OpenAI Codex plugin

The same plugin experience is available for OpenAI Codex:

**Repo-scoped install:**

Add to `$REPO_ROOT/.agents/plugins/marketplace.json`:

```json
{
  "name": "tree-of-thoughts",
  "plugins": [
    {
      "name": "tree-of-thoughts",
      "source": {
        "source": "local",
        "path": "./plugins/tree-of-thoughts"
      },
      "policy": {
        "installation": "AVAILABLE",
        "authentication": "ON_INSTALL"
      },
      "category": "Productivity"
    }
  ]
}
```

Then clone the plugin:

```bash
git clone https://github.com/viraf-pro/tree-of-thoughts.git plugins/tree-of-thoughts
```

**Personal install:**

Create `~/.agents/plugins/marketplace.json` with the same JSON (change `path` to `~/.codex/plugins/tree-of-thoughts`), then clone the repo there.

The Codex plugin shares the same 21 skills, 39 MCP tools, and launch scripts as the Claude Code plugin. The binary auto-downloads on first use.

### Claude Code MCP only

If you only need the 39 tools without skills, agents, or hooks:

```bash
curl -L https://github.com/viraf-pro/tree-of-thoughts/releases/latest/download/tot-mcp-darwin-arm64.tar.gz | tar xz
claude mcp add tree-of-thoughts -- ./tot-mcp
```

Verify: `claude mcp list`

### Claude Desktop

Add to `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "tree-of-thoughts": {
      "command": "/absolute/path/to/tot-mcp"
    }
  }
}
```

Semantic search works out of the box via the bundled on-device model. No API keys needed. For faster embeddings, add `"env": {"OPENAI_API_KEY": "sk-..."}`.

### ChatGPT Desktop

**macOS:** `~/Library/Application Support/com.openai.chat/mcp.json`
**Windows:** `%APPDATA%\com.openai.chat\mcp.json`

```json
{
  "mcpServers": {
    "tree-of-thoughts": {
      "command": "/absolute/path/to/tot-mcp"
    }
  }
}
```

Restart ChatGPT after saving.

## CLI mode

The binary doubles as a CLI for scripting and CI pipelines:

```bash
./tot-mcp help                          # Show commands
./tot-mcp suggest                       # What should I work on next?
./tot-mcp list                          # List all trees
./tot-mcp show <tree_id>                # Show tree summary and best path
./tot-mcp route "problem text"          # Check if problem matches existing tree
./tot-mcp create "problem text"         # Create a new tree (default: beam)
./tot-mcp add <tree> <parent> <thought> # Add a thought to a tree node
./tot-mcp eval <tree> <node> <eval>     # Evaluate: sure, maybe, or impossible
./tot-mcp solve <tree> <node>           # Mark a node as the solution
./tot-mcp ready                         # Show active trees with frontier nodes
./tot-mcp audit [tree_id]               # View audit trail (last 20 entries)
./tot-mcp stats                         # Retrieval store statistics
./tot-mcp compact                       # Find solutions eligible for compaction
./tot-mcp lint                          # Knowledge store health-check (JSON)
./tot-mcp drift                         # Entropy/drift scan (JSON)
./tot-mcp report                        # Knowledge base overview (JSON)
./tot-mcp health                        # Machine-readable health summary (JSON)
./tot-mcp ingest <url> [--tags a,b]     # Fetch URL and store as solution
./tot-mcp export --obsidian <dir>       # Export as Obsidian vault
```

The `lint`, `drift`, `report`, and `health` commands output structured JSON for use as CI sensors.

## Configuration

All configuration is via environment variables.

| Variable | Purpose | Default |
|---|---|---|
| `TOT_DB_PATH` | SQLite database location | `~/.tot-mcp/tot.db` |
| `TOT_DASHBOARD_PORT` | Dashboard HTTP port | `4545` |
| `TOT_NO_DASHBOARD` | Disable dashboard | (not set) |

### Embedding providers

Semantic search works out of the box with a bundled on-device model. No API keys needed.

| Provider | Variable | Model | Notes |
|---|---|---|---|
| **Local (default)** | (none) | `all-MiniLM-L6-v2` | On-device, pure Go ONNX. Downloads ~22MB on first run. |
| OpenAI | `OPENAI_API_KEY` | `text-embedding-3-small` | Fastest, cloud. |
| Voyage AI | `VOYAGE_API_KEY` | `voyage-3-lite` | Cloud. |
| Ollama | `OLLAMA_BASE_URL` | `mxbai-embed-large` | Local, requires Ollama. |

Provider auto-selection: OpenAI > Voyage > Ollama > Local > FTS5-only fallback.
Override with `TOT_EMBED_PROVIDER` and `TOT_EMBED_MODEL`.

## Tools (39)

### Tree operations (10)

| Tool | Description |
|---|---|
| `create_tree` | Initialize a new reasoning tree (BFS/DFS/Beam) |
| `generate_thoughts` | Add candidate thoughts as children of a node |
| `evaluate_thought` | Score as "sure", "maybe", or "impossible" |
| `search_step` | Pop the next frontier node (destructive) |
| `backtrack` | Prune a node and all descendants |
| `mark_solution` | Flag a node as the terminal answer |
| `get_best_path` | Extract highest-scoring root-to-leaf path |
| `get_tree_summary` | Node counts, frontier size, max depth |
| `inspect_node` | View a node, its children, and root path |
| `list_trees` | List all trees (auto-pauses stale ones) |

### Deep research (3)

| Tool | Description |
|---|---|
| `get_frontier` | List expandable nodes by score (non-destructive) |
| `get_all_paths` | Rank all paths for comparison |
| `get_tree_context` | Progressive-disclosure context for resuming trees |

### Tree lifecycle (4)

| Tool | Description |
|---|---|
| `route_problem` | Check for matching existing trees before creating |
| `resume_tree` | Reactivate a paused/abandoned tree |
| `abandon_tree` | Mark a tree as abandoned |
| `suggest_next` | Recommend the most promising tree to work on |

### Knowledge store (4)

| Tool | Description |
|---|---|
| `retrieve_context` | Hybrid search past solutions (vector + FTS5) |
| `store_solution` | Save solution with auto-linking and design rationale |
| `retrieval_stats` | Solution count, embedding coverage, compaction stats |
| `ingest_url` | Fetch a URL and store as a searchable solution |

### Knowledge graph (6)

| Tool | Description |
|---|---|
| `link_solutions` | Cross-reference solutions (related/supersedes/contradicts/extends) |
| `get_solution_links` | View cross-references for a solution |
| `link_trees` | Create tree dependencies |
| `get_tree_links` | View cross-tree relationships |
| `knowledge_graph` | Topology analysis: god nodes, communities, bridges |
| `knowledge_report` | Structured overview of the knowledge base |

### Quality & maintenance (5)

| Tool | Description |
|---|---|
| `lint_knowledge` | Health-check with actionable remediations |
| `drift_scan` | Detect duplicates, abandoned trees, unused solutions |
| `knowledge_log` | Knowledge evolution timeline |
| `audit_log` | Tool call audit trail |
| `open_dashboard` | Get the dashboard URL |

### Compaction (3)

| Tool | Description |
|---|---|
| `compact_analyze` | Find solutions eligible for compression |
| `compact_apply` | Compress a solution (original archived, restorable) |
| `compact_restore` | Restore a compacted solution to full content |

### Experiment runner (4)

| Tool | Description |
|---|---|
| `configure_experiment` | Set target file, run command, metric regex, direction |
| `prepare_experiment` | Apply a code patch and git commit |
| `execute_experiment` | Run, parse metric, auto-evaluate, keep or rollback |
| `experiment_history` | Run count, success rate, best metric |

## Workflows

### Standard ToT loop

```
suggest_next → route_problem → create_tree → retrieve_context
→ generate_thoughts → evaluate_thought → search_step
→ [repeat] → mark_solution → get_best_path → store_solution
```

### Deep research

Expand ALL promising branches before concluding:

```
create_tree → generate_thoughts (4-5 approaches)
→ evaluate_thought for ALL → get_frontier (non-destructive)
→ expand each promising branch → get_all_paths (ranked comparison)
→ mark_solution → store_solution
```

### Experiment loop

```
create_tree → configure_experiment
→ generate_thoughts (hypotheses) → search_step
→ prepare_experiment (patch + commit) → execute_experiment (run + evaluate)
→ [repeat] → store_solution
```

### Agent workflows

Multi-agent pipelines with explicit handoff ([Harness Engineering](https://martinfowler.com/articles/harness-engineering.html)):

| Workflow | Pipeline |
|---|---|
| `research-and-validate` | scout > researcher > critic > [revision loop] > librarian |
| `experiment-loop` | scout > researcher > experimenter > researcher > librarian |
| `knowledge-maintenance` | librarian > synthesizer > critic > librarian |

### Tree lifecycle

```
active  → solved       mark_solution() called
active  → paused       auto-pause (30 min idle)
active  → abandoned    abandon_tree()
paused  → active       resume_tree()
abandoned → active     resume_tree()
```

## Live dashboard

Auto-starts at `http://127.0.0.1:4545`. Disable with `TOT_NO_DASHBOARD=1`.

- **Radial tree** — D3-powered, color-coded nodes (blue=sure, amber=maybe, red=impossible, green=solution). Click any node to see the full reasoning path.
- **Paths** — All explored paths ranked by score with expandable analysis.
- **Solutions** — Full text, score badges, and tags.
- **Experiments** — Run count, success rate, metric progression chart.
- **Real-time** — SSE updates with 10s polling fallback.

## Architecture

```
tot-mcp/
  main.go              39 tools, 7 resources, dashboard + event bus
  cli.go               21 CLI commands
  internal/
    db/                SQLite (WAL, pure Go)
    tree/              Tree CRUD, search, routing, cross-tree links
    retrieval/         Hybrid search, knowledge graph, lint, compaction
    events/            Event bus, MCP notification bridge
    resources/         7 MCP resource templates
    experiment/        Git-based experiment runner
    dashboard/         HTTP API + embedded SPA + SSE
    embeddings/        Pluggable: local ONNX, OpenAI, Voyage, Ollama
    web/               Secure URL fetching
  skills/              21 Claude Code skills
  agents/              7 specialized agents
  hooks/               Harness engineering hooks
  scripts/             Hook scripts (install, briefing, verification)
  .claude-plugin/      Claude Code plugin manifest + marketplace
  .codex-plugin/       OpenAI Codex plugin manifest
```

### Dependencies

| Dependency | Purpose |
|---|---|
| `modernc.org/sqlite` | Pure Go SQLite (zero CGO) |
| `github.com/mark3labs/mcp-go` | MCP SDK (JSON-RPC 2.0) |
| `github.com/google/uuid` | Node and tree IDs |
| `github.com/knights-analytics/hugot` | On-device ONNX embeddings |

Four Go dependencies. No CGO. Fully static binary.

## Contributing

```bash
go build -o tot-mcp .       # Build
go test ./...                # 273 tests across 11 packages
go vet ./...                 # Lint
make all                     # Cross-compile
```

Releases are automated via GoReleaser — push a `v*` tag to build and publish.

## Design influences

- **[Karpathy's LLM Wiki](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)** — Compounding knowledge artifacts
- **[Harness Engineering](https://martinfowler.com/articles/harness-engineering.html)** — Feedforward guides + computational feedback sensors
- **[Graphify](https://graphify.net/)** — Graph topology analysis, confidence-scored relationships

## License

MIT. See [LICENSE](LICENSE).
