package tree

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
)

func TestMain(m *testing.M) {
	tmp := filepath.Join(os.TempDir(), "tot-mcp-tree-test.db")
	os.Remove(tmp)
	if _, err := db.Init(tmp); err != nil {
		panic(err)
	}
	code := m.Run()
	os.Remove(tmp)
	os.Exit(code)
}

func TestCreateTree(t *testing.T) {
	tr, root, err := CreateTree("test problem", "bfs", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}
	if tr.Problem != "test problem" {
		t.Fatalf("problem: got %q", tr.Problem)
	}
	if tr.SearchStrategy != "bfs" {
		t.Fatalf("strategy: got %q", tr.SearchStrategy)
	}
	if tr.Status != "active" {
		t.Fatalf("status: got %q", tr.Status)
	}
	if tr.MaxDepth != 5 {
		t.Fatalf("maxDepth: got %d", tr.MaxDepth)
	}
	if tr.BranchingFactor != 3 {
		t.Fatalf("branchingFactor: got %d", tr.BranchingFactor)
	}
	if root == nil {
		t.Fatal("root node is nil")
	}
	if root.Thought != "test problem" {
		t.Fatalf("root thought: got %q", root.Thought)
	}
	if root.Depth != 0 {
		t.Fatalf("root depth: got %d", root.Depth)
	}
	if root.ParentID != nil {
		t.Fatalf("root parentID: got %v", root.ParentID)
	}
}

func TestGetTree(t *testing.T) {
	tr, _, _ := CreateTree("get tree test", "dfs", 3, 2)
	got, err := GetTree(tr.ID)
	if err != nil {
		t.Fatalf("GetTree: %v", err)
	}
	if got.ID != tr.ID {
		t.Fatalf("ID mismatch")
	}
}

func TestGetTreeNotFound(t *testing.T) {
	_, err := GetTree("nonexistent-id")
	if err == nil {
		t.Fatal("expected error for nonexistent tree")
	}
}

func TestAddThought(t *testing.T) {
	tr, root, _ := CreateTree("add thought test", "bfs", 5, 3)

	child, err := AddThought(tr.ID, root.ID, "child thought 1", map[string]any{"key": "val"})
	if err != nil {
		t.Fatalf("AddThought: %v", err)
	}
	if child.Thought != "child thought 1" {
		t.Fatalf("thought: got %q", child.Thought)
	}
	if child.Depth != 1 {
		t.Fatalf("depth: got %d, want 1", child.Depth)
	}
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("parentID: got %v, want %s", child.ParentID, root.ID)
	}
}

func TestAddThoughtMaxDepth(t *testing.T) {
	tr, root, _ := CreateTree("max depth test", "bfs", 2, 3)

	// depth 0 → 1
	child1, err := AddThought(tr.ID, root.ID, "depth 1", nil)
	if err != nil {
		t.Fatalf("depth 1: %v", err)
	}
	// depth 1 → 2
	child2, err := AddThought(tr.ID, child1.ID, "depth 2", nil)
	if err != nil {
		t.Fatalf("depth 2: %v", err)
	}
	// depth 2 → 3 should fail (max_depth=2)
	_, err = AddThought(tr.ID, child2.ID, "depth 3", nil)
	if err == nil {
		t.Fatal("expected max depth error")
	}
}

func TestGetChildren(t *testing.T) {
	tr, root, _ := CreateTree("children test", "bfs", 5, 3)
	AddThought(tr.ID, root.ID, "c1", nil)
	AddThought(tr.ID, root.ID, "c2", nil)

	children, err := GetChildren(tr.ID, root.ID)
	if err != nil {
		t.Fatalf("GetChildren: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("children count: got %d, want 2", len(children))
	}
}

func TestEvaluateThoughtSure(t *testing.T) {
	tr, root, _ := CreateTree("eval test", "bfs", 5, 3)
	child, _ := AddThought(tr.ID, root.ID, "eval me", nil)

	evaled, err := EvaluateThought(tr.ID, child.ID, "sure", nil)
	if err != nil {
		t.Fatalf("EvaluateThought: %v", err)
	}
	if evaled.Score != 1.0 {
		t.Fatalf("sure score: got %f, want 1.0", evaled.Score)
	}
	if evaled.Evaluation == nil || *evaled.Evaluation != "sure" {
		t.Fatalf("evaluation: got %v", evaled.Evaluation)
	}
}

func TestEvaluateThoughtMaybe(t *testing.T) {
	tr, root, _ := CreateTree("eval maybe", "bfs", 5, 3)
	child, _ := AddThought(tr.ID, root.ID, "maybe", nil)

	evaled, _ := EvaluateThought(tr.ID, child.ID, "maybe", nil)
	if evaled.Score != 0.5 {
		t.Fatalf("maybe score: got %f, want 0.5", evaled.Score)
	}
}

func TestEvaluateThoughtImpossible(t *testing.T) {
	tr, root, _ := CreateTree("eval impossible", "bfs", 5, 3)
	child, _ := AddThought(tr.ID, root.ID, "impossible", nil)

	evaled, _ := EvaluateThought(tr.ID, child.ID, "impossible", nil)
	if evaled.Score != 0.0 {
		t.Fatalf("impossible score: got %f, want 0.0", evaled.Score)
	}
}

func TestEvaluateThoughtCustomScore(t *testing.T) {
	tr, root, _ := CreateTree("eval custom", "bfs", 5, 3)
	child, _ := AddThought(tr.ID, root.ID, "custom", nil)

	score := 0.75
	evaled, _ := EvaluateThought(tr.ID, child.ID, "sure", &score)
	if evaled.Score != 0.75 {
		t.Fatalf("custom score: got %f, want 0.75", evaled.Score)
	}
}

func TestSearchStepBFS(t *testing.T) {
	tr, root, _ := CreateTree("bfs search", "bfs", 5, 3)

	// Add two children at depth 1
	c1, _ := AddThought(tr.ID, root.ID, "shallow-1", nil)
	EvaluateThought(tr.ID, c1.ID, "sure", nil)
	_, _ = AddThought(tr.ID, root.ID, "shallow-2", nil)

	// Add one grandchild at depth 2
	deep, _ := AddThought(tr.ID, c1.ID, "deep", nil)
	EvaluateThought(tr.ID, deep.ID, "sure", nil)

	// Pop root first (it's still in frontier at depth 0)
	node, err := SearchStep(tr.ID)
	if err != nil {
		t.Fatalf("SearchStep: %v", err)
	}
	if node == nil {
		t.Fatal("SearchStep returned nil")
	}
	// BFS: should get shallowest node first (depth 0 = root)
	if node.Depth != 0 {
		t.Fatalf("BFS first pop depth: got %d, want 0", node.Depth)
	}
}

func TestSearchStepDFS(t *testing.T) {
	tr, root, _ := CreateTree("dfs search", "dfs", 5, 3)

	c1, _ := AddThought(tr.ID, root.ID, "shallow", nil)
	EvaluateThought(tr.ID, c1.ID, "sure", nil)

	deep, _ := AddThought(tr.ID, c1.ID, "deep", nil)
	EvaluateThought(tr.ID, deep.ID, "sure", nil)

	// DFS: deepest first
	node, _ := SearchStep(tr.ID)
	if node == nil {
		t.Fatal("SearchStep returned nil")
	}
	if node.Depth != 2 {
		t.Fatalf("DFS first pop depth: got %d, want 2", node.Depth)
	}
}

func TestSearchStepEmptyFrontier(t *testing.T) {
	tr, root, _ := CreateTree("empty frontier", "bfs", 5, 3)

	// Pop the only frontier node (root)
	SearchStep(tr.ID)

	// Now frontier should be empty
	node, err := SearchStep(tr.ID)
	if err != nil {
		t.Fatalf("SearchStep error: %v", err)
	}
	if node != nil {
		t.Fatalf("expected nil, got node %v", node.ID)
	}
	_ = root
}

func TestBacktrack(t *testing.T) {
	tr, root, _ := CreateTree("backtrack test", "bfs", 5, 3)
	c1, _ := AddThought(tr.ID, root.ID, "branch to prune", nil)
	c2, _ := AddThought(tr.ID, c1.ID, "grandchild", nil)

	parent, err := Backtrack(tr.ID, c1.ID)
	if err != nil {
		t.Fatalf("Backtrack: %v", err)
	}
	// Should return the parent of the pruned node (root)
	if parent == nil || parent.ID != root.ID {
		t.Fatalf("parent: got %v, want root", parent)
	}

	// Pruned nodes should be marked impossible
	pruned, _ := GetNode(tr.ID, c1.ID)
	if pruned.Evaluation == nil || *pruned.Evaluation != "impossible" {
		t.Fatalf("pruned c1 evaluation: got %v", pruned.Evaluation)
	}
	pruned2, _ := GetNode(tr.ID, c2.ID)
	if pruned2.Evaluation == nil || *pruned2.Evaluation != "impossible" {
		t.Fatalf("pruned c2 evaluation: got %v", pruned2.Evaluation)
	}
}

func TestBacktrackRoot(t *testing.T) {
	tr, root, _ := CreateTree("backtrack root", "bfs", 5, 3)

	// Backtracking root should return nil (no parent)
	parent, err := Backtrack(tr.ID, root.ID)
	if err != nil {
		t.Fatalf("Backtrack root: %v", err)
	}
	if parent != nil {
		t.Fatalf("expected nil parent, got %v", parent.ID)
	}
}

func TestMarkTerminal(t *testing.T) {
	tr, root, _ := CreateTree("terminal test", "bfs", 5, 3)
	c1, _ := AddThought(tr.ID, root.ID, "solution!", nil)

	node, err := MarkTerminal(tr.ID, c1.ID)
	if err != nil {
		t.Fatalf("MarkTerminal: %v", err)
	}
	if !node.IsTerminal {
		t.Fatal("node should be terminal")
	}

	// Should not appear in search
	// Pop root first, then check c1 isn't returned
	SearchStep(tr.ID) // pops root
	next, _ := SearchStep(tr.ID)
	if next != nil {
		t.Fatalf("terminal node should not be in frontier, got %v", next.ID)
	}
}

func TestGetPathToNode(t *testing.T) {
	tr, root, _ := CreateTree("path test", "bfs", 5, 3)
	c1, _ := AddThought(tr.ID, root.ID, "step 1", nil)
	c2, _ := AddThought(tr.ID, c1.ID, "step 2", nil)

	path, err := GetPathToNode(tr.ID, c2.ID)
	if err != nil {
		t.Fatalf("GetPathToNode: %v", err)
	}
	if path.Depth != 2 {
		t.Fatalf("path depth: got %d, want 2", path.Depth)
	}
	if len(path.NodeIDs) != 3 {
		t.Fatalf("path length: got %d, want 3 (root + 2 children)", len(path.NodeIDs))
	}
	if path.NodeIDs[0] != root.ID {
		t.Fatalf("path[0] should be root")
	}
	if path.NodeIDs[2] != c2.ID {
		t.Fatalf("path[2] should be c2")
	}
}

func TestGetBestPath(t *testing.T) {
	tr, root, _ := CreateTree("best path test", "bfs", 5, 3)

	// Create two branches
	good, _ := AddThought(tr.ID, root.ID, "good branch", nil)
	EvaluateThought(tr.ID, good.ID, "sure", nil)

	bad, _ := AddThought(tr.ID, root.ID, "bad branch", nil)
	EvaluateThought(tr.ID, bad.ID, "impossible", nil)

	// Mark good as terminal
	MarkTerminal(tr.ID, good.ID)

	path, err := GetBestPath(tr.ID)
	if err != nil {
		t.Fatalf("GetBestPath: %v", err)
	}
	if path == nil {
		t.Fatal("GetBestPath returned nil")
	}
	// Path should include root → good
	found := false
	for _, id := range path.NodeIDs {
		if id == good.ID {
			found = true
		}
	}
	if !found {
		t.Fatal("best path should include the 'good' node")
	}
}

func TestNodeCount(t *testing.T) {
	tr, root, _ := CreateTree("count test", "bfs", 5, 3)
	if NodeCount(tr.ID) != 1 {
		t.Fatalf("initial count: got %d, want 1", NodeCount(tr.ID))
	}
	AddThought(tr.ID, root.ID, "c1", nil)
	AddThought(tr.ID, root.ID, "c2", nil)
	if NodeCount(tr.ID) != 3 {
		t.Fatalf("after add: got %d, want 3", NodeCount(tr.ID))
	}
}

func TestSummary(t *testing.T) {
	tr, _, _ := CreateTree("summary test", "bfs", 5, 3)
	summary, err := Summary(tr.ID)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if summary["treeId"] != tr.ID {
		t.Fatalf("treeId mismatch")
	}
	stats := summary["stats"].(map[string]int)
	if stats["totalNodes"] != 1 {
		t.Fatalf("totalNodes: got %d, want 1", stats["totalNodes"])
	}
}

// --- Lifecycle tests ---

func TestSetStatusValidTransitions(t *testing.T) {
	tr, _, _ := CreateTree("lifecycle test", "bfs", 5, 3)

	// active → paused
	if err := SetStatus(tr.ID, "paused"); err != nil {
		t.Fatalf("active → paused: %v", err)
	}
	got, _ := GetTree(tr.ID)
	if got.Status != "paused" {
		t.Fatalf("status: got %q, want paused", got.Status)
	}

	// paused → active
	if err := SetStatus(tr.ID, "active"); err != nil {
		t.Fatalf("paused → active: %v", err)
	}

	// active → abandoned
	if err := SetStatus(tr.ID, "abandoned"); err != nil {
		t.Fatalf("active → abandoned: %v", err)
	}

	// abandoned → active
	if err := SetStatus(tr.ID, "active"); err != nil {
		t.Fatalf("abandoned → active: %v", err)
	}

	// active → solved
	if err := SetStatus(tr.ID, "solved"); err != nil {
		t.Fatalf("active → solved: %v", err)
	}
}

func TestSetStatusInvalidTransition(t *testing.T) {
	tr, _, _ := CreateTree("invalid transition", "bfs", 5, 3)

	// active → active is not valid
	if err := SetStatus(tr.ID, "active"); err == nil {
		t.Fatal("expected error for active → active")
	}

	// solved → active should fail
	SetStatus(tr.ID, "solved")
	if err := SetStatus(tr.ID, "active"); err == nil {
		t.Fatal("expected error for solved → active")
	}
}

// --- Routing tests ---

func TestRouteProblemCreate(t *testing.T) {
	result, err := RouteProblem("completely unique alien topic xyz123")
	if err != nil {
		t.Fatalf("RouteProblem: %v", err)
	}
	if result.Action != "create" {
		t.Fatalf("action: got %q, want create", result.Action)
	}
}

func TestRouteProblemContinue(t *testing.T) {
	CreateTree("optimize database query performance for slow reports", "bfs", 5, 3)

	result, err := RouteProblem("optimize database query performance for slow dashboards")
	if err != nil {
		t.Fatalf("RouteProblem: %v", err)
	}
	if result.Action != "continue" {
		t.Fatalf("action: got %q, want continue (similar problem should match)", result.Action)
	}
}

// --- Link tests ---

func TestLinkTrees(t *testing.T) {
	t1, _, _ := CreateTree("link source", "bfs", 5, 3)
	t2, _, _ := CreateTree("link target", "bfs", 5, 3)

	link, err := LinkTrees(t1.ID, t2.ID, "informs", "test note")
	if err != nil {
		t.Fatalf("LinkTrees: %v", err)
	}
	if link.LinkType != "informs" {
		t.Fatalf("linkType: got %q", link.LinkType)
	}

	links, err := GetTreeLinks(t1.ID)
	if err != nil {
		t.Fatalf("GetTreeLinks: %v", err)
	}
	if len(links) < 1 {
		t.Fatal("expected at least 1 link")
	}
}

func TestLinkTreesInvalidType(t *testing.T) {
	t1, _, _ := CreateTree("link invalid 1", "bfs", 5, 3)
	t2, _, _ := CreateTree("link invalid 2", "bfs", 5, 3)

	_, err := LinkTrees(t1.ID, t2.ID, "bogus", "")
	if err == nil {
		t.Fatal("expected error for invalid link type")
	}
}

func TestLinkTreesSelfLink(t *testing.T) {
	t1, _, _ := CreateTree("self link", "bfs", 5, 3)

	_, err := LinkTrees(t1.ID, t1.ID, "related", "")
	if err == nil {
		t.Fatal("expected error for self-link")
	}
}

// --- Jaccard helper tests ---

func TestTokenize(t *testing.T) {
	words := tokenize("Hello World 42 is a test")
	// "is" and "a" are <=2 chars, should be excluded
	if words["hello"] != true {
		t.Fatal("missing 'hello'")
	}
	if words["world"] != true {
		t.Fatal("missing 'world'")
	}
	if words["test"] != true {
		t.Fatal("missing 'test'")
	}
	if words["is"] {
		t.Fatal("'is' should be excluded (<=2 chars)")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	a := map[string]bool{"hello": true, "world": true}
	b := map[string]bool{"hello": true, "world": true}
	sim := jaccardSimilarity(a, b)
	if sim != 1.0 {
		t.Fatalf("identical sets: got %f, want 1.0", sim)
	}

	c := map[string]bool{"foo": true, "bar": true}
	sim = jaccardSimilarity(a, c)
	if sim != 0.0 {
		t.Fatalf("disjoint sets: got %f, want 0.0", sim)
	}
}

func TestListTrees(t *testing.T) {
	trees, err := ListTrees()
	if err != nil {
		t.Fatalf("ListTrees: %v", err)
	}
	// We've created many trees in prior tests
	if len(trees) < 1 {
		t.Fatal("expected at least 1 tree")
	}
}

func TestCreateTreeStoresEmbedding(t *testing.T) {
	// With noop provider (no API keys set in test), embedding should be nil
	tr, _, err := CreateTree("embedding storage test", "bfs", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}
	got, _ := GetTree(tr.ID)
	if got == nil {
		t.Fatal("GetTree returned nil")
	}
	// Embedding field should be accessible (nil with noop provider is expected)
	_ = got.Embedding
}

func TestRouteProblemFallsBackToJaccard(t *testing.T) {
	// With noop provider, embedding path is skipped, Jaccard is used

	// Abandon all existing trees so they don't interfere with LIMIT 20
	d := db.Get()
	d.Exec(`UPDATE trees SET status='abandoned' WHERE status IN ('active','paused')`)

	tr, _, err := CreateTree("analyze production server memory usage patterns", "bfs", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}
	_ = tr

	// Similar enough for Jaccard to match
	result, err := RouteProblem("analyze production server memory usage issues")
	if err != nil {
		t.Fatalf("RouteProblem: %v", err)
	}
	if result.Action != "continue" {
		t.Fatalf("expected continue for similar problem, got %q (similarity: %f)", result.Action, result.Similarity)
	}
}

func TestSuggestNextWork(t *testing.T) {
	// Create a fresh active tree with frontier
	CreateTree("suggest test tree", "bfs", 5, 3)

	result, err := SuggestNextWork()
	if err != nil {
		t.Fatalf("SuggestNextWork: %v", err)
	}
	action, ok := result["action"].(string)
	if !ok {
		t.Fatal("action missing from result")
	}
	// Should return "continue" since we have active trees with frontier
	if action != "continue" {
		t.Fatalf("action: got %q, want continue", action)
	}
}
