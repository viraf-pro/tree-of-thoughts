# MCP Subscriptions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add real-time subscriptions across all four domains (tree reasoning, lifecycle, experiments, knowledge) via an internal event bus, MCP resource notifications, and dashboard SSE.

**Architecture:** A central in-process event bus (`internal/events`) fans out typed events to three consumers: an MCP notification bridge (pushes `notifications/resources/updated` to subscribed clients), a dashboard SSE endpoint (replaces 10s polling), and optionally the audit logger. Mutation functions in `tree`, `experiment`, and `retrieval` packages publish events after successful DB writes. MCP resources are exposed via URI templates so clients can subscribe to specific trees, experiments, or solutions.

**Tech Stack:** Go 1.24, mcp-go v0.32.0 (`SendNotificationToAllClients`, `WithResourceCapabilities`), SSE (plain HTTP, no WebSocket), SQLite (read-only for resource handlers).

---

## Task 1: Event Bus Core

**Files:**
- Create: `internal/events/types.go`
- Create: `internal/events/bus.go`

**Step 1: Create event types**

Create `internal/events/types.go`:

```go
package events

import "time"

// Event type constants.
const (
	TreeCreated       = "tree.created"
	ThoughtAdded      = "thought.added"
	ThoughtEvaluated  = "thought.evaluated"
	SubtreePruned     = "subtree.pruned"
	SolutionMarked    = "solution.marked"
	TreeStatusChanged = "tree.status_changed"
	TreeAutoPaused    = "tree.auto_paused"
	TreeLinked        = "tree.linked"
	ExperimentPrepared  = "experiment.prepared"
	ExperimentCompleted = "experiment.completed"
	ExperimentFailed    = "experiment.failed"
	SolutionStored    = "solution.stored"
	SolutionCompacted = "solution.compacted"
	SolutionLinked    = "solution.linked"
	URLIngested       = "url.ingested"
)

// Event is the envelope published on the bus.
type Event struct {
	Type      string         `json:"type"`
	TreeID    string         `json:"treeId,omitempty"`
	NodeID    string         `json:"nodeId,omitempty"`
	Timestamp time.Time      `json:"timestamp"`
	Payload   map[string]any `json:"payload,omitempty"`
}
```

**Step 2: Create the bus**

Create `internal/events/bus.go`:

```go
package events

import (
	"log"
	"sync"
)

const bufSize = 64

// Bus is a typed pub/sub event bus. Safe for concurrent use.
type Bus struct {
	mu   sync.RWMutex
	subs map[int]chan Event
	next int
}

var (
	global     *Bus
	globalOnce sync.Once
)

// Get returns the global event bus singleton.
func Get() *Bus {
	globalOnce.Do(func() {
		global = &Bus{subs: make(map[int]chan Event)}
	})
	return global
}

// Subscribe returns a buffered channel of events and an ID for unsubscribing.
func (b *Bus) Subscribe() (int, <-chan Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	id := b.next
	b.next++
	ch := make(chan Event, bufSize)
	b.subs[id] = ch
	return id, ch
}

// Unsubscribe removes a subscription and closes its channel.
func (b *Bus) Unsubscribe(id int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if ch, ok := b.subs[id]; ok {
		delete(b.subs, id)
		close(ch)
	}
}

// Publish fans out an event to all subscribers.
// Non-blocking: if a subscriber's buffer is full, the event is dropped
// for that subscriber (logged once).
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, ch := range b.subs {
		select {
		case ch <- e:
		default:
			log.Printf("events: dropped %s for slow subscriber", e.Type)
		}
	}
}
```

**Step 3: Verify it compiles**

Run: `cd /Users/viraf/Git/tot-mcp && go vet ./internal/events/...`
Expected: no output (clean)

**Step 4: Commit**

```bash
git add internal/events/types.go internal/events/bus.go
git commit -m "feat: add internal event bus for subscriptions"
```

---

## Task 2: Instrument Tree Mutations

**Files:**
- Modify: `internal/tree/tree.go`

Each mutation function gets an `events.Publish(...)` call after the successful DB write, right before the return. Import `events` and `time` at the top of the file.

**Step 1: Add import**

Add to the import block in `internal/tree/tree.go:3-13`:

```go
"github.com/tot-mcp/tot-mcp-go/internal/events"
```

(`time` is already imported.)

**Step 2: Instrument CreateTree**

At `internal/tree/tree.go:99`, before `return tree, node, nil`:

```go
events.Get().Publish(events.Event{
	Type: events.TreeCreated, TreeID: treeID,
	Timestamp: time.Now(),
	Payload: map[string]any{"problem": problem, "strategy": strategy},
})
```

**Step 3: Instrument AddThought**

At `internal/tree/tree.go:231`, before `return GetNode(treeID, nodeID)`:

```go
events.Get().Publish(events.Event{
	Type: events.ThoughtAdded, TreeID: treeID, NodeID: nodeID,
	Timestamp: time.Now(),
	Payload: map[string]any{"parentId": parentID, "depth": newDepth},
})
```

**Step 4: Instrument EvaluateThought**

At `internal/tree/tree.go:266`, before `return GetNode(treeID, nodeID)`. The exact line is right after the tx.Commit succeeds. Add:

```go
events.Get().Publish(events.Event{
	Type: events.ThoughtEvaluated, TreeID: treeID, NodeID: nodeID,
	Timestamp: time.Now(),
	Payload: map[string]any{"evaluation": evaluation, "score": score},
})
```

**Step 5: Instrument MarkTerminal**

At `internal/tree/tree.go:301`, before `return GetNode(treeID, nodeID)`:

```go
events.Get().Publish(events.Event{
	Type: events.SolutionMarked, TreeID: treeID, NodeID: nodeID,
	Timestamp: time.Now(),
})
```

**Step 6: Instrument Backtrack**

At `internal/tree/tree.go:375`, before the `if node.ParentID != nil` block:

```go
events.Get().Publish(events.Event{
	Type: events.SubtreePruned, TreeID: treeID, NodeID: nodeID,
	Timestamp: time.Now(),
	Payload: map[string]any{"prunedCount": len(ids)},
})
```

**Step 7: Instrument SetStatus**

At `internal/tree/tree.go:562-563`, replace:

```go
_, err = d.Exec(`UPDATE trees SET status=?, updated_at=? WHERE id=?`, newStatus, now(), treeID)
return err
```

with:

```go
_, err = d.Exec(`UPDATE trees SET status=?, updated_at=? WHERE id=?`, newStatus, now(), treeID)
if err == nil {
	events.Get().Publish(events.Event{
		Type: events.TreeStatusChanged, TreeID: treeID,
		Timestamp: time.Now(),
		Payload: map[string]any{"oldStatus": tree.Status, "newStatus": newStatus},
	})
}
return err
```

**Step 8: Instrument AutoPause**

At `internal/tree/tree.go:584`, after `n, _ := res.RowsAffected()`, before `return int(n)`:

```go
if n > 0 {
	events.Get().Publish(events.Event{
		Type: events.TreeAutoPaused,
		Timestamp: time.Now(),
		Payload: map[string]any{"count": int(n), "staleMinutes": staleMinutes},
	})
}
```

**Step 9: Instrument LinkTrees**

At `internal/tree/tree.go:773`, before the `return &TreeLink{...}, nil`:

```go
events.Get().Publish(events.Event{
	Type: events.TreeLinked, TreeID: sourceTree,
	Timestamp: time.Now(),
	Payload: map[string]any{"targetTree": targetTree, "linkType": linkType},
})
```

**Step 10: Verify it compiles**

Run: `go vet ./internal/tree/...`
Expected: clean

**Step 11: Commit**

```bash
git add internal/tree/tree.go
git commit -m "feat: publish events from tree mutation functions"
```

---

## Task 3: Instrument Experiment Mutations

**Files:**
- Modify: `internal/experiment/experiment.go`

**Step 1: Add import**

Add to the import block in `internal/experiment/experiment.go:3-18`:

```go
"github.com/tot-mcp/tot-mcp-go/internal/events"
```

(`time` is already imported.)

**Step 2: Instrument Prepare**

At `internal/experiment/experiment.go:114`, before `return &PrepareResult{...}, nil`:

```go
events.Get().Publish(events.Event{
	Type: events.ExperimentPrepared, TreeID: treeID,
	Timestamp: time.Now(),
	Payload: map[string]any{"commitHash": hash, "branch": branch},
})
```

**Step 3: Instrument Execute**

At `internal/experiment/experiment.go:188-190`, after `logResult(treeID, nodeID, &result)`, before `return &result, nil`:

```go
evtType := events.ExperimentCompleted
if result.Status == "crashed" || result.Status == "timeout" {
	evtType = events.ExperimentFailed
}
payload := map[string]any{"status": result.Status, "kept": result.Kept, "durationSeconds": result.DurationSecs}
if result.Metric != nil {
	payload["metric"] = *result.Metric
}
events.Get().Publish(events.Event{
	Type: evtType, TreeID: treeID, NodeID: nodeID,
	Timestamp: time.Now(),
	Payload: payload,
})
```

**Step 4: Verify it compiles**

Run: `go vet ./internal/experiment/...`
Expected: clean

**Step 5: Commit**

```bash
git add internal/experiment/experiment.go
git commit -m "feat: publish events from experiment mutations"
```

---

## Task 4: Instrument Retrieval Mutations

**Files:**
- Modify: `internal/retrieval/retrieval.go`

**Step 1: Add import**

Add to the import block in `internal/retrieval/retrieval.go`:

```go
"github.com/tot-mcp/tot-mcp-go/internal/events"
```

(`time` is already imported.)

**Step 2: Instrument StoreSolution**

At `internal/retrieval/retrieval.go:62-66`, inside the `if err == nil` block, after the existing `LogKnowledgeEvent` call on line 64:

```go
events.Get().Publish(events.Event{
	Type: events.SolutionStored, TreeID: treeID,
	Timestamp: time.Now(),
	Payload: map[string]any{"solutionId": id, "tags": tags},
})
```

**Step 3: Instrument CompactApply**

At `internal/retrieval/retrieval.go:380`, replace `return tx.Commit()` with:

```go
if err := tx.Commit(); err != nil {
	return err
}
events.Get().Publish(events.Event{
	Type: events.SolutionCompacted,
	Timestamp: time.Now(),
	Payload: map[string]any{"solutionId": solutionID},
})
return nil
```

**Step 4: Instrument LinkSolutionsWithConfidence**

At `internal/retrieval/retrieval.go:475`, after the `LogKnowledgeEvent` call, before the final return:

```go
events.Get().Publish(events.Event{
	Type: events.SolutionLinked,
	Timestamp: time.Now(),
	Payload: map[string]any{"sourceId": sourceID, "targetId": targetID, "linkType": linkType, "confidence": confidence},
})
```

**Step 5: Verify it compiles**

Run: `go vet ./internal/retrieval/...`
Expected: clean

**Step 6: Commit**

```bash
git add internal/retrieval/retrieval.go
git commit -m "feat: publish events from retrieval mutations"
```

---

## Task 5: MCP Resources

**Files:**
- Create: `internal/resources/resources.go`
- Modify: `main.go`

This task exposes trees, experiments, and solutions as MCP resources that clients can read and subscribe to.

**Step 1: Create resource handlers**

Create `internal/resources/resources.go`:

```go
package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/experiment"
	"github.com/tot-mcp/tot-mcp-go/internal/retrieval"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

// Register adds all resource templates to the MCP server.
func Register(s *server.MCPServer) {
	// tot://trees — list all trees
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://trees", "List all reasoning trees"),
		readTreeList,
	)

	// tot://tree/{id} — full tree state
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://tree/{id}", "Full tree state with nodes, stats, and best path"),
		readTree,
	)

	// tot://tree/{id}/frontier — frontier nodes
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://tree/{id}/frontier", "Frontier nodes available for expansion"),
		readFrontier,
	)

	// tot://tree/{id}/experiments — experiment history
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://tree/{id}/experiments", "Experiment history and metrics"),
		readExperiments,
	)

	// tot://tree/{id}/status — lifecycle status
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://tree/{id}/status", "Tree lifecycle status"),
		readStatus,
	)

	// tot://solutions — retrieval store overview
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://solutions", "Retrieval store overview"),
		readSolutions,
	)

	// tot://solution/{id} — single solution
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://solution/{id}", "Single solution detail"),
		readSolution,
	)
}

func j(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

func readTreeList(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	trees, err := tree.ListTrees()
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, len(trees))
	for i, t := range trees {
		items[i] = map[string]any{
			"id": t.ID, "problem": t.Problem, "status": t.Status,
			"strategy": t.SearchStrategy, "nodeCount": tree.NodeCount(t.ID),
		}
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents(req.Params.URI, j(items), "application/json"),
	}, nil
}

func readTree(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	treeID := extractParam(req.Params.URI, "tot://tree/", "/")
	if treeID == "" {
		return nil, fmt.Errorf("missing tree ID")
	}
	t, err := tree.GetTree(treeID)
	if err != nil {
		return nil, err
	}
	summary, _ := tree.Summary(treeID)
	bestPath, _ := tree.GetBestPath(treeID)
	data := map[string]any{"tree": t, "summary": summary, "bestPath": bestPath}
	return []mcp.ResourceContents{
		mcp.TextResourceContents(req.Params.URI, j(data), "application/json"),
	}, nil
}

func readFrontier(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	treeID := extractMiddleParam(req.Params.URI, "tot://tree/", "/frontier")
	nodes, err := tree.GetFrontier(treeID)
	if err != nil {
		return nil, err
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents(req.Params.URI, j(nodes), "application/json"),
	}, nil
}

func readExperiments(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	treeID := extractMiddleParam(req.Params.URI, "tot://tree/", "/experiments")
	history := experiment.History(treeID)
	return []mcp.ResourceContents{
		mcp.TextResourceContents(req.Params.URI, j(history), "application/json"),
	}, nil
}

func readStatus(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	treeID := extractMiddleParam(req.Params.URI, "tot://tree/", "/status")
	t, err := tree.GetTree(treeID)
	if err != nil {
		return nil, err
	}
	data := map[string]any{"treeId": treeID, "status": t.Status, "updatedAt": t.UpdatedAt}
	return []mcp.ResourceContents{
		mcp.TextResourceContents(req.Params.URI, j(data), "application/json"),
	}, nil
}

func readSolutions(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	stats := retrieval.Stats()
	return []mcp.ResourceContents{
		mcp.TextResourceContents(req.Params.URI, j(stats), "application/json"),
	}, nil
}

func readSolution(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	solID := extractParam(req.Params.URI, "tot://solution/", "")
	d := db.Get()
	var prob, sol, tags, created string
	var score float64
	err := d.QueryRow(`SELECT problem, solution, tags, score, created_at FROM solutions WHERE id=?`, solID).
		Scan(&prob, &sol, &tags, &score, &created)
	if err != nil {
		return nil, fmt.Errorf("solution not found")
	}
	data := map[string]any{"id": solID, "problem": prob, "solution": sol, "tags": tags, "score": score, "createdAt": created}
	return []mcp.ResourceContents{
		mcp.TextResourceContents(req.Params.URI, j(data), "application/json"),
	}, nil
}

// extractParam extracts an ID from a URI like "tot://tree/abc-123" or "tot://tree/abc-123/sub".
func extractParam(uri, prefix, suffix string) string {
	s := uri
	if len(prefix) > 0 {
		if len(s) <= len(prefix) {
			return ""
		}
		s = s[len(prefix):]
	}
	if suffix != "" {
		for i := 0; i < len(s); i++ {
			if s[i] == '/' {
				return s[:i]
			}
		}
	}
	return s
}

// extractMiddleParam extracts an ID between a prefix and suffix: "tot://tree/{id}/frontier".
func extractMiddleParam(uri, prefix, suffix string) string {
	if len(uri) <= len(prefix)+len(suffix) {
		return ""
	}
	return uri[len(prefix) : len(uri)-len(suffix)]
}
```

**Step 2: Register resources in main.go**

Add import in `main.go`:

```go
"github.com/tot-mcp/tot-mcp-go/internal/resources"
```

Change server creation at `main.go:60` from:

```go
s := server.NewMCPServer("tot-mcp-server", version, server.WithToolCapabilities(false))
```

to:

```go
s := server.NewMCPServer("tot-mcp-server", version,
	server.WithToolCapabilities(false),
	server.WithResourceCapabilities(true, true),
)
```

Add after the existing `registerKnowledgeTools(s)` call (around line 66):

```go
resources.Register(s)
```

**Step 3: Verify it compiles**

Run: `go vet ./...`
Expected: clean

**Step 4: Commit**

```bash
git add internal/resources/resources.go main.go
git commit -m "feat: expose trees, experiments, solutions as MCP resources"
```

---

## Task 6: MCP Notification Bridge

**Files:**
- Create: `internal/events/mcpbridge.go`
- Modify: `main.go`

This goroutine subscribes to the event bus and sends MCP `notifications/resources/updated` for affected resource URIs.

**Step 1: Create the bridge**

Create `internal/events/mcpbridge.go`:

```go
package events

import (
	"fmt"
	"log"

	"github.com/mark3labs/mcp-go/server"
)

// StartMCPBridge subscribes to the global bus and forwards events
// as MCP resource-updated notifications. Non-blocking — runs in a goroutine.
func StartMCPBridge(s *server.MCPServer) {
	id, ch := Get().Subscribe()
	go func() {
		defer Get().Unsubscribe(id)
		for evt := range ch {
			for _, uri := range affectedURIs(evt) {
				s.SendNotificationToAllClients(
					"notifications/resources/updated",
					map[string]any{"uri": uri},
				)
			}
			log.Printf("event: %s tree=%s node=%s", evt.Type, evt.TreeID, evt.NodeID)
		}
	}()
}

// affectedURIs maps an event to the resource URIs that changed.
func affectedURIs(e Event) []string {
	var uris []string

	// Always notify the tree list if a tree event
	switch e.Type {
	case TreeCreated, TreeStatusChanged, TreeAutoPaused:
		uris = append(uris, "tot://trees")
	}

	// Tree-specific resources
	if e.TreeID != "" {
		uris = append(uris, fmt.Sprintf("tot://tree/%s", e.TreeID))

		switch e.Type {
		case ThoughtAdded, SubtreePruned:
			uris = append(uris, fmt.Sprintf("tot://tree/%s/frontier", e.TreeID))
		case TreeStatusChanged, TreeAutoPaused:
			uris = append(uris, fmt.Sprintf("tot://tree/%s/status", e.TreeID))
		case ExperimentPrepared, ExperimentCompleted, ExperimentFailed:
			uris = append(uris, fmt.Sprintf("tot://tree/%s/experiments", e.TreeID))
		}
	}

	// Solution resources
	switch e.Type {
	case SolutionStored, SolutionCompacted, SolutionLinked, URLIngested:
		uris = append(uris, "tot://solutions")
		if sid, ok := e.Payload["solutionId"].(string); ok {
			uris = append(uris, fmt.Sprintf("tot://solution/%s", sid))
		}
	}

	// Tree links affect both trees
	if e.Type == TreeLinked {
		if tgt, ok := e.Payload["targetTree"].(string); ok {
			uris = append(uris, fmt.Sprintf("tot://tree/%s", tgt))
		}
	}

	return uris
}
```

**Step 2: Wire bridge in main.go**

In `main.go`, after `resources.Register(s)` and before `server.ServeStdio(s)`:

```go
events.StartMCPBridge(s)
```

Add import if not already present:

```go
"github.com/tot-mcp/tot-mcp-go/internal/events"
```

**Step 3: Verify it compiles**

Run: `go vet ./...`
Expected: clean

**Step 4: Commit**

```bash
git add internal/events/mcpbridge.go main.go
git commit -m "feat: MCP notification bridge forwards events to subscribed clients"
```

---

## Task 7: Instrument ingest_url in main.go

**Files:**
- Modify: `main.go`

The `ingest_url` tool handler in `main.go` needs to publish `URLIngested`. Unlike the other mutations, this one lives in the tool handler, not in a library function.

**Step 1: Add event publish**

At `main.go:895`, after the `db.LogAudit(...)` call and before `return textResult(...)`:

```go
events.Get().Publish(events.Event{
	Type: events.URLIngested,
	Timestamp: time.Now(),
	Payload: map[string]any{"solutionId": id, "url": rawURL},
})
```

Add `"time"` to imports in main.go if not already present. (It is not currently imported in main.go — check and add if needed.)

**Step 2: Verify it compiles**

Run: `go vet ./...`
Expected: clean

**Step 3: Commit**

```bash
git add main.go
git commit -m "feat: publish URLIngested event from ingest_url handler"
```

---

## Task 8: Dashboard SSE Endpoint

**Files:**
- Modify: `internal/dashboard/server.go`

**Step 1: Add SSE handler**

Add import to `internal/dashboard/server.go`:

```go
"github.com/tot-mcp/tot-mcp-go/internal/events"
```

Register the SSE route in the `Start` function, after the existing `mux.HandleFunc` calls (around line 26):

```go
mux.HandleFunc("/api/events", handleSSE)
```

Add the handler function at the bottom of the file, before the helpers section:

```go
func handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	// Send initial keepalive
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
			flusher.Flush()
		}
	}
}
```

**Step 2: Verify it compiles**

Run: `go vet ./internal/dashboard/...`
Expected: clean

**Step 3: Commit**

```bash
git add internal/dashboard/server.go
git commit -m "feat: add SSE endpoint /api/events to dashboard"
```

---

## Task 9: Dashboard Frontend — Replace Polling with EventSource

**Files:**
- Modify: `internal/dashboard/html.go`

**Step 1: Replace setInterval with EventSource**

In `internal/dashboard/html.go:587`, replace:

```js
setInterval(render, 10000);
```

with:

```js
// Live updates via SSE, fallback to polling
if (typeof EventSource !== 'undefined') {
  const es = new EventSource('/api/events');
  es.addEventListener('tree.created', () => render());
  es.addEventListener('thought.added', () => { if (location.hash.startsWith('#tree/')) render(); });
  es.addEventListener('thought.evaluated', () => { if (location.hash.startsWith('#tree/')) render(); });
  es.addEventListener('subtree.pruned', () => { if (location.hash.startsWith('#tree/')) render(); });
  es.addEventListener('solution.marked', () => render());
  es.addEventListener('tree.status_changed', () => render());
  es.addEventListener('tree.auto_paused', () => render());
  es.addEventListener('experiment.completed', () => { if (location.hash.startsWith('#tree/')) render(); });
  es.addEventListener('experiment.failed', () => { if (location.hash.startsWith('#tree/')) render(); });
  es.addEventListener('solution.stored', () => { if (location.hash.startsWith('#tree/')) render(); });
  es.addEventListener('solution.compacted', () => render());
  es.onerror = () => {
    es.close();
    setInterval(render, 10000);
  };
} else {
  setInterval(render, 10000);
}
```

**Step 2: Verify it compiles**

Run: `go vet ./internal/dashboard/...`
Expected: clean

**Step 3: Commit**

```bash
git add internal/dashboard/html.go
git commit -m "feat: dashboard uses SSE for live updates, polling as fallback"
```

---

## Task 10: Full Build & Manual Test

**Files:** None (verification only)

**Step 1: Full build**

Run: `make build`
Expected: `./tot-mcp` binary produced with no errors

**Step 2: Typecheck and vet**

Run: `go vet ./...`
Expected: clean

**Step 3: Manual smoke test — SSE**

Run: `TOT_NO_DASHBOARD= ./tot-mcp &` in one terminal, then:

```bash
curl -N http://127.0.0.1:4545/api/events
```

Expected: `: connected` initial keepalive, then SSE frames as mutations happen.

**Step 4: Manual smoke test — MCP resources**

Using MCP Inspector or a test client, call:
- `resources/list` — should return the 7 resource templates
- `resources/read` with URI `tot://trees` — should return JSON tree list
- `resources/subscribe` with URI `tot://tree/{some-id}` — then trigger a mutation, verify `notifications/resources/updated` arrives

**Step 5: Commit (if any fixups)**

```bash
git add -A
git commit -m "fix: address issues found during manual testing"
```

**Step 6: Final commit**

```bash
git add -A
git commit -m "feat: complete MCP subscriptions — event bus, resources, SSE dashboard"
```

---

## Dependency Graph

```
Task 1 (event bus)
  ├── Task 2 (tree mutations)      ─┐
  ├── Task 3 (experiment mutations) ├── Task 6 (MCP bridge) → Task 10
  ├── Task 4 (retrieval mutations)  │
  ├── Task 5 (MCP resources)       ─┘
  ├── Task 7 (ingest_url event)
  ├── Task 8 (dashboard SSE)       ─── Task 9 (frontend EventSource) → Task 10
```

Tasks 2, 3, 4, 5, 7, 8 are all independent of each other (only depend on Task 1).
Task 6 depends on Tasks 2-5 and 7 (needs events being published + resources registered).
Task 9 depends on Task 8.
Task 10 depends on everything.
