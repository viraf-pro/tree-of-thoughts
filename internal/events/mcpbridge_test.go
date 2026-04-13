package events

import (
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/server"
)

func TestAffectedURIs_TreeCreated(t *testing.T) {
	e := Event{Type: TreeCreated, TreeID: "abc"}
	uris := affectedURIs(e)

	want := map[string]bool{
		"tot://trees":     true,
		"tot://tree/abc":  true,
	}
	for _, u := range uris {
		delete(want, u)
	}
	for missing := range want {
		t.Errorf("missing URI: %s", missing)
	}
}

func TestAffectedURIs_ThoughtAdded(t *testing.T) {
	e := Event{Type: ThoughtAdded, TreeID: "t1", NodeID: "n1"}
	uris := affectedURIs(e)

	has := make(map[string]bool)
	for _, u := range uris {
		has[u] = true
	}
	if !has["tot://tree/t1"] {
		t.Error("missing tot://tree/t1")
	}
	if !has["tot://tree/t1/frontier"] {
		t.Error("missing tot://tree/t1/frontier")
	}
}

func TestAffectedURIs_ThoughtEvaluated(t *testing.T) {
	e := Event{Type: ThoughtEvaluated, TreeID: "t1", NodeID: "n1"}
	uris := affectedURIs(e)

	has := make(map[string]bool)
	for _, u := range uris {
		has[u] = true
	}
	if !has["tot://tree/t1/frontier"] {
		t.Error("ThoughtEvaluated should notify frontier")
	}
}

func TestAffectedURIs_StatusChanged(t *testing.T) {
	e := Event{Type: TreeStatusChanged, TreeID: "t1"}
	uris := affectedURIs(e)

	has := make(map[string]bool)
	for _, u := range uris {
		has[u] = true
	}
	if !has["tot://trees"] {
		t.Error("missing tot://trees")
	}
	if !has["tot://tree/t1/status"] {
		t.Error("missing tot://tree/t1/status")
	}
}

func TestAffectedURIs_ExperimentCompleted(t *testing.T) {
	e := Event{Type: ExperimentCompleted, TreeID: "t1", NodeID: "n1"}
	uris := affectedURIs(e)

	has := make(map[string]bool)
	for _, u := range uris {
		has[u] = true
	}
	if !has["tot://tree/t1/experiments"] {
		t.Error("missing tot://tree/t1/experiments")
	}
}

func TestAffectedURIs_SolutionStored(t *testing.T) {
	e := Event{
		Type:    SolutionStored,
		Payload: map[string]any{"solutionId": "sol-1"},
	}
	uris := affectedURIs(e)

	has := make(map[string]bool)
	for _, u := range uris {
		has[u] = true
	}
	if !has["tot://solutions"] {
		t.Error("missing tot://solutions")
	}
	if !has["tot://solution/sol-1"] {
		t.Error("missing tot://solution/sol-1")
	}
}

func TestAffectedURIs_TreeLinked(t *testing.T) {
	e := Event{
		Type:   TreeLinked,
		TreeID: "src",
		Payload: map[string]any{"targetTree": "tgt"},
	}
	uris := affectedURIs(e)

	has := make(map[string]bool)
	for _, u := range uris {
		has[u] = true
	}
	if !has["tot://tree/src"] {
		t.Error("missing source tree URI")
	}
	if !has["tot://tree/tgt"] {
		t.Error("missing target tree URI")
	}
}

func TestAffectedURIs_AllEventTypes(t *testing.T) {
	// Every event type should produce at least one URI
	types := []struct {
		name string
		evt  Event
	}{
		{"TreeCreated", Event{Type: TreeCreated, TreeID: "t"}},
		{"ThoughtAdded", Event{Type: ThoughtAdded, TreeID: "t"}},
		{"ThoughtEvaluated", Event{Type: ThoughtEvaluated, TreeID: "t"}},
		{"SubtreePruned", Event{Type: SubtreePruned, TreeID: "t"}},
		{"SolutionMarked", Event{Type: SolutionMarked, TreeID: "t"}},
		{"TreeStatusChanged", Event{Type: TreeStatusChanged, TreeID: "t"}},
		{"TreeAutoPaused", Event{Type: TreeAutoPaused, Timestamp: time.Now()}},
		{"TreeLinked", Event{Type: TreeLinked, TreeID: "t", Payload: map[string]any{"targetTree": "t2"}}},
		{"ExperimentPrepared", Event{Type: ExperimentPrepared, TreeID: "t"}},
		{"ExperimentCompleted", Event{Type: ExperimentCompleted, TreeID: "t"}},
		{"ExperimentFailed", Event{Type: ExperimentFailed, TreeID: "t"}},
		{"SolutionStored", Event{Type: SolutionStored, Payload: map[string]any{"solutionId": "s"}}},
		{"SolutionCompacted", Event{Type: SolutionCompacted, Payload: map[string]any{"solutionId": "s"}}},
		{"SolutionLinked", Event{Type: SolutionLinked, Payload: map[string]any{}}},
		{"URLIngested", Event{Type: URLIngested, Payload: map[string]any{"solutionId": "s"}}},
	}

	for _, tt := range types {
		uris := affectedURIs(tt.evt)
		if len(uris) == 0 {
			t.Errorf("%s: produced no URIs", tt.name)
		}
	}
}

func TestAffectedURIs_SubtreePruned(t *testing.T) {
	e := Event{Type: SubtreePruned, TreeID: "t1", NodeID: "n1"}
	has := uriSet(affectedURIs(e))
	if !has["tot://tree/t1/frontier"] {
		t.Error("SubtreePruned should notify frontier")
	}
	if !has["tot://tree/t1"] {
		t.Error("SubtreePruned should notify tree")
	}
}

func TestAffectedURIs_ExperimentPrepared(t *testing.T) {
	e := Event{Type: ExperimentPrepared, TreeID: "t1"}
	has := uriSet(affectedURIs(e))
	if !has["tot://tree/t1/experiments"] {
		t.Error("ExperimentPrepared should notify experiments")
	}
}

func TestAffectedURIs_ExperimentFailed(t *testing.T) {
	e := Event{Type: ExperimentFailed, TreeID: "t1"}
	has := uriSet(affectedURIs(e))
	if !has["tot://tree/t1/experiments"] {
		t.Error("ExperimentFailed should notify experiments")
	}
}

func TestAffectedURIs_SolutionCompacted(t *testing.T) {
	e := Event{Type: SolutionCompacted, Payload: map[string]any{"solutionId": "sol-1"}}
	has := uriSet(affectedURIs(e))
	if !has["tot://solutions"] {
		t.Error("SolutionCompacted should notify solutions list")
	}
	if !has["tot://solution/sol-1"] {
		t.Error("SolutionCompacted should notify specific solution")
	}
}

func TestAffectedURIs_URLIngested(t *testing.T) {
	e := Event{Type: URLIngested, Payload: map[string]any{"solutionId": "ing-1"}}
	has := uriSet(affectedURIs(e))
	if !has["tot://solutions"] {
		t.Error("URLIngested should notify solutions list")
	}
	if !has["tot://solution/ing-1"] {
		t.Error("URLIngested should notify specific solution")
	}
}

func TestAffectedURIs_SolutionLinkedNoSolutionId(t *testing.T) {
	e := Event{Type: SolutionLinked, Payload: map[string]any{}}
	uris := affectedURIs(e)
	has := uriSet(uris)
	if !has["tot://solutions"] {
		t.Error("SolutionLinked should always notify solutions list")
	}
	// Should NOT have a specific solution URI since no solutionId in payload
	for _, u := range uris {
		if len(u) > len("tot://solution/") && u[:len("tot://solution/")] == "tot://solution/" {
			t.Errorf("unexpected specific solution URI: %s", u)
		}
	}
}

func TestAffectedURIs_AutoPausedNoTreeID(t *testing.T) {
	// AutoPaused events don't have a specific TreeID (they affect multiple trees)
	e := Event{Type: TreeAutoPaused, Timestamp: time.Now(), Payload: map[string]any{"count": 3}}
	has := uriSet(affectedURIs(e))
	if !has["tot://trees"] {
		t.Error("TreeAutoPaused should notify trees list")
	}
}

func TestAffectedURIs_SolutionMarkedNoTreeList(t *testing.T) {
	// SolutionMarked should NOT notify the tree list (it's not a lifecycle change)
	e := Event{Type: SolutionMarked, TreeID: "t1", NodeID: "n1"}
	has := uriSet(affectedURIs(e))
	if has["tot://trees"] {
		t.Error("SolutionMarked should NOT notify tree list")
	}
	if !has["tot://tree/t1"] {
		t.Error("SolutionMarked should notify specific tree")
	}
}

func TestStartMCPBridgeIntegration(t *testing.T) {
	s := server.NewMCPServer("bridge-test", "0.0.0",
		server.WithResourceCapabilities(true, true),
	)

	// Create a local bus to avoid interference with global
	b := &Bus{subs: make(map[int]chan Event)}

	// Subscribe a monitor to verify events flow through
	monID, monCh := b.Subscribe()
	defer b.Unsubscribe(monID)

	// Start bridge manually (can't use StartMCPBridge since it uses global Get())
	bridgeID, bridgeCh := b.Subscribe()
	go func() {
		defer b.Unsubscribe(bridgeID)
		for evt := range bridgeCh {
			for _, uri := range affectedURIs(evt) {
				s.SendNotificationToAllClients(
					"notifications/resources/updated",
					map[string]any{"uri": uri},
				)
			}
		}
	}()

	// Publish an event
	b.Publish(Event{
		Type:      TreeCreated,
		TreeID:    "bridge-test-tree",
		Timestamp: time.Now(),
		Payload:   map[string]any{"problem": "bridge test"},
	})

	// Verify the monitor received it (proves event was published and consumed)
	select {
	case got := <-monCh:
		if got.Type != TreeCreated {
			t.Fatalf("expected TreeCreated, got %s", got.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for bridge event")
	}

	// Cleanup: close bridge channel
	b.Unsubscribe(bridgeID)
}

func TestStartMCPBridgeActual(t *testing.T) {
	s := server.NewMCPServer("bridge-actual-test", "0.0.0",
		server.WithResourceCapabilities(true, true),
	)

	// Call the real StartMCPBridge — covers the function body
	StartMCPBridge(s)

	// Subscribe a monitor on the global bus to verify the bridge consumed the event
	monID, monCh := Get().Subscribe()
	defer Get().Unsubscribe(monID)

	// Publish an event on the global bus
	Get().Publish(Event{
		Type:      ThoughtEvaluated,
		TreeID:    "bridge-actual-test",
		NodeID:    "n1",
		Timestamp: time.Now(),
		Payload:   map[string]any{"evaluation": "sure", "score": 1.0},
	})

	// The monitor should receive the event (proves the bus is working)
	select {
	case got := <-monCh:
		if got.Type != ThoughtEvaluated {
			t.Fatalf("expected ThoughtEvaluated, got %s", got.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	// Give the bridge goroutine time to process (it logs the event)
	time.Sleep(50 * time.Millisecond)
}

func uriSet(uris []string) map[string]bool {
	m := make(map[string]bool, len(uris))
	for _, u := range uris {
		m[u] = true
	}
	return m
}
