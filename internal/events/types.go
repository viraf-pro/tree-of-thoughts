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
