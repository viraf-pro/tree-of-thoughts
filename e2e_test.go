package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/tot-mcp/tot-mcp-go/internal/dashboard"
	"github.com/tot-mcp/tot-mcp-go/internal/events"
	"github.com/tot-mcp/tot-mcp-go/internal/retrieval"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

// TestMain is in cli_test.go — it initializes the DB for all tests in this package.

// TestEventBusIntegration verifies the full pipeline:
// mutation → event published → subscriber receives it.
func TestEventBusIntegration(t *testing.T) {
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	// Create a tree — should produce TreeCreated event
	tr, _, err := tree.CreateTree("e2e test problem", "bfs", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	got := drainEvent(t, ch, events.TreeCreated, 2*time.Second)
	if got.TreeID != tr.ID {
		t.Fatalf("event treeId: got %q, want %q", got.TreeID, tr.ID)
	}
	if got.Payload["problem"] != "e2e test problem" {
		t.Fatalf("event payload: %v", got.Payload)
	}
}

func TestThoughtEventsIntegration(t *testing.T) {
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	tr, root, err := tree.CreateTree("thought events test", "bfs", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}
	drainEvent(t, ch, events.TreeCreated, 2*time.Second) // consume create event

	// Add a thought
	node, err := tree.AddThought(tr.ID, root.ID, "candidate thought 1", map[string]any{"source": "e2e"})
	if err != nil {
		t.Fatalf("AddThought: %v", err)
	}

	got := drainEvent(t, ch, events.ThoughtAdded, 2*time.Second)
	if got.NodeID != node.ID {
		t.Fatalf("ThoughtAdded nodeId: got %q, want %q", got.NodeID, node.ID)
	}
	// Depth is stored as int in the payload (not JSON-decoded, so it stays int)
	depth, ok := got.Payload["depth"].(int)
	if !ok {
		// Could be float64 if JSON round-tripped
		if df, ok2 := got.Payload["depth"].(float64); ok2 {
			depth = int(df)
		} else {
			t.Fatalf("ThoughtAdded depth: unexpected type %T, value %v", got.Payload["depth"], got.Payload["depth"])
		}
	}
	if depth != 1 {
		t.Fatalf("ThoughtAdded depth: got %d, want 1", depth)
	}

	// Evaluate the thought
	_, err = tree.EvaluateThought(tr.ID, node.ID, "sure", nil)
	if err != nil {
		t.Fatalf("EvaluateThought: %v", err)
	}

	got = drainEvent(t, ch, events.ThoughtEvaluated, 2*time.Second)
	if got.Payload["evaluation"] != "sure" {
		t.Fatalf("ThoughtEvaluated evaluation: got %v", got.Payload["evaluation"])
	}
}

func TestBacktrackEventIntegration(t *testing.T) {
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	tr, root, _ := tree.CreateTree("backtrack test", "bfs", 5, 3)
	drainEvent(t, ch, events.TreeCreated, 2*time.Second)

	node, _ := tree.AddThought(tr.ID, root.ID, "branch to prune", nil)
	drainEvent(t, ch, events.ThoughtAdded, 2*time.Second)

	_, err := tree.Backtrack(tr.ID, node.ID)
	if err != nil {
		t.Fatalf("Backtrack: %v", err)
	}

	got := drainEvent(t, ch, events.SubtreePruned, 2*time.Second)
	if got.TreeID != tr.ID {
		t.Fatalf("SubtreePruned treeId: got %q", got.TreeID)
	}
}

func TestMarkSolutionEventIntegration(t *testing.T) {
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	tr, root, _ := tree.CreateTree("solution test", "bfs", 5, 3)
	drainEvent(t, ch, events.TreeCreated, 2*time.Second)

	node, _ := tree.AddThought(tr.ID, root.ID, "the answer", nil)
	drainEvent(t, ch, events.ThoughtAdded, 2*time.Second)

	_, err := tree.MarkTerminal(tr.ID, node.ID)
	if err != nil {
		t.Fatalf("MarkTerminal: %v", err)
	}

	got := drainEvent(t, ch, events.SolutionMarked, 2*time.Second)
	if got.NodeID != node.ID {
		t.Fatalf("SolutionMarked nodeId: got %q", got.NodeID)
	}
}

func TestTreeStatusEventIntegration(t *testing.T) {
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	tr, _, _ := tree.CreateTree("status test", "bfs", 5, 3)
	drainEvent(t, ch, events.TreeCreated, 2*time.Second)

	err := tree.SetStatus(tr.ID, "paused")
	if err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	got := drainEvent(t, ch, events.TreeStatusChanged, 2*time.Second)
	if got.Payload["oldStatus"] != "active" {
		t.Fatalf("oldStatus: got %v", got.Payload["oldStatus"])
	}
	if got.Payload["newStatus"] != "paused" {
		t.Fatalf("newStatus: got %v", got.Payload["newStatus"])
	}
}

func TestTreeLinkEventIntegration(t *testing.T) {
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	t1, _, _ := tree.CreateTree("link source", "bfs", 5, 3)
	drainEvent(t, ch, events.TreeCreated, 2*time.Second)
	t2, _, _ := tree.CreateTree("link target", "bfs", 5, 3)
	drainEvent(t, ch, events.TreeCreated, 2*time.Second)

	_, err := tree.LinkTrees(t1.ID, t2.ID, "informs", "test link")
	if err != nil {
		t.Fatalf("LinkTrees: %v", err)
	}

	got := drainEvent(t, ch, events.TreeLinked, 2*time.Second)
	if got.TreeID != t1.ID {
		t.Fatalf("TreeLinked treeId: got %q", got.TreeID)
	}
	if got.Payload["targetTree"] != t2.ID {
		t.Fatalf("TreeLinked targetTree: got %v", got.Payload["targetTree"])
	}
}

func TestSolutionStoredEventIntegration(t *testing.T) {
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	tr, _, _ := tree.CreateTree("solution store test", "bfs", 5, 3)
	drainEvent(t, ch, events.TreeCreated, 2*time.Second)

	solID, err := retrieval.StoreSolution(tr.ID, "test problem", "test solution", nil, nil, 0.8, []string{"e2e"})
	if err != nil {
		t.Fatalf("StoreSolution: %v", err)
	}

	got := drainEvent(t, ch, events.SolutionStored, 2*time.Second)
	if got.Payload["solutionId"] != solID {
		t.Fatalf("SolutionStored solutionId: got %v, want %s", got.Payload["solutionId"], solID)
	}
}

func TestSSEEndpoint(t *testing.T) {
	// Start dashboard on a random port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	url, err := dashboard.Start(port)
	if err != nil {
		t.Fatalf("dashboard.Start: %v", err)
	}
	defer dashboard.Stop()
	t.Logf("Dashboard at %s", url)

	// Give server a moment to be ready
	time.Sleep(100 * time.Millisecond)

	// Connect to SSE endpoint
	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/events", port))
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("Content-Type: got %q", resp.Header.Get("Content-Type"))
	}

	scanner := bufio.NewScanner(resp.Body)

	// Read initial keepalive
	if !scanner.Scan() {
		t.Fatal("no initial keepalive")
	}
	line := scanner.Text()
	if !strings.HasPrefix(line, ": connected") {
		t.Fatalf("expected keepalive, got %q", line)
	}

	// Trigger an event
	go func() {
		time.Sleep(200 * time.Millisecond)
		tree.CreateTree("sse test", "bfs", 5, 3)
	}()

	// Read SSE frames — look for the tree.created event
	deadline := time.After(5 * time.Second)
	found := false
	for !found {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for SSE event")
		default:
		}
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if strings.HasPrefix(line, "event: tree.created") {
			found = true
		}
	}

	if !found {
		t.Fatal("did not receive tree.created SSE event")
	}

	// Read the data line
	if scanner.Scan() {
		data := scanner.Text()
		if !strings.HasPrefix(data, "data: ") {
			t.Fatalf("expected data line, got %q", data)
		}
		payload := data[6:] // strip "data: "
		var evt events.Event
		if err := json.Unmarshal([]byte(payload), &evt); err != nil {
			t.Fatalf("unmarshal SSE data: %v", err)
		}
		if evt.Type != events.TreeCreated {
			t.Fatalf("SSE event type: got %q", evt.Type)
		}
		if evt.Payload["problem"] != "sse test" {
			t.Fatalf("SSE event payload: %v", evt.Payload)
		}
	}
}

func TestDashboardRESTEndpoints(t *testing.T) {
	// Ensure we have at least one tree from prior tests
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	url, err := dashboard.Start(port)
	if err != nil {
		t.Fatalf("dashboard.Start: %v", err)
	}
	defer dashboard.Stop()

	time.Sleep(100 * time.Millisecond)

	// Test /api/trees
	resp, err := http.Get(url + "/api/trees")
	if err != nil {
		t.Fatalf("GET /api/trees: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("/api/trees status: %d", resp.StatusCode)
	}

	var trees []map[string]any
	json.NewDecoder(resp.Body).Decode(&trees)
	if len(trees) == 0 {
		t.Fatal("/api/trees returned empty list")
	}
	t.Logf("/api/trees returned %d trees", len(trees))

	// Test /api/tree/{id}
	treeID := trees[0]["id"].(string)
	resp2, err := http.Get(fmt.Sprintf("%s/api/tree/%s", url, treeID))
	if err != nil {
		t.Fatalf("GET /api/tree/{id}: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != 200 {
		t.Fatalf("/api/tree/{id} status: %d", resp2.StatusCode)
	}
}

func TestCompactApplyEventIntegration(t *testing.T) {
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	// Create a tree and store a solution
	tr, _, _ := tree.CreateTree("compact test", "bfs", 5, 3)
	drainEvent(t, ch, events.TreeCreated, 2*time.Second)

	solID, err := retrieval.StoreSolution(tr.ID, "compact problem", "detailed solution with lots of thoughts", nil, nil, 0.7, []string{"compact-test"})
	if err != nil {
		t.Fatalf("StoreSolution: %v", err)
	}
	drainEvent(t, ch, events.SolutionStored, 2*time.Second)

	// Compact the solution
	err = retrieval.CompactApply(solID, "short summary")
	if err != nil {
		t.Fatalf("CompactApply: %v", err)
	}

	got := drainEvent(t, ch, events.SolutionCompacted, 2*time.Second)
	if got.Payload["solutionId"] != solID {
		t.Fatalf("SolutionCompacted solutionId: got %v, want %s", got.Payload["solutionId"], solID)
	}
}

func TestMultipleEventsInSequence(t *testing.T) {
	// Verify a full tree workflow produces events in order
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	// Create → AddThought → Evaluate → MarkTerminal
	tr, root, _ := tree.CreateTree("sequence test", "bfs", 5, 3)
	drainEvent(t, ch, events.TreeCreated, 2*time.Second)

	node, _ := tree.AddThought(tr.ID, root.ID, "the answer", nil)
	drainEvent(t, ch, events.ThoughtAdded, 2*time.Second)

	tree.EvaluateThought(tr.ID, node.ID, "sure", nil)
	drainEvent(t, ch, events.ThoughtEvaluated, 2*time.Second)

	tree.MarkTerminal(tr.ID, node.ID)
	drainEvent(t, ch, events.SolutionMarked, 2*time.Second)

	tree.SetStatus(tr.ID, "solved")
	got := drainEvent(t, ch, events.TreeStatusChanged, 2*time.Second)
	if got.Payload["newStatus"] != "solved" {
		t.Fatalf("expected solved, got %v", got.Payload["newStatus"])
	}
}

func TestSolutionLinkedEventIntegration(t *testing.T) {
	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	tr, _, _ := tree.CreateTree("link sol test", "bfs", 5, 3)
	drainEvent(t, ch, events.TreeCreated, 2*time.Second)

	sol1, _ := retrieval.StoreSolution(tr.ID, "problem A", "solution A", nil, nil, 0.8, []string{"a"})
	drainEvent(t, ch, events.SolutionStored, 2*time.Second)

	sol2, _ := retrieval.StoreSolution(tr.ID, "problem B", "solution B", nil, nil, 0.7, []string{"b"})
	drainEvent(t, ch, events.SolutionStored, 2*time.Second)

	_, err := retrieval.LinkSolutions(sol1, sol2, "related", "test link")
	if err != nil {
		t.Fatalf("LinkSolutions: %v", err)
	}

	got := drainEvent(t, ch, events.SolutionLinked, 2*time.Second)
	if got.Payload["sourceId"] != sol1 {
		t.Fatalf("SolutionLinked sourceId: got %v", got.Payload["sourceId"])
	}
	if got.Payload["targetId"] != sol2 {
		t.Fatalf("SolutionLinked targetId: got %v", got.Payload["targetId"])
	}
}

// drainEvent reads from the channel until it finds an event of the expected type,
// discarding others. Fails if the deadline is reached.
func drainEvent(t *testing.T, ch <-chan events.Event, wantType string, timeout time.Duration) events.Event {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt := <-ch:
			if evt.Type == wantType {
				return evt
			}
			// Discard other events (e.g., auto-link events from retrieval)
		case <-deadline:
			t.Fatalf("timed out waiting for event %q", wantType)
			return events.Event{}
		}
	}
}
