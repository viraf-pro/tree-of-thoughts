# Tree of Thoughts MCP

A single-binary MCP server for tree-structured reasoning with persistent storage, hybrid retrieval, a compounding knowledge graph, web research ingestion, real-time subscriptions, an autonomous experiment runner, a live web dashboard, and a Claude Code plugin with 21 skills, 7 agents, and harness engineering hooks.

Built on the [Tree of Thoughts](https://arxiv.org/abs/2305.10601) framework (Yao et al., 2023). The server gives LLMs structured exploration over multiple reasoning paths with evaluation, backtracking, and search algorithms, instead of linear chain-of-thought.

## The idea in plain English

When you ask an AI a question, it thinks in a straight line — one thought leads to the next, like walking down a single hallway. If it picks the wrong direction early on, it's stuck with a bad answer.

Tree of Thoughts fixes this by making the AI think more like a human solving a hard problem. Instead of one hallway, imagine a **tree** with many branches:

1. **Branch out.** When facing a problem, the AI comes up with 3-5 different ideas at once (not just one).
2. **Rate each idea.** It scores them: "this one looks promising," "this one might work," or "this is a dead end." Dead ends get chopped off.
3. **Go deeper on the good ones.** For each promising idea, it branches again — more sub-ideas, more scoring.
4. **Compare everything.** After exploring many branches, it looks at ALL the paths it tried and picks the best one based on evidence, not gut instinct.

Think of it like a chess player who considers multiple moves ahead on different branches, instead of just playing the first move that looks okay.

Everything the AI figures out gets saved in a **compounding knowledge graph**. Solutions are cross-referenced, tagged with confidence scores, and analyzed for topology (god nodes, communities, bridge connections). Next time a similar problem comes up, it remembers past solutions and builds on them — a notebook that gets smarter over time.

## What it does

**Tree reasoning.** The LLM generates multiple candidate thoughts at each step, evaluates them (sure/maybe/impossible), and uses search algorithms (BFS, DFS, Beam) to decide which branch to explore next. Dead ends get pruned. The best path gets extracted.

**Deep research.** Unlike greedy beam search that follows a single path, deep research explores all viable branches before concluding. The `get_frontier` tool shows all expandable nodes without consuming them, and `get_all_paths` ranks every explored branch for comparison. This prevents premature conclusions and surfaces non-obvious solutions.

**Compounding knowledge graph.** Solutions are automatically cross-referenced when stored. Each link carries a confidence score (0.0-1.0) and source label (manual vs auto). Graph topology analysis identifies god nodes (most connected solutions), communities (clusters of related knowledge), and bridge edges (surprising cross-domain connections). Knowledge compounds across sessions — it doesn't just accumulate, it interconnects.

**Web research ingestion.** The `ingest_url` tool fetches web pages (articles, docs, papers) and stores their content as solutions for future retrieval. Secure fetching: HTTP/HTTPS only, localhost/metadata endpoints blocked, 1MB size cap, redirect validation. HTML is stripped to clean text with title extraction.

**Knowledge quality sensors.** Three computational feedback tools maintain knowledge health:
- `lint_knowledge` — structural health-check with actionable remediations (specific tool calls to fix each issue)
- `drift_scan` — entropy detection (duplicate trees, abandoned trees with value, never-retrieved solutions)
- `knowledge_report` — structured overview of what the knowledge base knows (the "map" agents read before querying)

**Design rationale capture.** When solutions are stored, the system auto-generates a rationale from the tree exploration: "Evaluated 3 branches. Selected: X (score 0.85). Considered: Y (score 0.62)." This captures *why* a solution was chosen, not just what it is.

**Token-budgeted retrieval.** The `retrieve_context` tool accepts an optional `max_tokens` parameter to cap response size. Solutions are truncated to fit within the budget, reducing context window consumption.

**Persistent storage.** All trees, nodes, solutions, links, and knowledge events live in a SQLite database. Survives restarts. A single `.db` file holds everything.

**Hybrid retrieval.** Past solutions are stored with embeddings. When a new problem arrives, the server searches by vector similarity (cosine distance) and keyword matching (FTS5), then merges the results. Solutions that match on both get boosted. Knowledge compounds across sessions.

**Smart routing.** Before creating a new tree, `route_problem` checks existing trees using embedding cosine similarity (primary) with Jaccard keyword overlap (fallback). When both signals agree, a hybrid boost is applied. This prevents duplicate trees across sessions even when the problem is rephrased.

**Progressive disclosure.** The `get_tree_context` tool provides tiered context for resuming trees: summary tier (problem, status, best path, frontier) or full tier (adds pruned branches, all paths, cross-tree links). Agents get exactly the context they need without overwhelming their context window.

**Obsidian vault export.** Export the entire knowledge base as an Obsidian vault with interlinked markdown files, YAML frontmatter, wiki-links between solutions, and a tag-grouped index. Browse your knowledge graph visually in Obsidian's graph view.

**Experiment runner.** An optional autoresearch-style loop: modify code, run training/evaluation, parse the metric, keep or discard, repeat. The ToT tree guides which experiments to try. Inspired by [karpathy/autoresearch](https://github.com/karpathy/autoresearch).

**Real-time subscriptions.** An internal event bus publishes typed events for every mutation (thought added, evaluated, pruned, solution stored, experiment completed, etc.). Three consumers tap in: MCP clients receive `notifications/resources/updated` for subscribed resource URIs, the web dashboard gets instant SSE updates, and all events are logged. Clients can subscribe to 7 resource URI patterns (`tot://tree/{id}`, `tot://tree/{id}/frontier`, `tot://solutions`, etc.) and get notified the moment something changes.

**Live dashboard.** A web UI at `localhost:4545` renders an interactive D3 radial tree with click-to-explore path analysis, experiment history, metric charts, and a full-text solution store. Updates in real-time via Server-Sent Events (SSE), with polling fallback. Create new trees directly from the dashboard via the "+ New Tree" button.

**Claude Code plugin.** Install as a Claude Code plugin for the richest experience. Ships 21 skills (create-tree, deep-research, decide, run-experiment, knowledge-health, etc.), 7 specialized agents (researcher, experimenter, librarian, critic, synthesizer, conductor, scout), and harness engineering hooks. The plugin auto-downloads the binary on first use.

**Agent workflows.** Multi-agent pipelines with explicit handoff and feedback loops, based on [Harness Engineering](https://martinfowler.com/articles/harness-engineering.html):
- `research-and-validate` — scout → researcher → critic → [revision loop] → librarian
- `experiment-loop` — scout → researcher (hypothesize) → experimenter (test) → researcher (interpret) → librarian
- `knowledge-maintenance` — librarian (detect) → synthesizer (consolidate) → critic (validate) → librarian (apply)

**Harness engineering hooks.** Feedforward guides steer agents before they act (session briefing, duplicate tree prevention, prior knowledge injection). Feedback sensors verify quality after mutations (research depth checks, orphan detection, experiment safety, session-end quality gate).

## Quick start

```bash
# Clone
git clone https://github.com/viraf-pro/tree-of-thoughts.git
cd tree-of-thoughts

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
curl -L https://github.com/viraf-pro/tree-of-thoughts/releases/latest/download/tot-mcp-linux-amd64.tar.gz | tar xz
chmod +x tot-mcp

# macOS Apple Silicon
curl -L https://github.com/viraf-pro/tree-of-thoughts/releases/latest/download/tot-mcp-darwin-arm64.tar.gz | tar xz
chmod +x tot-mcp

# Windows (PowerShell)
Invoke-WebRequest https://github.com/viraf-pro/tree-of-thoughts/releases/latest/download/tot-mcp-windows-amd64.zip -OutFile tot-mcp.zip
Expand-Archive tot-mcp.zip -DestinationPath .
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

The binary doubles as a lightweight CLI for scripting, CI pipelines, and agent sensor feedback (~1-2k tokens vs 10-50k for the full MCP tool schema).

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

The `lint`, `drift`, `report`, and `health` commands output structured JSON, making them usable as computational feedback sensors in CI pipelines, pre-commit hooks, or background monitoring.

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

**Option A: MCP server only**

```bash
claude mcp add tree-of-thoughts /absolute/path/to/tot-mcp
```

**Option B: Full plugin (recommended)**

Install as a Claude Code plugin for skills, agents, and hooks:

```bash
# Local testing
claude --plugin-dir /path/to/tree-of-thoughts

# Or from marketplace (when published)
# claude plugin install tree-of-thoughts
```

The plugin includes:
- **21 skills** — `/tree-of-thoughts:create-tree`, `:deep-research`, `:decide`, `:run-experiment`, etc.
- **7 agents** — researcher, experimenter, librarian, critic, synthesizer, conductor, scout
- **7 hooks** — session briefing, duplicate prevention, research verification, knowledge lint
- Auto-downloads the binary from GitHub releases on first use

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

Restart ChatGPT after saving. The tools will appear in the toolbox icon at the bottom of the chat input.

> **Note:** ChatGPT currently supports tools only — prompts and resources are not yet supported. All 39 tot-mcp tools are tool-type, so everything works.

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

## Tools (39)

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

### Deep research (3)

| Tool | Description |
|---|---|
| `get_frontier` | List all expandable frontier nodes ranked by score **without removing them**. Unlike `search_step`, this is non-destructive. Use to see all options before deciding which branches to expand. Essential for deep research. |
| `get_all_paths` | Return all paths to leaf and terminal nodes, ranked by average score. Use to compare all explored branches before marking a solution. |
| `get_tree_context` | Progressive-disclosure context for resuming a tree. `detail="summary"` returns problem, status, best path, frontier. `detail="full"` adds pruned branches, all paths, and cross-tree links. |

### Tree lifecycle (4)

| Tool | Description |
|---|---|
| `route_problem` | **Call before create_tree.** Checks if the new problem matches an existing active/paused tree using embedding cosine similarity (primary) + Jaccard keyword overlap (fallback). Hybrid boost when both signals agree. Returns `action: "continue"` with the tree ID, or `action: "create"`. |
| `resume_tree` | Reactivate a paused or abandoned tree. |
| `abandon_tree` | Mark a tree as abandoned. Tree stays in the database but is excluded from routing. |
| `suggest_next` | Zero-arg "what should I work on next?" Returns the most promising active or paused tree. |

### Knowledge store (4)

| Tool | Description |
|---|---|
| `retrieve_context` | Hybrid search past solutions. Vector similarity + FTS5 keyword matching. Results merged, hybrid matches boosted 20%. Optional `max_tokens` caps response size. |
| `store_solution` | Save a completed solution with tags. Generates an embedding for future semantic search. Auto-generates design rationale from tree exploration. Auto-links to similar existing solutions with confidence scores. |
| `retrieval_stats` | Total solutions, embedding coverage, compaction stats. |
| `ingest_url` | Fetch a web page (article, docs, paper) and store its content as a solution. HTTP/HTTPS only, 1MB size cap, secure redirect validation. |

### Knowledge graph (6)

| Tool | Description |
|---|---|
| `link_solutions` | Create a cross-reference between solutions with confidence score (0.0-1.0) and source label (manual/auto). Link types: related, supersedes, contradicts, extends. |
| `get_solution_links` | View all cross-references for a solution with confidence scores. |
| `link_trees` | Create a dependency between two trees (depends_on, informs, supersedes, related). |
| `get_tree_links` | View all cross-tree dependencies and relationships. |
| `knowledge_graph` | Analyze graph topology: god nodes (most connected solutions), communities (connected components with tag aggregation), bridge edges (cross-community connections). |
| `knowledge_report` | Structured overview: top solutions, tag coverage, graph summary, recent events, suggested queries. The "map" agents should read before querying. |

### Quality & maintenance (5)

| Tool | Description |
|---|---|
| `lint_knowledge` | Health-check: orphan solutions, missing cross-references, stale entries, similar pairs. Returns **remediations** — specific tool calls to fix each issue (positive prompt injection). |
| `drift_scan` | Entropy detection: duplicate trees with similar problems, abandoned trees with valuable content, solutions that were never retrieved. Returns remediations. |
| `knowledge_log` | View the knowledge evolution timeline: stored, linked, retrieved, lint, drift_scan events. |
| `audit_log` | View the audit trail of tool calls for debugging and decision tracing. |
| `open_dashboard` | Returns the dashboard URL. Optionally links directly to a specific tree. |

### Compaction (3)

| Tool | Description |
|---|---|
| `compact_analyze` | Find solutions older than N days eligible for compression. Returns full content for summary generation. Default: 30 days. |
| `compact_apply` | Replace a solution's detailed thoughts with a compressed summary. Original archived and restorable. Embedding stays intact. |
| `compact_restore` | Restore a compacted solution to its original full content from the archive. |

### Experiment runner (4)

| Tool | Description |
|---|---|
| `configure_experiment` | Set target file, run command, metric regex, direction, timeout, work dir, git branch prefix. |
| `prepare_experiment` | Apply a code patch (full file replacement) and git commit. Returns commit hash and previous hash for rollback. |
| `execute_experiment` | Run the command, parse the metric, auto-evaluate the thought node, keep or git-reset. |
| `experiment_history` | View experiment stats: total runs, success rate, best metric, crash count. |

## Reasoning workflows

### Standard ToT loop

```
0. suggest_next()                                   → "create" (nothing to resume)
1. route_problem("Solve problem X")                 → "create" (no match)
2. create_tree("Solve problem X", strategy="beam", branching_factor=3)
3. retrieve_context("problem X keywords")            # check past solutions
4. generate_thoughts(root_id, [idea_1, idea_2, idea_3])
5. evaluate_thought(node_1, "sure", 0.8)
   evaluate_thought(node_2, "maybe", 0.5)
   evaluate_thought(node_3, "impossible")             # pruned
6. search_step()                                      → node_1
7. generate_thoughts(node_1, [refinement_a, refinement_b])
8. evaluate_thought(...) → search_step() → ... repeat ...
9. mark_solution(winning_node)
10. get_best_path()                                   # extract full chain
11. store_solution(tree_id, "solution text", tags)    # auto-links, auto-rationale
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

8. mark_solution(best_path_leaf)
9. store_solution(tree_id, "comparative analysis", tags)
```

### Resuming a tree (progressive disclosure)

```
1. suggest_next()                    → "continue", tree_id
2. get_tree_context(tree_id, "summary")
   → problem, status, best path, frontier nodes
3. get_tree_context(tree_id, "full")   # if more context needed
   → adds pruned branches, all paths, cross-tree links
4. resume_tree(tree_id)
5. ... continue exploration ...
```

### Web research ingestion

```
1. ingest_url("https://arxiv.org/abs/2305.10601", tags=["paper", "tot"])
   → fetches, strips HTML, stores as solution with "ingested" tag
2. ingest_url("https://docs.example.com/api", tags=["docs"])
3. retrieve_context("tree of thoughts")
   → finds ingested articles alongside tree-generated solutions
```

### Knowledge maintenance

```
1. lint_knowledge()                  # structural health-check
   → { unlinked: 5, orphan: 2, remediations: [...] }
   # Each remediation has the exact tool call to fix it

2. drift_scan()                      # entropy detection
   → { duplicateTreePairs: [...], abandonedWithValue: [...] }

3. knowledge_report()                # the "map" of what you know
   → { topSolutions, tagCoverage, communities, suggestedQueries }

4. knowledge_graph()                 # topology analysis
   → { godNodes, communities, bridges }

5. export --obsidian ./vault         # browse in Obsidian
```

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

**Real-time updates.** Live updates via Server-Sent Events (SSE). The tree visualization redraws instantly when thoughts are added or evaluated. Falls back to 10-second polling if SSE is unavailable.

Disable with `TOT_NO_DASHBOARD=1`. Change the port with `TOT_DASHBOARD_PORT=8080`.

## Architecture

```
tot-mcp/
  main.go                 Entry point. 39 tools, 7 resources. Dashboard + event bus startup.
  cli.go                  21 CLI commands for scripting and CI sensors.
  internal/
    db/                   SQLite init, schema, WAL mode, audit logging. Pure Go.
    tree/                 Tree CRUD, BFS/DFS/Beam search, frontier, routing, cross-tree links.
    retrieval/            Hybrid search (vector + FTS5), knowledge graph, lint, drift, compaction.
    events/               Event bus (pub/sub), MCP notification bridge, 15 event types.
    resources/            7 MCP resource templates (tot://tree/{id}, tot://solutions, etc.).
    experiment/           Git-based experiment runner with metric parsing and auto-evaluation.
    dashboard/            HTTP API + embedded SPA with SSE real-time updates.
    embeddings/           Pluggable providers: local ONNX, OpenAI, Voyage, Ollama.
    web/                  Secure URL fetching with HTML-to-text extraction.
    encoding/             Shared float32-to-bytes encoding for embedding BLOBs.
  skills/                 21 Claude Code skills (create-tree, deep-research, decide, etc.).
  agents/                 7 specialized agents (researcher, critic, conductor, etc.).
  hooks/                  Harness engineering hooks (session briefing, verification, etc.).
  scripts/                8 hook scripts (install, briefing, verification, lint, safety).
  .claude-plugin/         Plugin manifest for Claude Code distribution.
```

### Database schema

```sql
trees               -- Tree metadata (id, problem, strategy, status, embedding BLOB)
nodes               -- Thought nodes (id, tree_id, parent_id, thought, evaluation, score, depth)
frontier            -- Expandable nodes (tree_id, node_id, priority)
solutions           -- Stored solutions with embeddings, rationale, and design reasoning
solutions_fts       -- FTS5 index for keyword search
solution_archive    -- Original content of compacted solutions (for restore)
solution_links      -- Cross-references between solutions (type, confidence, source)
knowledge_log       -- Knowledge evolution events (stored, linked, retrieved, lint, drift)
experiment_configs  -- One config per tree (target file, run command, metric regex)
experiment_results  -- Every experiment run (metric, duration, status, commit hash, kept)
audit_log           -- Tool call audit trail (tree_id, node_id, tool, input, result)
tree_links          -- Cross-tree relationships (source, target, type, note)
```

All tables use WAL mode and foreign keys. Recursive CTEs handle path extraction (node to root) and subtree pruning (node to all descendants). Embedding BLOBs are little-endian encoded float32 arrays. Solution links have a UNIQUE constraint on (source_id, target_id, link_type) to prevent duplicates.

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

## Design influences

This project incorporates ideas from three sources:

- **[Karpathy's LLM Wiki](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f)** — Persistent compounding knowledge artifacts. Solutions cross-reference each other, knowledge is compiled once and kept current, lint operations detect contradictions and gaps. The knowledge graph is a persistent artifact that compounds, not a flat database that merely accumulates.

- **[Harness Engineering](https://martinfowler.com/articles/harness-engineering.html) ([Fowler](https://martinfowler.com/articles/harness-engineering.html), [OpenAI](https://openai.com/index/harness-engineering/))** — Feedforward guides (skills, session briefing, duplicate prevention) and computational feedback sensors (verify-research, verify-experiment, verify-knowledge, session quality gate). Multi-agent workflows with the conductor agent orchestrating feedback loops. Hook-based control system with context baton handoff between agents.

- **[Graphify](https://graphify.net/)** — Graph topology analysis (god nodes, communities, bridge edges), confidence-scored relationships, token-budgeted retrieval, design rationale capture, multi-format export (Obsidian vault).

## Contributing

```bash
# Build
go build -o tot-mcp .

# Run tests (215 tests across 10 packages)
go test ./...

# Type check
go vet ./...

# Cross-compile
make all
```

The project uses no CGO. `go build` produces a static binary on any platform with Go 1.24+.

Releases are automated via GoReleaser — push a `v*` tag to run tests and build cross-platform binaries.

## License

MIT. See [LICENSE](LICENSE).
