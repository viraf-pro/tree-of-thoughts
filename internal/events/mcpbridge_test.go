package events

import (
	"testing"
	"time"
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
