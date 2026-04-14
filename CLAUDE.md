# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

tot-mcp is a Tree of Thoughts (ToT) MCP server written in Go. It provides structured multi-path reasoning, persistent storage (SQLite), hybrid retrieval (vector + FTS5), real-time subscriptions, autonomous experiment execution, a live web dashboard, and a Claude Code plugin with 21 skills, 7 agents, and harness engineering hooks. Single binary, pure Go, zero CGO.

## Build & Run

```bash
make build                    # Build binary (./tot-mcp)
make all                      # Cross-compile all platforms → dist/
make clean                    # Remove artifacts
go build -o tot-mcp .         # Direct build
go vet ./...                  # Lint/check
```

215 tests across 10 packages (`go test ./...`). No package.json — this is a pure Go project (go 1.24).

**Running:**
- `./tot-mcp` — MCP stdio server + HTTP dashboard (port 4545)
- `./tot-mcp <command>` — CLI mode (21 commands: list, suggest, show, route, create, add, eval, solve, ready, audit, stats, compact, lint, health, drift, report, export, ingest, version, help)

## Architecture

The binary operates in two modes based on `os.Args`: CLI mode (with arguments) or MCP server mode (no arguments, stdio JSON-RPC).

### Root-level files
- **main.go** — MCP server setup, tool registration (39 tools across 4 categories), resource registration, event bus + MCP bridge startup, embedding provider init, dashboard startup
- **cli.go** — Lightweight CLI dispatcher (~1-2k tokens vs 10-50k for full MCP schemas)

### Internal packages (`internal/`)

| Package | Purpose |
|---|---|
| `db` | SQLite init (WAL mode, singleton via sync.Once), schema DDL, audit logging |
| `tree` | Core tree CRUD, search strategies (BFS/DFS/Beam), frontier management, pruning via recursive CTEs, routing (Jaccard similarity), cross-tree links, lifecycle (active/paused/abandoned/solved) |
| `retrieval` | Hybrid search: vector cosine similarity + FTS5 keyword, 20% boost for dual-match, solution compaction (memory decay at 30+ days) |
| `embeddings` | Pluggable provider interface (`Embed(text) → []float32`). Auto-selects: OpenAI → Voyage → Ollama → local Hugot (pure Go ONNX, all-MiniLM-L6-v2) → Noop |
| `experiment` | Two-phase autonomous runner: prepare (apply patch, git commit) → execute (run command, parse metric via regex, auto-evaluate, rollback on failure) |
| `events` | Internal event bus (typed pub/sub, buffered channel fan-out, global singleton). MCP notification bridge maps events to affected resource URIs and calls `SendNotificationToAllClients`. 15 event types across tree, experiment, retrieval, and knowledge domains |
| `resources` | MCP resource templates (7 URI patterns: `tot://trees`, `tot://tree/{id}`, `tot://tree/{id}/frontier`, `tot://tree/{id}/experiments`, `tot://tree/{id}/status`, `tot://solutions`, `tot://solution/{id}`). Read handlers backed by the same SQLite queries as the dashboard |
| `dashboard` | HTTP API server + embedded SPA (SVG tree visualization, Chart.js metrics). SSE endpoint (`/api/events`) for real-time push updates. REST endpoints at `/api/trees`, `/api/tree/:id`, etc. |

### Key design patterns
- **Single-writer SQLite:** `SetMaxOpenConns(1)`, WAL mode, foreign keys enabled
- **Recursive CTEs:** Used for path extraction (node→root), subtree pruning, best-path finding
- **Auto-pause:** Trees idle >30 min are paused lazily (checked during `list_trees` and `route_problem`, no background goroutine)
- **Embedding BLOBs:** Stored as `[]byte` in SQLite, decoded to `[]float32` for cosine similarity at query time

### Environment variables

| Variable | Default | Purpose |
|---|---|---|
| `TOT_DB_PATH` | `~/.tot-mcp/tot.db` | SQLite database location |
| `TOT_DASHBOARD_PORT` | `4545` | Dashboard HTTP port |
| `TOT_NO_DASHBOARD` | _(unset)_ | Set to `1` to disable dashboard |
| `TOT_EMBED_PROVIDER` | _(auto)_ | Force: `local`, `openai`, `voyage`, `ollama` |
| `TOT_EMBED_MODEL` | _(per-provider)_ | Override model name |
| `TOT_MODEL_CACHE` | `~/.tot-mcp/models/` | Local embedding model cache |
| `OPENAI_API_KEY` | _(unset)_ | Enables OpenAI embeddings |
| `VOYAGE_API_KEY` | _(unset)_ | Enables Voyage AI embeddings |
| `OLLAMA_BASE_URL` | `http://localhost:11434` | Ollama endpoint |

### Dependencies (minimal)
- `modernc.org/sqlite` — Pure Go SQLite (no CGO)
- `github.com/mark3labs/mcp-go` — MCP SDK (tool registration, JSON-RPC 2.0)
- `github.com/google/uuid` — Node/tree IDs
- `github.com/knights-analytics/hugot` — On-device ONNX embeddings

## Code Style

- Go 1.23, pure Go (zero CGO constraint is intentional)
- All packages under `internal/` — not importable externally
- Tool handlers defined inline in `main.go` as closures passed to `s.AddTool()`
- Database schema managed via `CREATE TABLE IF NOT EXISTS` in `db.Init()`
- Error handling: return errors up, `log.Fatal` only in `main()`

## Agent Workflow (TOT_INSTRUCTIONS.md)

When using tot-mcp as an agent tool: `suggest_next` → `route_problem` → `create_tree` or `resume_tree` → `generate_thoughts` → `evaluate_thought` → `search_step` → `mark_solution` → `store_solution`. See TOT_INSTRUCTIONS.md for the full protocol.
