# tot-mcp

A single-binary MCP server for tree-structured reasoning with persistent storage, hybrid retrieval, an autonomous experiment runner, and a live web dashboard.

Built on the [Tree of Thoughts](https://arxiv.org/abs/2305.10601) framework (Yao et al., 2023). The server gives LLMs structured exploration over multiple reasoning paths with evaluation, backtracking, and search algorithms, instead of linear chain-of-thought.

## What it does

**Tree reasoning.** The LLM generates multiple candidate thoughts at each step, evaluates them (sure/maybe/impossible), and uses search algorithms (BFS, DFS, Beam) to decide which branch to explore next. Dead ends get pruned. The best path gets extracted.

**Deep research.** Unlike greedy beam search that follows a single path, deep research explores all viable branches before concluding. The `get_frontier` tool shows all expandable nodes without consuming them, and `get_all_paths` ranks every explored branch for comparison. This prevents premature conclusions and surfaces non-obvious solutions.

**Persistent storage.** All trees, nodes, solutions, and experiment results live in a SQLite database. Survives restarts. A single `.db` file holds everything.

**Hybrid retrieval.** Past solutions are stored with embeddings. When a new problem arrives, the server searches by vector similarity (cosine distance) and keyword matching (FTS5), then merges the results. Solutions that match on both get boosted. Knowledge compounds across sessions.

**Smart routing.** Before creating a new tree, `route_problem` checks existing trees using embedding cosine similarity (primary) with Jaccard keyword overlap (fallback). When both signals agree, a hybrid boost is applied. This prevents duplicate trees across sessions even when the problem is rephrased.

**Experiment runner.** An optional autoresearch-style loop: modify code, run training/evaluation, parse the metric, keep or discard, repeat. The ToT tree guides which experiments to try. Inspired by [karpathy/autoresearch](https://github.com/karpathy/autoresearch).

**Live dashboard.** A web UI at `localhost:4545` renders an interactive D3 radial tree with click-to-explore path analysis, experiment history, metric charts, and a full-text solution store. Auto-refreshes every 10 seconds.

## Quick start

```bash
# Clone
git clone https://github.com/your-org/tot-mcp.git
cd tot-mcp

# Build
go mod tidy
go build -o tot-mcp .

# Run
./tot-mcp
```

The binary starts two things: an MCP server on stdio (for Claude Desktop / Claude Code) and an HTTP dashboard on port 4545.

## Install from releases

```bash
# Linux x86_64
curl -L https://github.com/your-org/tot-mcp/releases/latest/download/tot-mcp-linux-amd64 -o tot-mcp
chmod +x tot-mcp

# macOS Apple Silicon
curl -L https://github.com/your-org/tot-mcp/releases/latest/download/tot-mcp-darwin-arm64 -o tot-mcp
chmod +x tot-mcp

# Windows
curl -L https://github.com/your-org/tot-mcp/releases/latest/download/tot-mcp-windows-amd64.exe -o tot-mcp.exe
```

## Cross-compile all platforms

```bash
make all
# Outputs:
#   dist/tot-mcp-linux-amd64
#   dist/tot-mcp-linux-arm64
#   dist/tot-mcp-darwin-amd64
#   dist/tot-mcp-darwin-arm64
#   dist/tot-mcp-windows-amd64.exe
```

## CLI mode

The binary doubles as a lightweight CLI for scripting and quick queries (~1-2k tokens vs 10-50k for the full MCP tool schema).

```bash
./tot-mcp help                     # Show commands
./tot-mcp suggest                  # What should I work on next?
./tot-mcp list                     # List all trees
./tot-mcp show <tree_id>           # Show tree summary and best path
./tot-mcp route "problem text"     # Check if problem matches existing tree
./tot-mcp create "problem text"    # Create a new tree (default: beam)
./tot-mcp ready                    # Show active trees with frontier nodes
./tot-mcp audit [tree_id]          # View audit trail (last 20 entries)
./tot-mcp stats                    # Retrieval store statistics
./tot-mcp compact                  # Find solutions eligible for compaction
```

## Connect to Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "tree-of-thoughts": {
      "command": "/absolute/path/to/tot-mcp"
    }
  }
}
```

That's it. Semantic search works out of the box via the bundled on-device model. No API keys needed.

Optionally, for faster embeddings via OpenAI:

```json
{
  "mcpServers": {
    "tree-of-thoughts": {
      "command": "/absolute/path/to/tot-mcp",
      "env": {
        "OPENAI_API_KEY": "sk-..."
      }
    }
  }
}
```

## Connect to Claude Code

```bash
claude mcp add tree-of-thoughts /absolute/path/to/tot-mcp
```

## Connect to ChatGPT Desktop

ChatGPT desktop (macOS and Windows) supports MCP servers via its settings file.

**macOS:** Edit `~/Library/Application Support/com.openai.chat/mcp.json`

**Windows:** Edit `%APPDATA%\com.openai.chat\mcp.json`

Create the file if it doesn't exist, then add:

```json
{
  "mcpServers": {
    "tree-of-thoughts": {
      "command": "/absolute/path/to/tot-mcp"
    }
  }
}
```

With an embedding API key:

```json
{
  "mcpServers": {
    "tree-of-thoughts": {
      "command": "/absolute/path/to/tot-mcp",
      "env": {
        "OPENAI_API_KEY": "sk-..."
      }
    }
  }
}
```

Restart ChatGPT after saving. The tools will appear in the toolbox icon at the bottom of the chat input.

> **Note:** ChatGPT currently supports tools only — prompts and resources are not yet supported. All 30 tot-mcp tools are tool-type, so everything works.

## Configuration

All configuration is via environment variables.

### Core

| Variable | Purpose | Default |
|---|---|---|
| `TOT_DB_PATH` | SQLite database file location | `~/.tot-mcp/tot.db` |
| `TOT_DASHBOARD_PORT` | Dashboard HTTP port | `4545` |
| `TOT_NO_DASHBOARD` | Set to any value to disable dashboard | (not set) |

### Embedding providers

Semantic search is enabled by default using an on-device embedding model. No API keys needed.

| Provider | Variable | Default model | Dimensions | Notes |
|---|---|---|---|---|
| **Local (default)** | (none needed) | `all-MiniLM-L6-v2` | 384 | On-device, pure Go, zero config. Downloads model (~22MB) on first run. |
| OpenAI | `OPENAI_API_KEY` | `text-embedding-3-small` | 1536 | Fastest. Requires API key. |
| Voyage AI | `VOYAGE_API_KEY` | `voyage-3-lite` | 512 | Requires API key. |
| Ollama | `OLLAMA_BASE_URL` | `mxbai-embed-large` | 1024 | Local, requires Ollama running. |

**How provider selection works:**
1. If `OPENAI_API_KEY` is set, uses OpenAI (fastest, cloud).
2. Else if `VOYAGE_API_KEY` is set, uses Voyage.
3. Else if `OLLAMA_BASE_URL` is set, uses Ollama.
4. Else uses the **local on-device provider** automatically.
5. If local model fails to load, falls back to FTS5 keyword search only.

Force a specific provider with `TOT_EMBED_PROVIDER` (local, openai, voyage, or ollama). Override the model with `TOT_EMBED_MODEL`. Change the model cache directory with `TOT_MODEL_CACHE` (default: `~/.tot-mcp/models/`).

**On first run**, the local provider downloads `sentence-transformers/all-MiniLM-L6-v2` from HuggingFace (~22MB ONNX model). Subsequent starts load from cache in under a second. For air-gapped environments, pre-download the model files into `~/.tot-mcp/models/` before running.

The local provider uses [Hugot](https://github.com/knights-analytics/hugot) with a pure Go ONNX backend. Zero CGO. The single-binary promise is preserved.

## Tools (30)

### Tree operations (10)

| Tool | Description |
|---|---|
| `create_tree` | Initialize a new reasoning tree. Params: problem, search_strategy (bfs/dfs/beam), max_depth, branching_factor. |
| `generate_thoughts` | Add candidate thoughts as children of a parent node. Accepts an array of thought objects. |
| `evaluate_thought` | Score a node as "sure", "maybe", or "impossible". Optional custom 0-1 score. Impossible nodes are pruned from the search frontier. |
| `search_step` | Pop the next node from the frontier based on the tree's search strategy. BFS = shallowest first. DFS = deepest first. Beam = highest score first. **Destructive** — removes the node from the frontier. |
| `backtrack` | Prune a node and all descendants (recursive CTE). Returns the parent node. |
| `mark_solution` | Flag a node as the terminal answer. Removes it from the frontier. |
| `get_best_path` | Extract the highest-scoring complete path from root to a terminal/leaf node. |
| `get_tree_summary` | Stats: total nodes, terminal count, pruned count, frontier size, max depth reached. |
| `inspect_node` | View a node, its children, and the full path from root. |
| `list_trees` | List all reasoning trees in the database. Auto-pauses stale trees. |

### Deep research (2)

| Tool | Description |
|---|---|
| `get_frontier` | List all expandable frontier nodes ranked by score **without removing them**. Unlike `search_step`, this is non-destructive. Use to see all options before deciding which branches to expand. Essential for deep research. |
| `get_all_paths` | Return all paths to leaf and terminal nodes, ranked by average score. Use to compare all explored branches before marking a solution. |

### Tree lifecycle (4)

| Tool | Description |
|---|---|
| `route_problem` | **Call before create_tree.** Checks if the new problem matches an existing active/paused tree using embedding cosine similarity (primary) + Jaccard keyword overlap (fallback). Hybrid boost when both signals agree. Returns `action: "continue"` with the tree ID, or `action: "create"`. |
| `resume_tree` | Reactivate a paused or abandoned tree. |
| `abandon_tree` | Mark a tree as abandoned. Tree stays in the database but is excluded from routing. |
| `suggest_next` | Zero-arg "what should I work on next?" Returns the most promising active or paused tree. |

### Retrieval (3)

| Tool | Description |
|---|---|
| `retrieve_context` | Hybrid search past solutions. Vector similarity + FTS5 keyword matching. Results merged, hybrid matches boosted 20%. |
| `store_solution` | Save a completed solution with tags. Generates an embedding for future semantic search. Links back to the tree and best path. |
| `retrieval_stats` | Total solutions, embedding coverage, compaction stats. |

### Compaction (3)

| Tool | Description |
|---|---|
| `compact_analyze` | Find solutions older than N days eligible for compression. Returns full content for summary generation. Default: 30 days. |
| `compact_apply` | Replace a solution's detailed thoughts with a compressed summary. Original archived and restorable. Embedding stays intact. |
| `compact_restore` | Restore a compacted solution to its original full content from the archive. |

### Knowledge graph (3)

| Tool | Description |
|---|---|
| `link_trees` | Create a dependency between two trees (depends_on, informs, supersedes, related). |
| `get_tree_links` | View all cross-tree dependencies and relationships. |
| `audit_log` | View the audit trail of tool calls for debugging and decision tracing. |

### Experiment runner (4)

| Tool | Description |
|---|---|
| `configure_experiment` | Set target file, run command, metric regex, direction, timeout, work dir, git branch prefix. |
| `prepare_experiment` | Apply a code patch (full file replacement) and git commit. Returns commit hash and previous hash for rollback. |
| `execute_experiment` | Run the command, parse the metric, auto-evaluate the thought node, keep or git-reset. |
| `experiment_history` | View experiment stats: total runs, success rate, best metric, crash count. |

### Dashboard (1)

| Tool | Description |
|---|---|
| `open_dashboard` | Returns the dashboard URL. Optionally links directly to a specific tree. |

## Reasoning workflows

### Standard ToT loop

```
0. route_problem("Solve problem X")              → "create" (no match)
1. create_tree("Solve problem X", strategy="beam", branching_factor=3)
2. retrieve_context("problem X keywords")         # check past solutions
3. generate_thoughts(root_id, [idea_1, idea_2, idea_3])
4. evaluate_thought(node_1, "sure", 0.8)
   evaluate_thought(node_2, "maybe", 0.5)
   evaluate_thought(node_3, "impossible")          # pruned
5. search_step()                                    → node_1
6. generate_thoughts(node_1, [refinement_a, refinement_b])
7. evaluate_thought(...) → search_step() → ... repeat ...
8. mark_solution(winning_node)
9. get_best_path()                                  # extract full chain
10. store_solution(tree_id, "solution text", tags)  # save for retrieval
```

### Deep research workflow

For complex problems with multiple viable approaches, expand ALL promising branches before concluding:

```
1. create_tree("Complex problem", strategy="beam", branching_factor=5)
2. generate_thoughts(root, [approach_A, approach_B, approach_C, approach_D])
3. evaluate_thought for ALL branches

4. get_frontier()                    # see all options (non-destructive)
   → [{score: 0.88, "Approach A"}, {score: 0.82, "Approach B"}, ...]

5. For EACH promising branch:
     generate_thoughts(branch, [detail_1, detail_2, detail_3])
     evaluate_thought for all details

6. Repeat at next depth: get_frontier → expand → evaluate

7. get_all_paths()                   # compare ALL explored branches ranked
   → #1 avg=0.68 A → detail_2 → risk_mitigation
     #2 avg=0.66 A → detail_1 → diagnostic_pathway
     #3 avg=0.61 B → variant_3 → cost_analysis
     ...

8. mark_solution(best_path_leaf)
9. store_solution(tree_id, "comparative analysis", tags)
```

The key difference: `get_frontier` shows all options without popping, and `get_all_paths` enables informed comparison across branches before committing to a solution.

### Tree lifecycle

Trees have four states:

```
active  → solved       mark_solution() called
active  → paused       auto-pause (30 min idle) or LLM switches trees
active  → abandoned    LLM calls abandon_tree
paused  → active       LLM calls resume_tree
abandoned → active     LLM calls resume_tree
```

**Auto-pause.** Active trees untouched for 30 minutes are automatically paused when `list_trees` or `route_problem` runs. This keeps the active tree list clean without a background process.

**Topic routing.** Before calling `create_tree`, the LLM should call `route_problem` with the new problem statement. The server compares it against all active/paused trees using embedding cosine similarity (when a provider is active) and keyword overlap (always available). If both signals agree, a 20% hybrid boost is applied. If an existing tree matches (30%+ score), it returns `action: "continue"` instead of creating a duplicate.

## Experiment runner workflow

For autonomous research (e.g., ML training, hyperparameter tuning):

```
1. create_tree("Optimize metric X below threshold Y")

2. configure_experiment(
     tree_id,
     target_file: "train.py",
     run_command: "uv run train.py",
     metric_regex: "^val_bpb:\\s+([\\d.]+)",
     metric_direction: "lower",
     timeout_seconds: 600,
     work_dir: "/path/to/repo"
   )

3. generate_thoughts(root_id, ["Try MQA attention", "Increase depth", "Switch optimizer"])

4. search_step()  → highest-scored node

5. prepare_experiment(tree_id, patch_content: "...new train.py...", commit_message: "try MQA")
   → { commitHash, previousHash }

6. execute_experiment(tree_id, node_id, previous_hash)
   → runs command, parses metric, auto-evaluates node
   → improved: keeps commit, updates baseline
   → regressed: git resets to previous_hash
   → crashed: git resets, marks node impossible

7. search_step() → next node ... repeat ...

8. store_solution()  # save winning configuration
```

**Two-phase design.** `prepare_experiment` applies the patch and git commits. `execute_experiment` runs the command. This split lets the LLM review the change before burning minutes of compute.

**Git integration.** Each tree gets its own git branch (`autoresearch/<tree-id>`). Kept experiments advance the branch. Discarded experiments get `git reset --hard` to the previous commit. Full history in git, results in SQLite.

**Dashboard.** When experiments are running, the dashboard shows experiment stats (success rate, best metric), run history with keep/discard/crash status, and a metric progression chart. These panels are hidden for reasoning-only trees.

## Search strategies

**BFS (breadth-first search).** Explores shallowest nodes first, breaking ties by score. Use when you expect the solution at a known depth.

**DFS (depth-first search).** Explores deepest nodes first. Use when going deep quickly matters. Finds solutions faster but may miss better shallow alternatives.

**Beam search.** Keeps only the top-K scoring nodes in the frontier (K = branching_factor). Use for large branching factors where you want to focus on the most promising paths.

## Live dashboard

The binary automatically starts an HTTP server at `http://127.0.0.1:4545`.

**Interactive radial tree.** D3-powered radial tree visualization. Nodes color-coded by evaluation (blue = sure, amber = maybe, red = impossible, green = solution). Best path highlighted with thick green edges. Pan and zoom with mouse. Click any node to open a slide-in panel showing the complete reasoning path from root to that node with full analysis text at every depth level.

**Reasoning paths.** Below the tree, all explored paths are listed as expandable cards ranked by average score. Click to expand and read the full analysis at each depth (Problem, Approach, Implementation, Analysis).

**Solution store.** Past solutions displayed with full text, score badges, and tag pills. No truncation.

**Experiment stats.** For trees with experiments: run count, success rate, best metric, and a metric progression chart. Hidden for reasoning-only trees, replaced with frontier/depth/active node stats.

**Auto-refresh.** Polls every 10 seconds. Leave it open during an overnight autoresearch run and watch progress live.

Disable with `TOT_NO_DASHBOARD=1`. Change the port with `TOT_DASHBOARD_PORT=8080`.

## Architecture

```
tot-mcp/
  main.go                 Entry point. 30 tool registrations. Dashboard startup. CLI dispatch.
  cli.go                  Lightweight CLI for scripting (~1-2k tokens vs 10-50k MCP).
  Makefile                Cross-compilation targets.
  internal/
    db/
      db.go               SQLite init, schema migration, WAL mode. Pure Go (modernc.org/sqlite).
      audit.go            Audit trail logging and retrieval.
    tree/
      tree.go             Tree CRUD. BFS/DFS/Beam search. Frontier management. Recursive CTE for
                           path extraction and subtree pruning. Routing with hybrid embedding +
                           keyword scoring. Deep research (get_frontier, get_all_paths).
    retrieval/
      retrieval.go        Hybrid search: cosine similarity (pure Go) + FTS5 keyword matching.
                           Solution storage with embeddings. Compaction (memory decay).
    embeddings/
      embeddings.go       Pluggable providers: local (Hugot ONNX), OpenAI, Voyage AI, Ollama,
                           noop fallback. Thread-safe via sync.Once. Pure Go cosine similarity.
    encoding/
      encoding.go         Shared float32-to-bytes encoding for embedding BLOBs.
    experiment/
      experiment.go       Git management (branch, commit, reset). Shell command execution with
                           timeout. Metric parsing via regex. Auto-evaluation. Path traversal
                           protection on target files.
    dashboard/
      server.go           HTTP API: /api/trees, /api/tree/:id, /api/experiments/:id,
                           /api/retrieval/:id. JSON responses from SQLite.
      html.go             Embedded SPA. D3 radial tree with click-to-explore path panel.
                           Chart.js experiment metrics. Reasoning paths. Solution store.
```

### Database schema

```sql
trees               -- Tree metadata (id, problem, strategy, status, embedding BLOB)
nodes               -- Thought nodes (id, tree_id, parent_id, thought, evaluation, score, depth)
frontier            -- Expandable nodes (tree_id, node_id, priority)
solutions           -- Stored solutions with embeddings (problem, solution, thoughts, tags, embedding BLOB)
solutions_fts       -- FTS5 index for keyword search
solution_archive    -- Original content of compacted solutions (for restore)
experiment_configs  -- One config per tree (target file, run command, metric regex)
experiment_results  -- Every experiment run (metric, duration, status, commit hash, kept)
audit_log           -- Tool call audit trail (tree_id, node_id, tool, input, result)
tree_links          -- Cross-tree relationships (source, target, type, note)
```

All tables use WAL mode and foreign keys. Recursive CTEs handle path extraction (node to root) and subtree pruning (node to all descendants). Embedding BLOBs are little-endian encoded float32 arrays.

### Dependencies

| Dependency | Purpose | Why |
|---|---|---|
| `modernc.org/sqlite` | SQLite driver | Pure Go. Zero CGO. Static binary. No C compiler needed. |
| `github.com/mark3labs/mcp-go` | MCP SDK | Tool registration, stdio transport, JSON-RPC 2.0. |
| `github.com/google/uuid` | UUID generation | Node and tree IDs. |
| `github.com/knights-analytics/hugot` | On-device embeddings | Pure Go ONNX backend. Runs `all-MiniLM-L6-v2` locally. |
| `d3.js` (CDN) | Dashboard tree visualization | Radial tree layout with pan/zoom/click interaction. |

Four Go dependencies. No CGO. The binary is fully static and runs anywhere.

### Why Go over TypeScript

| | TypeScript (v1) | Go (v2) |
|---|---|---|
| Install | `npm install` + node-gyp + Python + C++ compiler | Download one binary |
| Startup | ~300ms cold start | ~5ms |
| Memory | ~50MB idle | ~10MB idle |
| Cross-compile | Platform-specific npm packages for native modules | `GOOS=linux go build` |
| SQLite | better-sqlite3 (native, build fails often) | modernc.org/sqlite (pure Go) |
| Distribution | git clone + npm install + npm build | Single file, chmod +x |
| Binary size | N/A (requires Node.js runtime) | ~15-20MB self-contained |

## Examples

### Medical differential diagnosis (deep research)

Use ToT with deep research to explore all viable diagnostic paths before concluding:

```
create_tree("45yo male, B-symptoms, bilateral hilar LAD, elevated LDH", strategy="beam", bf=5)

generate_thoughts(root, ["Lymphoma", "Sarcoidosis", "Tuberculosis", "Lung cancer", "Autoimmune"])
evaluate_thought(lymphoma, "sure", 0.88)     # B-symptoms + LAD + LDH classic
evaluate_thought(sarcoid, "sure", 0.82)      # bilateral hilar LAD textbook
evaluate_thought(tb, "sure", 0.70)           # must rule out
evaluate_thought(lung_ca, "maybe", 0.35)     # non-smoker, unlikely
evaluate_thought(autoimmune, "maybe", 0.25)  # no joint/skin symptoms

# Deep research: expand ALL top 3, not just beam-best
get_frontier()  → see all options
For each of [lymphoma, sarcoid, tb]:
  generate_thoughts(branch, [variant_1, variant_2, variant_3])
  evaluate_thought(...)

# Compare all paths before concluding
get_all_paths()
  → #1 avg=0.68: Lymphoma → DLBCL → Diagnostic pathway
    #2 avg=0.66: Lymphoma → Hodgkin → Treatment & prognosis
    #3 avg=0.62: Sarcoidosis → Sarcoid-lymphoma overlap → Biopsy critical
    #4 avg=0.56: TB → Miliary → HRCT discriminating
    ...

mark_solution(diagnostic_pathway_node)
store_solution(tree_id, "DLBCL primary, HL secondary, rule out TB...", tags=["differential-diagnosis"])
```

### Multi-tenant caching architecture

```
create_tree("Caching strategy for multi-tenant SaaS API, 10K RPM, $500 budget", strategy="beam", bf=4)

# Depth 1: Broad approaches
generate_thoughts(root, ["Redis per-tenant", "In-process LRU", "CDN edge", "Hybrid L1+L2"])

# Deep research all viable branches to depth 3
# ... expand Redis → managed/self-hosted variants
# ... expand Hybrid → DragonflyDB/Redis Cluster variants
# ... each variant → risk analysis and cost breakdown

get_all_paths()
  → #1 avg=0.67: Hybrid → DragonflyDB → SPOF replication ($120/mo)
    #2 avg=0.61: Redis → Upstash serverless ($60/mo)
    #3 avg=0.60: Hybrid → Redis Cluster + Spot instances ($120/mo)

store_solution(tree_id, "Hybrid L1+L2 with DragonflyDB...", tags=["caching", "multi-tenant"])
```

### Autonomous ML research

```
create_tree("Reduce val_bpb below 0.93", strategy="beam")
configure_experiment(tree_id, target_file="train.py", run_command="uv run train.py", ...)
generate_thoughts(root, ["MQA attention", "Increase depth", "Switch optimizer"])
search_step → MQA (highest score)
prepare_experiment(tree_id, patch_content="...", commit_message="try MQA")
execute_experiment(tree_id, mqa_node, previous_hash)
  → val_bpb: 0.938 (improved from 0.941, kept)
... repeat for hours ...
store_solution(tree_id, "MQA + depth 14 + rotary = 0.928")
```

## Contributing

```bash
# Build
go build -o tot-mcp .

# Run tests (56 tests across 5 packages)
go test ./...

# Type check
go vet ./...

# Cross-compile
make all
```

The project uses no CGO. `go build` produces a static binary on any platform with Go 1.23+.

## License

MIT. See [LICENSE](LICENSE).
