# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

tot-mcp is a Tree of Thoughts (ToT) MCP server written in Go. It provides structured multi-path reasoning, persistent storage (SQLite), hybrid retrieval (vector + FTS5), autonomous experiment execution, and a live web dashboard. Single binary, pure Go, zero CGO.

## Build & Run

```bash
make build                    # Build binary (./tot-mcp)
make all                      # Cross-compile all platforms → dist/
make clean                    # Remove artifacts
go build -o tot-mcp .         # Direct build
go vet ./...                  # Lint/check
```

There are no tests in this codebase currently. No package.json — this is a pure Go project (go 1.23).

**Running:**
- `./tot-mcp` — MCP stdio server + HTTP dashboard (port 4545)
- `./tot-mcp <command>` — CLI mode (list, suggest, show, route, create, ready, audit, stats, compact, help)

## Architecture

The binary operates in two modes based on `os.Args`: CLI mode (with arguments) or MCP server mode (no arguments, stdio JSON-RPC).

### Root-level files
- **main.go** — MCP server setup, tool registration (28 tools across 4 categories), embedding provider init, dashboard startup
- **cli.go** — Lightweight CLI dispatcher (~1-2k tokens vs 10-50k for full MCP schemas)

### Internal packages (`internal/`)

| Package | Purpose |
|---|---|
| `db` | SQLite init (WAL mode, singleton via sync.Once), schema DDL, audit logging |
| `tree` | Core tree CRUD, search strategies (BFS/DFS/Beam), frontier management, pruning via recursive CTEs, routing (Jaccard similarity), cross-tree links, lifecycle (active/paused/abandoned/solved) |
| `retrieval` | Hybrid search: vector cosine similarity + FTS5 keyword, 20% boost for dual-match, solution compaction (memory decay at 30+ days) |
| `embeddings` | Pluggable provider interface (`Embed(text) → []float32`). Auto-selects: OpenAI → Voyage → Ollama → local Hugot (pure Go ONNX, all-MiniLM-L6-v2) → Noop |
| `experiment` | Two-phase autonomous runner: prepare (apply patch, git commit) → execute (run command, parse metric via regex, auto-evaluate, rollback on failure) |
| `dashboard` | HTTP API server + embedded SPA (SVG tree visualization, Chart.js metrics). Endpoints at `/api/trees`, `/api/tree/:id`, etc. |

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
