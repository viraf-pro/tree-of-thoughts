# tot-mcp

A single-binary MCP server for tree-structured reasoning with persistent storage, hybrid retrieval, an autonomous experiment runner, and a live web dashboard.

Built on the [Tree of Thoughts](https://arxiv.org/abs/2305.10601) framework (Yao et al., 2023). The server gives LLMs structured exploration over multiple reasoning paths with evaluation, backtracking, and search algorithms, instead of linear chain-of-thought.

## What it does

**Tree reasoning.** The LLM generates multiple candidate thoughts at each step, evaluates them (sure/maybe/impossible), and uses search algorithms (BFS, DFS, Beam) to decide which branch to explore next. Dead ends get pruned. The best path gets extracted.

**Persistent storage.** All trees, nodes, solutions, and experiment results live in a SQLite database. Survives restarts. A single `.db` file holds everything.

**Hybrid retrieval.** Past solutions are stored with embeddings. When a new problem arrives, the server searches by vector similarity (cosine distance) and keyword matching (FTS5), then merges the results. Solutions that match on both get boosted. Knowledge compounds across sessions.

**Experiment runner.** An optional autoresearch-style loop: modify code, run training/evaluation, parse the metric, keep or discard, repeat. The ToT tree guides which experiments to try. Inspired by [karpathy/autoresearch](https://github.com/karpathy/autoresearch).

**Live dashboard.** A web UI at `localhost:4545` renders the reasoning tree, experiment history, metric charts, and retrieval store. Auto-refreshes every 10 seconds.

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

## Connect to Claude Desktop

Add to your `claude_desktop_config.json`:

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

## Configuration

All configuration is via environment variables.

### Core

| Variable | Purpose | Default |
|---|---|---|
| `TOT_DB_PATH` | SQLite database file location | `~/.tot-mcp/tot.db` |
| `TOT_DASHBOARD_PORT` | Dashboard HTTP port | `4545` |
| `TOT_NO_DASHBOARD` | Set to any value to disable dashboard | (not set) |

### Embedding providers (optional, enables semantic search)

Without an embedding provider, retrieval falls back to FTS5 keyword search only. Set one of these to enable vector search:

| Variable | Provider | Default model |
|---|---|---|
| `OPENAI_API_KEY` | OpenAI | `text-embedding-3-small` (1536d) |
| `VOYAGE_API_KEY` | Voyage AI | `voyage-3-lite` (512d) |
| `OLLAMA_BASE_URL` | Ollama (local) | `mxbai-embed-large` (1024d) |

Override the model with `TOT_EMBED_MODEL`. Force a specific provider with `TOT_EMBED_PROVIDER` (openai, voyage, or ollama).

## Tools (18)

### Tree operations

| Tool | Description |
|---|---|
| `create_tree` | Initialize a new reasoning tree. Params: problem, search_strategy (bfs/dfs/beam), max_depth, branching_factor. |
| `generate_thoughts` | Add candidate thoughts as children of a parent node. Accepts an array of thought objects. |
| `evaluate_thought` | Score a node as "sure", "maybe", or "impossible". Optional custom 0-1 score. Impossible nodes are pruned from the search frontier. |
| `search_step` | Returns the next node to expand based on the tree's search strategy. BFS = shallowest first. DFS = deepest first. Beam = highest score first. |
| `backtrack` | Prune a node and all descendants (recursive CTE). Returns the parent node. |
| `mark_solution` | Flag a node as the terminal answer. Removes it from the frontier. |
| `get_best_path` | Extract the highest-scoring complete path from root to a terminal/leaf node. |
| `get_tree_summary` | Stats: total nodes, terminal count, pruned count, frontier size, max depth reached. |
| `inspect_node` | View a node, its children, and the full path from root. |
| `list_trees` | List all reasoning trees in the database. |

### Retrieval

| Tool | Description |
|---|---|
| `retrieve_context` | Hybrid search past solutions. Vector similarity (if embeddings configured) + FTS5 keyword matching. Results merged, hybrid matches boosted. |
| `store_solution` | Save a completed solution with tags. Generates an embedding for future semantic search. Links back to the tree and best path. |
| `retrieval_stats` | Total solutions, embedding coverage, available tags. |

### Experiment runner

| Tool | Description |
|---|---|
| `configure_experiment` | Set the target file, run command, metric regex, timeout, working directory, and git branch prefix for a tree. Call once per tree. |
| `prepare_experiment` | Apply a code patch (full file replacement) and git commit. Returns commit hash and previous hash for rollback. |
| `execute_experiment` | Run the training command, parse the metric, auto-evaluate the thought node, keep or git-reset. Long-running (minutes). |
| `experiment_history` | View experiment stats: total runs, success rate, best metric, crash count. |

### Dashboard

| Tool | Description |
|---|---|
| `open_dashboard` | Returns the dashboard URL. Optionally links directly to a specific tree. |

## Reasoning workflow

The standard ToT loop:

```
1. create_tree("Solve problem X", strategy="beam", branching_factor=3)
2. retrieve_context("problem X keywords")          # check past solutions
3. generate_thoughts(root_id, [idea_1, idea_2, idea_3])
4. evaluate_thought(node_1, "sure", 0.8)
   evaluate_thought(node_2, "maybe", 0.5)
   evaluate_thought(node_3, "impossible")           # pruned
5. search_step()                                     # returns node_1
6. generate_thoughts(node_1, [refinement_a, refinement_b])
7. evaluate_thought(...)
8. search_step()
   ... repeat until solved ...
9. mark_solution(winning_node)
10. get_best_path()                                   # extract full chain
11. store_solution(tree_id, "solution text", tags)     # save for retrieval
```

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

4. search_step()  # returns highest-scored node

5. prepare_experiment(tree_id, node_id, patch_content: "...new train.py...", commit_message: "try MQA")
   # returns { commitHash, previousHash }

6. execute_experiment(tree_id, node_id, previous_hash)
   # runs ~5 min, parses metric, auto-evaluates node
   # if improved: keeps commit, updates baseline
   # if regressed: git resets to previous_hash
   # if crashed: git resets, marks node impossible

7. search_step()  # next node
   ... repeat ...

8. store_solution()  # save winning configuration
```

## Search strategies

**BFS (breadth-first search).** Explores shallowest nodes first, breaking ties by score. Use when you expect the solution at a known depth. Ensures all candidates at depth N are evaluated before going to depth N+1.

**DFS (depth-first search).** Explores deepest nodes first. Use when going deep quickly matters. Finds solutions faster but may miss better shallow alternatives.

**Beam search.** Keeps only the top-K scoring nodes in the frontier (K = branching_factor). Use for large branching factors where you want to focus resources on the most promising paths. Combines breadth with scoring.

## Live dashboard

The binary automatically starts an HTTP server at `http://127.0.0.1:4545`.

**Tree list view.** All reasoning trees with node count, experiment count, strategy, and status.

**Tree detail view.** Interactive SVG tree visualization. Nodes color-coded by evaluation (blue = sure, amber = maybe, red = impossible, green = solution). Best path highlighted. Click any node for details.

**Experiment history.** Each run listed with metric, duration, keep/discard/crash status.

**Metric chart.** Bar chart tracking metric progression over experiments.

**Solution store.** Past solutions available for retrieval with scores and tags.

**Auto-refresh.** Polls every 10 seconds. Leave it open during an overnight autoresearch run and watch progress live.

Disable with `TOT_NO_DASHBOARD=1`. Change the port with `TOT_DASHBOARD_PORT=8080`.

## Architecture

```
tot-mcp/
  main.go                              Entry point. 18 tool registrations. Dashboard startup.
  go.mod                               Module definition and dependencies.
  Makefile                             Cross-compilation targets.
  LICENSE                              MIT license.
  internal/
    db/
      db.go                            SQLite init, schema migration, WAL mode. Pure Go (modernc.org/sqlite).
    tree/
      tree.go                          Tree CRUD. BFS/DFS/Beam search. Recursive CTE for path extraction
                                       and subtree pruning. Frontier management.
    retrieval/
      retrieval.go                     Hybrid search: cosine similarity (pure Go) + FTS5 keyword matching.
                                       Solution storage with embeddings. Score merging and boosting.
    embeddings/
      embeddings.go                    Pluggable providers: OpenAI, Voyage AI, Ollama, noop fallback.
                                       Pure Go cosine similarity (no C extensions needed).
    experiment/
      experiment.go                    Git management (branch, commit, reset). Child process execution
                                       with timeout. Metric parsing via regex. Auto-evaluation.
                                       Result logging to SQLite.
    dashboard/
      server.go                        HTTP API: /api/trees, /api/tree/:id, /api/experiments/:id,
                                       /api/retrieval/:id. JSON responses from SQLite.
      html.go                          Embedded SPA. Tree visualization, Chart.js metrics,
                                       experiment history, solution store. Auto-refresh.
```

### Database schema

```sql
trees                   -- Reasoning tree metadata (id, problem, strategy, status)
nodes                   -- Thought nodes (id, tree_id, parent_id, thought, evaluation, score, depth)
frontier                -- Nodes waiting to be expanded (tree_id, node_id, priority)
solutions               -- Stored solutions with embeddings (problem, solution, thoughts, tags, embedding BLOB)
solutions_fts           -- FTS5 index for keyword search
experiment_configs      -- One config per tree (target file, run command, metric regex)
experiment_results      -- Every experiment run logged (metric, duration, status, commit hash, kept)
```

All tables use WAL mode and foreign keys. Recursive CTEs handle path extraction (node → root) and subtree pruning (node → all descendants).

### Dependencies

| Dependency | Purpose | Why |
|---|---|---|
| `modernc.org/sqlite` | SQLite driver | Pure Go. Zero CGO. Static binary. No C compiler needed. |
| `github.com/mark3labs/mcp-go` | MCP SDK | Tool registration, stdio transport, JSON-RPC 2.0. |
| `github.com/google/uuid` | UUID generation | Node and tree IDs. |

Three dependencies. No CGO. The binary is fully static and runs anywhere.

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

## Retrieval system

The retrieval store creates a knowledge flywheel. Every solved problem becomes context for the next one.

**Storage.** When `store_solution` is called, the server concatenates the problem, thoughts, and solution text, generates an embedding via the configured provider, and stores it as a BLOB in SQLite alongside the text and tags.

**Search.** When `retrieve_context` is called:
1. The query is embedded and compared against all stored solution embeddings via cosine similarity (pure Go, no C extension).
2. The query is also run through FTS5 for keyword matching.
3. Results from both are merged by solution ID. Solutions that appear in both vector and keyword results get a 20% similarity boost ("hybrid" match).
4. Results are sorted by similarity and capped at top_k.

**Fallback.** Without an embedding provider, step 1 is skipped. FTS5 keyword search still works. You get keyword matches without semantic understanding.

**Cross-session value.** After 50-100 stored solutions, `retrieve_context` starts surfacing relevant prior reasoning before you even begin a new tree. The server evolves from a reasoning tool into a reasoning memory.

## Experiment runner

The experiment runner closes the loop between planning (ToT) and execution (running actual code). It replaces the greedy "try one thing, keep or discard" loop with tree-guided exploration.

**Two-phase design:**

1. `prepare_experiment` — applies a code patch, git commits. Returns the commit hash and previous hash. The LLM reviews the change before burning 5 minutes of compute.

2. `execute_experiment` — spawns the training command as a child process, captures stdout/stderr to a log file, enforces a hard timeout. After completion: parses the metric via regex, compares against the baseline, auto-evaluates the thought node, and either keeps the commit (if improved) or git resets (if not).

**Configurable for any codebase.** The experiment runner is not locked to any specific setup. Configure the target file, run command, metric regex, direction (lower/higher), timeout, and git branch prefix. Works for ML training, hyperparameter tuning, prompt engineering evaluation, or anything with a measurable output.

**Git integration.** Each tree gets its own git branch (`autoresearch/<tree-id>`). Kept experiments advance the branch. Discarded experiments get `git reset --hard` to the previous commit. The full history is in git; the results are in SQLite.

## Examples

### Market expansion analysis

Use ToT to evaluate which new vertical a company should enter:

```
create_tree("Identify best new market for [company]", strategy="beam", branching_factor=4)
generate_thoughts(root, ["Cannabis", "Nutraceuticals", "Pet food", "Cosmetics"])
evaluate_thought(cannabis, "sure", 0.88)    # mandated traceability
evaluate_thought(nutra, "sure", 0.78)       # enterprise-heavy
evaluate_thought(petfood, "maybe", 0.72)    # small SMB tier
evaluate_thought(cosmetics, "maybe", 0.55)  # weak compliance driver
search_step → cannabis (highest beam score)
... expand cannabis branch ...
mark_solution(winning_node)
store_solution(tree_id, "Enter cannabis via seed-to-sale module", tags=["market-expansion"])
```

### Autonomous ML research

Use ToT + experiment runner to optimize a training script overnight:

```
create_tree("Reduce val_bpb below 0.93", strategy="beam")
configure_experiment(tree_id, target_file="train.py", run_command="uv run train.py", ...)
generate_thoughts(root, ["MQA attention", "Increase depth", "Switch optimizer"])
search_step → MQA (highest score)
prepare_experiment(tree_id, mqa_node, patch_content="...", commit_message="try MQA")
execute_experiment(tree_id, mqa_node, previous_hash)
  → val_bpb: 0.938 (improved from 0.941, kept)
search_step → depth increase
... repeat for hours ...
store_solution(tree_id, "MQA + depth 14 + rotary = 0.928")
```

### Complex debugging

Use ToT to systematically diagnose a production issue:

```
create_tree("Why does app have 4s TTFB", strategy="dfs")
generate_thoughts(root, ["Database queries", "DNS resolution", "Cold start", "Middleware"])
evaluate_thought(dns, "impossible")  # ruled out by dig test
backtrack(dns)                        # prune, return to root
search_step → database (DFS goes deep)
generate_thoughts(db_node, ["N+1 queries", "Missing index", "Connection pool"])
... continue until root cause found ...
```

## Contributing

```bash
# Run locally
go run .

# Type check
go vet ./...

# Build
go build -o tot-mcp .

# Cross-compile
make all
```

The project uses no CGO. `go build` produces a static binary on any platform with Go 1.23+.

## License

MIT. See [LICENSE](LICENSE).
