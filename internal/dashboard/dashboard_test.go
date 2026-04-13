package dashboard

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/events"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

func TestMain(m *testing.M) {
	tmp := filepath.Join(os.TempDir(), "tot-mcp-dashboard-test.db")
	os.Remove(tmp)
	if _, err := db.Init(tmp); err != nil {
		panic(err)
	}
	// Create fixture data
	tree.CreateTree("dashboard test problem", "bfs", 5, 3)

	code := m.Run()
	os.Remove(tmp)
	os.Exit(code)
}

func TestStartStop(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	url, err := Start(port)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer Stop()

	if url == "" {
		t.Fatal("Start returned empty URL")
	}

	// Verify server is responsive
	resp, err := http.Get(url + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET / status: %d", resp.StatusCode)
	}
}

func TestStartFallbackPort(t *testing.T) {
	// Occupy a port, then try to start on it — should fallback
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	// Keep ln open to block the port

	url, err := Start(port)
	ln.Close()
	if err != nil {
		t.Fatalf("Start with occupied port: %v", err)
	}
	defer Stop()

	if url == "" {
		t.Fatal("Start returned empty URL on fallback")
	}
}

func TestHandleIndex(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handleIndex(w, req)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("Content-Type: %q", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "ToT Dashboard") {
		t.Fatal("index HTML missing title")
	}
}

func TestHandleIndexNotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	w := httptest.NewRecorder()
	handleIndex(w, req)

	resp := w.Result()
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestHandleTrees(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/trees", nil)
	w := httptest.NewRecorder()
	handleTrees(w, req)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	var trees []map[string]any
	json.NewDecoder(resp.Body).Decode(&trees)
	if len(trees) == 0 {
		t.Fatal("expected at least 1 tree")
	}

	// Verify structure
	tr := trees[0]
	for _, key := range []string{"id", "problem", "strategy", "status"} {
		if _, ok := tr[key]; !ok {
			t.Fatalf("missing key %q in tree response", key)
		}
	}
}

func TestHandleTreeDetail(t *testing.T) {
	// Get a tree ID first
	trees, _ := tree.ListTrees()
	if len(trees) == 0 {
		t.Fatal("no trees")
	}
	treeID := trees[0].ID

	req := httptest.NewRequest("GET", "/api/tree/"+treeID, nil)
	w := httptest.NewRecorder()
	handleTreeDetail(w, req)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	var detail map[string]any
	json.NewDecoder(resp.Body).Decode(&detail)
	if _, ok := detail["tree"]; !ok {
		t.Fatal("missing 'tree' key")
	}
	if _, ok := detail["nodes"]; !ok {
		t.Fatal("missing 'nodes' key")
	}
	if _, ok := detail["stats"]; !ok {
		t.Fatal("missing 'stats' key")
	}
}

func TestHandleTreeDetailNotFound(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/tree/nonexistent", nil)
	w := httptest.NewRecorder()
	handleTreeDetail(w, req)

	resp := w.Result()
	if resp.StatusCode != 400 {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandleExperiments(t *testing.T) {
	trees, _ := tree.ListTrees()
	treeID := trees[0].ID

	req := httptest.NewRequest("GET", "/api/experiments/"+treeID, nil)
	w := httptest.NewRecorder()
	handleExperiments(w, req)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if _, ok := body["stats"]; !ok {
		t.Fatal("missing 'stats' key")
	}
}

func TestHandleRetrieval(t *testing.T) {
	trees, _ := tree.ListTrees()
	treeID := trees[0].ID

	req := httptest.NewRequest("GET", "/api/retrieval/"+treeID, nil)
	w := httptest.NewRecorder()
	handleRetrieval(w, req)

	resp := w.Result()
	if resp.StatusCode != 200 {
		t.Fatalf("status: %d", resp.StatusCode)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if _, ok := body["stats"]; !ok {
		t.Fatal("missing 'stats' key")
	}
}

func TestHandleSSE(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/events", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handleSSE(w, req)
		close(done)
	}()

	// Give SSE handler time to set up
	time.Sleep(50 * time.Millisecond)

	// Publish an event
	events.Get().Publish(events.Event{
		Type:      events.TreeCreated,
		TreeID:    "sse-test",
		Timestamp: time.Now(),
		Payload:   map[string]any{"problem": "sse test"},
	})

	// Give event time to be written
	time.Sleep(50 * time.Millisecond)

	// Cancel context to stop SSE handler
	cancel()
	<-done

	resp := w.Result()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type: %q", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	output := string(body)

	if !strings.Contains(output, ": connected") {
		t.Fatal("missing keepalive")
	}
	if !strings.Contains(output, "event: tree.created") {
		t.Fatalf("missing tree.created event in output: %s", output)
	}
	if !strings.Contains(output, `"treeId":"sse-test"`) {
		t.Fatalf("missing treeId in SSE data: %s", output)
	}
}

func TestHandleSSEMultipleEvents(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/events", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handleSSE(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Publish multiple events
	bus := events.Get()
	bus.Publish(events.Event{Type: events.ThoughtAdded, TreeID: "t1", Timestamp: time.Now()})
	bus.Publish(events.Event{Type: events.ThoughtEvaluated, TreeID: "t1", Timestamp: time.Now()})
	bus.Publish(events.Event{Type: events.SolutionMarked, TreeID: "t1", Timestamp: time.Now()})

	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body, _ := io.ReadAll(w.Result().Body)
	output := string(body)

	for _, evt := range []string{"thought.added", "thought.evaluated", "solution.marked"} {
		if !strings.Contains(output, "event: "+evt) {
			t.Errorf("missing event: %s", evt)
		}
	}
}

func TestHandleSSEClientDisconnect(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/events", nil)
	ctx, cancel := context.WithCancel(req.Context())
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		handleSSE(w, req)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)

	// Simulate client disconnect
	cancel()

	select {
	case <-done:
		// Handler exited cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("SSE handler did not exit after client disconnect")
	}
}

func TestHandleSSELiveHTTP(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	url, err := Start(port)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer Stop()

	resp, err := http.Get(url + "/api/events")
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)

	// Read keepalive
	if !scanner.Scan() {
		t.Fatal("no keepalive")
	}
	if !strings.HasPrefix(scanner.Text(), ": connected") {
		t.Fatalf("expected keepalive, got %q", scanner.Text())
	}

	// Publish an event
	go func() {
		time.Sleep(100 * time.Millisecond)
		events.Get().Publish(events.Event{
			Type: events.ExperimentCompleted, TreeID: "live-test",
			Timestamp: time.Now(),
			Payload:   map[string]any{"status": "improved"},
		})
	}()

	// Read until we find our event
	deadline := time.After(3 * time.Second)
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
		if strings.Contains(scanner.Text(), "experiment.completed") {
			found = true
		}
	}
	if !found {
		t.Fatal("did not receive experiment.completed event")
	}
}

func TestHelpers(t *testing.T) {
	// trunc
	if trunc("hello", 10) != "hello" {
		t.Fatal("trunc should not truncate short strings")
	}
	if trunc("hello world", 5) != "hello..." {
		t.Fatalf("trunc: got %q", trunc("hello world", 5))
	}

	// pct
	if pct(0, 0) != 0 {
		t.Fatal("pct(0,0) should be 0")
	}
	if pct(1, 4) != 25 {
		t.Fatalf("pct(1,4): got %d", pct(1, 4))
	}
	if pct(3, 4) != 75 {
		t.Fatalf("pct(3,4): got %d", pct(3, 4))
	}
}

func TestFindBestPath(t *testing.T) {
	d := db.Get()
	trees, _ := tree.ListTrees()
	if len(trees) == 0 {
		t.Fatal("no trees")
	}
	path := findBestPath(d, trees[0].ID)
	// May be nil or non-nil depending on tree state, just verify no panic
	_ = fmt.Sprintf("%v", path)
}
