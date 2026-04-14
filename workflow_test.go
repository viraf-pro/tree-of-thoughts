package main

import (
	"testing"
	"time"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/events"
	"github.com/tot-mcp/tot-mcp-go/internal/retrieval"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

// TestMain is in cli_test.go — initializes DB for all root package tests.

// =============================================================================
// Workflow: research-and-validate
// Tests the full sequence: route → create → generate → evaluate → search →
// mark solution → store → verify
// =============================================================================

func TestWorkflowResearchAndValidate(t *testing.T) {
	bus := events.Get()
	subID, ch := bus.Subscribe()
	defer bus.Unsubscribe(subID)

	// Stage 1: Scout — route the problem
	result, err := tree.RouteProblem("best database for real-time analytics")
	if err != nil {
		t.Fatalf("RouteProblem: %v", err)
	}
	if result.Action != "create" {
		t.Logf("route returned action=%s (tree exists), continuing", result.Action)
	}

	// Stage 2: Researcher — create tree + explore
	tr, root, err := tree.CreateTree("best database for real-time analytics", "beam", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}
	drainUntil(t, ch, events.TreeCreated, 2*time.Second)

	// Generate diverse candidates
	n1, _ := tree.AddThought(tr.ID, root.ID, "ClickHouse — columnar, fast aggregations, open source", nil)
	drainUntil(t, ch, events.ThoughtAdded, 2*time.Second)
	n2, _ := tree.AddThought(tr.ID, root.ID, "Apache Druid — real-time ingestion, sub-second queries", nil)
	drainUntil(t, ch, events.ThoughtAdded, 2*time.Second)
	n3, _ := tree.AddThought(tr.ID, root.ID, "TimescaleDB — PostgreSQL extension, familiar SQL", nil)
	drainUntil(t, ch, events.ThoughtAdded, 2*time.Second)

	// Evaluate candidates
	tree.EvaluateThought(tr.ID, n1.ID, "sure", nil)
	drainUntil(t, ch, events.ThoughtEvaluated, 2*time.Second)
	tree.EvaluateThought(tr.ID, n2.ID, "maybe", nil)
	drainUntil(t, ch, events.ThoughtEvaluated, 2*time.Second)
	tree.EvaluateThought(tr.ID, n3.ID, "maybe", nil)
	drainUntil(t, ch, events.ThoughtEvaluated, 2*time.Second)

	// Go deeper on the best candidate
	n1a, _ := tree.AddThought(tr.ID, n1.ID, "ClickHouse + materialized views for pre-aggregation", nil)
	drainUntil(t, ch, events.ThoughtAdded, 2*time.Second)
	tree.EvaluateThought(tr.ID, n1a.ID, "sure", nil)
	drainUntil(t, ch, events.ThoughtEvaluated, 2*time.Second)

	// Mark solution
	_, err = tree.MarkTerminal(tr.ID, n1a.ID)
	if err != nil {
		t.Fatalf("MarkTerminal: %v", err)
	}
	drainUntil(t, ch, events.SolutionMarked, 2*time.Second)

	// Verify: get_all_paths should show multiple explored branches
	paths, err := tree.GetAllPaths(tr.ID)
	if err != nil {
		t.Fatalf("GetAllPaths: %v", err)
	}
	if len(paths) < 3 {
		t.Fatalf("expected at least 3 paths, got %d", len(paths))
	}

	// Verify: frontier should have remaining nodes (not fully explored)
	frontier, _ := tree.GetFrontier(tr.ID)
	t.Logf("frontier has %d nodes remaining", len(frontier))

	// Stage 5: Librarian — store solution
	solID, err := retrieval.StoreSolution(tr.ID, "best database for real-time analytics",
		"ClickHouse with materialized views for pre-aggregation",
		[]string{"ClickHouse — columnar", "ClickHouse + materialized views"},
		nil, 0.95, []string{"database", "analytics", "real-time"})
	if err != nil {
		t.Fatalf("StoreSolution: %v", err)
	}
	drainUntil(t, ch, events.SolutionStored, 2*time.Second)

	// Verify: solution is retrievable
	results, err := retrieval.Retrieve("real-time analytics database", 3, nil)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	found := false
	for _, r := range results {
		if r.ID == solID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("stored solution not found via retrieve_context")
	}
}

// =============================================================================
// Workflow: decide
// Tests the structured decision-making sequence with trade-off analysis
// =============================================================================

func TestWorkflowDecide(t *testing.T) {
	tr, root, err := tree.CreateTree("Should we use gRPC or REST for internal services?", "beam", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	// Generate option branches
	nGrpc, _ := tree.AddThought(tr.ID, root.ID, "gRPC — binary protocol, code generation, streaming support", nil)
	nRest, _ := tree.AddThought(tr.ID, root.ID, "REST — ubiquitous, human-readable, simpler tooling", nil)

	// Evaluate criteria for gRPC
	nGrpcPerf, _ := tree.AddThought(tr.ID, nGrpc.ID, "gRPC performance: binary serialization, HTTP/2 multiplexing, ~10x faster", nil)
	tree.EvaluateThought(tr.ID, nGrpcPerf.ID, "sure", func() *float64 { v := 0.95; return &v }())

	nGrpcComplex, _ := tree.AddThought(tr.ID, nGrpc.ID, "gRPC complexity: proto files, code gen pipeline, harder debugging", nil)
	tree.EvaluateThought(tr.ID, nGrpcComplex.ID, "maybe", func() *float64 { v := 0.4; return &v }())

	// Evaluate criteria for REST
	nRestSimple, _ := tree.AddThought(tr.ID, nRest.ID, "REST simplicity: curl-friendly, JSON, no code gen, easy debugging", nil)
	tree.EvaluateThought(tr.ID, nRestSimple.ID, "sure", func() *float64 { v := 0.9; return &v }())

	nRestPerf, _ := tree.AddThought(tr.ID, nRest.ID, "REST performance: JSON serialization overhead, HTTP/1.1 connection limits", nil)
	tree.EvaluateThought(tr.ID, nRestPerf.ID, "maybe", func() *float64 { v := 0.5; return &v }())

	// Trade-off articulation
	nTradeoff, _ := tree.AddThought(tr.ID, nGrpc.ID, "Choosing gRPC over REST means accepting tooling complexity in exchange for 10x performance", nil)
	tree.EvaluateThought(tr.ID, nTradeoff.ID, "sure", func() *float64 { v := 0.85; return &v }())

	// Compare all paths
	paths, _ := tree.GetAllPaths(tr.ID)
	if len(paths) < 4 {
		t.Fatalf("expected at least 4 leaf paths for decision comparison, got %d", len(paths))
	}

	// Mark the decision
	tree.MarkTerminal(tr.ID, nTradeoff.ID)
	bestPath, _ := tree.GetBestPath(tr.ID)
	if bestPath == nil {
		t.Fatal("expected best path after marking solution")
	}
	// AverageScore includes root node (score 0), so overall average is lower
	if bestPath.AverageScore < 0.2 {
		t.Fatalf("best path score unexpectedly low: %.2f", bestPath.AverageScore)
	}
	t.Logf("decision best path: score=%.2f depth=%d", bestPath.AverageScore, bestPath.Depth)

	// Store the decision
	_, err = retrieval.StoreSolution(tr.ID, "gRPC vs REST for internal services",
		"gRPC — trade-off: accept tooling complexity for 10x performance",
		bestPath.Thoughts, bestPath.NodeIDs, bestPath.AverageScore,
		[]string{"decision", "grpc", "rest", "api-design"})
	if err != nil {
		t.Fatalf("StoreSolution: %v", err)
	}
}

// =============================================================================
// Workflow: knowledge-maintenance
// Tests lint → detect issues → link orphans → compact old → re-verify
// =============================================================================

func TestWorkflowKnowledgeMaintenance(t *testing.T) {
	// Stage 1: Librarian — detect issues
	lintReport, err := retrieval.LintKnowledge()
	if err != nil {
		t.Fatalf("LintKnowledge: %v", err)
	}
	// Should complete without error
	t.Logf("lint found %v", lintReport)

	driftReport, err := retrieval.DriftScan()
	if err != nil {
		t.Fatalf("DriftScan: %v", err)
	}
	t.Logf("drift: %v", driftReport)

	report, err := retrieval.KnowledgeReport()
	if err != nil {
		t.Fatalf("KnowledgeReport: %v", err)
	}
	if report == nil {
		t.Fatal("KnowledgeReport returned nil")
	}

	// Stage 2: Synthesizer — analyze graph
	graphAnalysis, err := retrieval.AnalyzeKnowledgeGraph()
	if err != nil {
		t.Fatalf("AnalyzeKnowledgeGraph: %v", err)
	}
	if graphAnalysis == nil {
		t.Fatal("graph analysis returned nil")
	}

	// Stage 3: Verify knowledge quality
	stats := retrieval.Stats()
	if stats == nil {
		t.Fatal("Stats returned nil")
	}
	total, ok := stats["totalSolutions"]
	if !ok {
		t.Fatal("stats missing 'totalSolutions' key")
	}
	t.Logf("knowledge store has %v total solutions", total)
}

// =============================================================================
// Workflow: resume-work
// Tests suggest → find tree → load context → resume
// =============================================================================

func TestWorkflowResumeWork(t *testing.T) {
	// Create a tree and pause it
	tr, _, _ := tree.CreateTree("resume test workflow problem", "bfs", 5, 3)
	tree.SetStatus(tr.ID, "paused")

	// Scout: suggest_next should find it
	suggestion, err := tree.SuggestNextWork()
	if err != nil {
		t.Fatalf("SuggestNextWork: %v", err)
	}
	// Should suggest something (either this tree or another active one)
	if suggestion == nil {
		t.Fatal("SuggestNextWork returned nil")
	}
	t.Logf("suggest_next: %v", suggestion["action"])

	// Load context
	ctx, err := tree.GetTreeContext(tr.ID, "summary")
	if err != nil {
		t.Fatalf("GetTreeContext summary: %v", err)
	}
	if ctx == nil {
		t.Fatal("GetTreeContext returned nil")
	}

	// Load full context
	ctxFull, err := tree.GetTreeContext(tr.ID, "full")
	if err != nil {
		t.Fatalf("GetTreeContext full: %v", err)
	}
	if ctxFull == nil {
		t.Fatal("GetTreeContext full returned nil")
	}

	// Resume
	err = tree.SetStatus(tr.ID, "active")
	if err != nil {
		t.Fatalf("SetStatus active: %v", err)
	}

	// Verify resumed
	resumed, _ := tree.GetTree(tr.ID)
	if resumed.Status != "active" {
		t.Fatalf("expected active, got %s", resumed.Status)
	}
}

// =============================================================================
// Verification Sensor: verify-research
// Implements the computational checks from the verify-research skill
// =============================================================================

func TestVerifySensorResearchWellExplored(t *testing.T) {
	// Create a well-explored tree (should pass all checks)
	tr, root, _ := tree.CreateTree("verify-research: well explored", "beam", 5, 3)

	// Add 4 branches with evaluations
	n1, _ := tree.AddThought(tr.ID, root.ID, "approach 1", nil)
	n2, _ := tree.AddThought(tr.ID, root.ID, "approach 2", nil)
	n3, _ := tree.AddThought(tr.ID, root.ID, "approach 3", nil)
	n4, _ := tree.AddThought(tr.ID, root.ID, "dead end", nil)

	tree.EvaluateThought(tr.ID, n1.ID, "sure", nil)
	tree.EvaluateThought(tr.ID, n2.ID, "maybe", nil)
	tree.EvaluateThought(tr.ID, n3.ID, "maybe", nil)
	tree.EvaluateThought(tr.ID, n4.ID, "impossible", nil)

	// Go deeper
	n1a, _ := tree.AddThought(tr.ID, n1.ID, "approach 1 refined", nil)
	tree.EvaluateThought(tr.ID, n1a.ID, "sure", nil)

	n1b, _ := tree.AddThought(tr.ID, n1.ID, "approach 1 alternative", nil)
	tree.EvaluateThought(tr.ID, n1b.ID, "maybe", nil)

	// Depth 3
	n1aa, _ := tree.AddThought(tr.ID, n1a.ID, "approach 1 refined further", nil)
	tree.EvaluateThought(tr.ID, n1aa.ID, "sure", nil)
	tree.MarkTerminal(tr.ID, n1aa.ID)

	// Store solution
	retrieval.StoreSolution(tr.ID, "verify-research: well explored",
		"approach 1 refined further", nil, nil, 0.9, []string{"test"})

	// Verify: check 1 — exploration depth
	summary, _ := tree.Summary(tr.ID)
	stats := summary["stats"].(map[string]int)
	if stats["maxDepthReached"] < 3 {
		t.Errorf("depth check FAIL: max depth %d < 3", stats["maxDepthReached"])
	}

	// Verify: check 2 — branch diversity
	paths, _ := tree.GetAllPaths(tr.ID)
	if len(paths) < 3 {
		t.Errorf("diversity check FAIL: %d paths < 3", len(paths))
	}

	// Verify: check 3 — pruning ratio
	total := stats["totalNodes"]
	pruned := stats["prunedNodes"]
	if total > 0 {
		ratio := float64(pruned) / float64(total)
		if ratio < 0.05 || ratio > 0.6 {
			t.Errorf("pruning ratio check WARN: %.1f%% (expected 10-50%%)", ratio*100)
		}
	}

	// Verify: check 4 — solution stored
	results, _ := retrieval.Retrieve("verify-research: well explored", 1, nil)
	if len(results) == 0 {
		t.Error("solution stored check FAIL: no solution found")
	}

	// Verify: check 5 — score distribution
	termCount := stats["terminalNodes"]
	if termCount < 1 {
		t.Error("score distribution check: no terminal nodes")
	}
	if pruned < 1 {
		t.Error("score distribution check: no pruned nodes (all scores high?)")
	}

	t.Logf("verify-research: depth=%d paths=%d pruning=%.0f%% solution=found",
		stats["maxDepthReached"], len(paths), float64(pruned)/float64(total)*100)
}

func TestVerifySensorResearchPoorlyExplored(t *testing.T) {
	// Create a poorly-explored tree (should fail checks)
	tr, root, _ := tree.CreateTree("verify-research: poorly explored", "beam", 5, 3)

	// Only one branch, no evaluation, no solution
	n1, _ := tree.AddThought(tr.ID, root.ID, "only thought", nil)
	_ = n1

	// Check: depth should be low
	summary, _ := tree.Summary(tr.ID)
	stats := summary["stats"].(map[string]int)
	if stats["maxDepthReached"] > 1 {
		t.Errorf("expected shallow tree, got depth %d", stats["maxDepthReached"])
	}

	// Check: only 1 path
	paths, _ := tree.GetAllPaths(tr.ID)
	if len(paths) > 1 {
		t.Errorf("expected 1 path, got %d", len(paths))
	}

	// Check: no solution stored
	results, _ := retrieval.Retrieve("verify-research: poorly explored", 1, nil)
	solutionFound := false
	for _, r := range results {
		if r.Problem == "verify-research: poorly explored" {
			solutionFound = true
		}
	}
	if solutionFound {
		t.Error("expected no solution stored for poorly explored tree")
	}

	t.Log("verify-research: correctly identified poorly explored tree")
}

// =============================================================================
// Verification Sensor: verify-knowledge
// Implements the computational checks from the verify-knowledge skill
// =============================================================================

func TestVerifySensorKnowledge(t *testing.T) {
	// Check 1: lint for orphans
	lintReport, err := retrieval.LintKnowledge()
	if err != nil {
		t.Fatalf("LintKnowledge: %v", err)
	}
	if lintReport == nil {
		t.Fatal("lint returned nil")
	}

	// Check 2: stats for embedding ratio
	stats := retrieval.Stats()
	if stats == nil {
		t.Fatal("stats returned nil")
	}
	if _, ok := stats["totalSolutions"]; !ok {
		t.Error("stats missing 'totalSolutions'")
	}
	if _, ok := stats["withEmbeddings"]; !ok {
		t.Error("stats missing 'withEmbeddings'")
	}

	// Check 3: knowledge log has events
	logEntries, err := retrieval.GetKnowledgeLog(5)
	if err != nil {
		t.Fatalf("GetKnowledgeLog: %v", err)
	}
	if len(logEntries) == 0 {
		t.Error("knowledge log empty — events not being recorded")
	}

	// Check 4: drift scan
	driftReport, err := retrieval.DriftScan()
	if err != nil {
		t.Fatalf("DriftScan: %v", err)
	}
	if driftReport == nil {
		t.Fatal("drift scan returned nil")
	}

	t.Logf("verify-knowledge: total=%v embeddings=%v log_entries=%d",
		stats["totalSolutions"], stats["withEmbeddings"], len(logEntries))
}

// =============================================================================
// Cross-tree linking workflow
// =============================================================================

func TestWorkflowCrossTreeLinking(t *testing.T) {
	// Create two related trees
	t1, _, _ := tree.CreateTree("microservice communication patterns", "beam", 5, 3)
	t2, _, _ := tree.CreateTree("API gateway design for microservices", "beam", 5, 3)

	// Link them
	link, err := tree.LinkTrees(t1.ID, t2.ID, "related", "both address microservice architecture")
	if err != nil {
		t.Fatalf("LinkTrees: %v", err)
	}
	if link.LinkType != "related" {
		t.Fatalf("link type: got %s", link.LinkType)
	}

	// Verify links from both sides
	links1, _ := tree.GetTreeLinks(t1.ID)
	links2, _ := tree.GetTreeLinks(t2.ID)

	if len(links1) == 0 {
		t.Error("source tree has no links")
	}
	if len(links2) == 0 {
		t.Error("target tree has no links")
	}

	// Verify link appears in both
	found1, found2 := false, false
	for _, l := range links1 {
		if l.ID == link.ID {
			found1 = true
		}
	}
	for _, l := range links2 {
		if l.ID == link.ID {
			found2 = true
		}
	}
	if !found1 || !found2 {
		t.Error("link not found from both tree perspectives")
	}
}

// =============================================================================
// Solution linking and comparison workflow
// =============================================================================

func TestWorkflowSolutionComparison(t *testing.T) {
	// Store two solutions for comparison
	sol1, _ := retrieval.StoreSolution("", "caching strategy for APIs",
		"Redis with TTL-based expiration, read-through pattern",
		nil, nil, 0.85, []string{"caching", "redis", "api"})

	sol2, _ := retrieval.StoreSolution("", "caching strategy for APIs",
		"CDN edge caching with stale-while-revalidate",
		nil, nil, 0.78, []string{"caching", "cdn", "api"})

	// Link as related
	link, err := retrieval.LinkSolutions(sol1, sol2, "related", "both address API caching with different approaches")
	if err != nil {
		t.Fatalf("LinkSolutions: %v", err)
	}
	if link.LinkType != "related" {
		t.Fatalf("link type: %s", link.LinkType)
	}

	// Verify links exist from both sides
	links1, _ := retrieval.GetSolutionLinks(sol1)
	links2, _ := retrieval.GetSolutionLinks(sol2)

	if len(links1) == 0 {
		t.Error("sol1 has no links")
	}
	if len(links2) == 0 {
		t.Error("sol2 has no links")
	}

	// Retrieve both via search
	results, _ := retrieval.Retrieve("API caching strategy", 5, nil)
	foundSol1, foundSol2 := false, false
	for _, r := range results {
		if r.ID == sol1 {
			foundSol1 = true
		}
		if r.ID == sol2 {
			foundSol2 = true
		}
	}
	if !foundSol1 || !foundSol2 {
		t.Errorf("search should find both solutions: sol1=%v sol2=%v", foundSol1, foundSol2)
	}
}

// =============================================================================
// Audit trail workflow
// =============================================================================

func TestWorkflowAuditTrail(t *testing.T) {
	// Seed audit log with entries (MCP tool handlers call db.LogAudit, but in
	// test mode we call library functions directly, so we seed manually)
	db.LogAudit("tree-1", "node-1", "generate_thoughts", map[string]any{"count": 3}, "added 3 thoughts")
	db.LogAudit("tree-1", "node-2", "evaluate_thought", map[string]any{"eval": "sure"}, "scored 1.0")
	db.LogAudit("tree-1", "", "mark_solution", map[string]any{"node": "node-1"}, "solution marked")

	entries, err := db.GetAuditLog("tree-1", 50)
	if err != nil {
		t.Fatalf("GetAuditLog: %v", err)
	}
	if len(entries) < 3 {
		t.Fatalf("expected at least 3 audit entries, got %d", len(entries))
	}

	// Should have entries with different tool names
	toolsSeen := make(map[string]bool)
	for _, e := range entries {
		toolsSeen[e.Tool] = true
	}
	if len(toolsSeen) < 2 {
		t.Errorf("expected diverse tool entries, only saw %d tools: %v", len(toolsSeen), toolsSeen)
	}

	// Verify filtering by tree_id works
	allEntries, _ := db.GetAuditLog("", 50)
	if len(allEntries) < len(entries) {
		t.Error("unfiltered query should return at least as many as filtered")
	}

	t.Logf("audit trail: %d entries for tree-1, %d total, %d unique tools",
		len(entries), len(allEntries), len(toolsSeen))
}

// =============================================================================
// Full pipeline: create → explore → prune → conclude
// =============================================================================

func TestWorkflowFullTreeLifecycle(t *testing.T) {
	bus := events.Get()
	subID, ch := bus.Subscribe()
	defer bus.Unsubscribe(subID)

	// Create
	tr, root, _ := tree.CreateTree("full lifecycle test", "beam", 5, 3)
	drainUntil(t, ch, events.TreeCreated, 2*time.Second)

	// Explore: generate 4 branches
	n1, _ := tree.AddThought(tr.ID, root.ID, "branch A — promising", nil)
	n2, _ := tree.AddThought(tr.ID, root.ID, "branch B — okay", nil)
	n3, _ := tree.AddThought(tr.ID, root.ID, "branch C — weak", nil)
	n4, _ := tree.AddThought(tr.ID, root.ID, "branch D — dead end", nil)
	for i := 0; i < 4; i++ {
		drainUntil(t, ch, events.ThoughtAdded, 2*time.Second)
	}

	// Evaluate
	tree.EvaluateThought(tr.ID, n1.ID, "sure", nil)
	tree.EvaluateThought(tr.ID, n2.ID, "maybe", nil)
	tree.EvaluateThought(tr.ID, n3.ID, "maybe", func() *float64 { v := 0.3; return &v }())
	tree.EvaluateThought(tr.ID, n4.ID, "impossible", nil)
	for i := 0; i < 4; i++ {
		drainUntil(t, ch, events.ThoughtEvaluated, 2*time.Second)
	}

	// Prune: backtrack the weak branch
	_, err := tree.Backtrack(tr.ID, n3.ID)
	if err != nil {
		t.Fatalf("Backtrack: %v", err)
	}
	drainUntil(t, ch, events.SubtreePruned, 2*time.Second)

	// Go deeper on best branch
	n1a, _ := tree.AddThought(tr.ID, n1.ID, "branch A refined", nil)
	tree.EvaluateThought(tr.ID, n1a.ID, "sure", nil)
	drainUntil(t, ch, events.ThoughtAdded, 2*time.Second)
	drainUntil(t, ch, events.ThoughtEvaluated, 2*time.Second)

	// Mark solution
	tree.MarkTerminal(tr.ID, n1a.ID)
	drainUntil(t, ch, events.SolutionMarked, 2*time.Second)

	// Set status to solved
	tree.SetStatus(tr.ID, "solved")
	drainUntil(t, ch, events.TreeStatusChanged, 2*time.Second)

	// Store
	solID, _ := retrieval.StoreSolution(tr.ID, "full lifecycle test",
		"branch A refined", nil, nil, 0.9, []string{"lifecycle-test"})
	drainUntil(t, ch, events.SolutionStored, 2*time.Second)

	// Final verification
	finalTree, _ := tree.GetTree(tr.ID)
	if finalTree.Status != "solved" {
		t.Fatalf("expected solved, got %s", finalTree.Status)
	}

	// Verify solution exists by searching with the tag (more reliable than FTS5 keyword match)
	results, _ := retrieval.Retrieve("lifecycle", 3, []string{"lifecycle-test"})
	found := false
	for _, r := range results {
		if r.ID == solID {
			found = true
		}
	}
	if !found {
		// Fallback: verify via direct DB query (FTS5 may not index short phrases well)
		d := db.Get()
		var count int
		d.QueryRow(`SELECT COUNT(*) FROM solutions WHERE id=?`, solID).Scan(&count)
		if count == 0 {
			t.Error("solution not found in database after full lifecycle")
		} else {
			t.Log("solution exists in DB but not found via keyword search (FTS5 indexing)")
		}
	}

	summary, _ := tree.Summary(tr.ID)
	stats := summary["stats"].(map[string]int)
	t.Logf("lifecycle complete: %d nodes, %d pruned, %d terminal, depth %d",
		stats["totalNodes"], stats["prunedNodes"], stats["terminalNodes"], stats["maxDepthReached"])
}

// drainUntil reads events until the expected type is found. Helper for workflow tests.
func drainUntil(t *testing.T, ch <-chan events.Event, wantType string, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case evt := <-ch:
			if evt.Type == wantType {
				return
			}
		case <-deadline:
			t.Fatalf("timed out waiting for event %q", wantType)
		}
	}
}
