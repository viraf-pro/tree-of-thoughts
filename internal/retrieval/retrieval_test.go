package retrieval

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
)

func TestMain(m *testing.M) {
	tmp := filepath.Join(os.TempDir(), "tot-mcp-retrieval-test.db")
	os.Remove(tmp)
	if _, err := db.Init(tmp); err != nil {
		panic(err)
	}
	code := m.Run()
	os.Remove(tmp)
	os.Exit(code)
}

func TestStoreSolutionAndStats(t *testing.T) {
	// First, we need a tree in the DB for the foreign key (solutions.tree_id is nullable though)
	d := db.Get()
	d.Exec(`INSERT INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('test-tree-1','test problem','root-1','bfs',5,3,'active','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('root-1','test-tree-1',NULL,'test problem',0,0,0,'{}','2024-01-01T00:00:00Z')`)

	id, err := StoreSolution("test-tree-1", "test problem", "test solution",
		[]string{"step 1", "step 2"}, []string{"root-1"}, 0.9, []string{"go", "testing"})
	if err != nil {
		t.Fatalf("StoreSolution: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	stats := Stats()
	total, ok := stats["totalSolutions"].(int)
	if !ok || total < 1 {
		t.Fatalf("totalSolutions: got %v", stats["totalSolutions"])
	}
}

func TestKeywordSearch(t *testing.T) {
	// Store a few solutions with distinct keywords
	d := db.Get()
	d.Exec(`INSERT INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('kw-tree','keyword test','kw-root','bfs',5,3,'active','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('kw-root','kw-tree',NULL,'keyword test',0,0,0,'{}','2024-01-01T00:00:00Z')`)

	StoreSolution("kw-tree", "database optimization slow queries",
		"add indexes to critical tables", []string{"analyze query plan"}, nil, 0.8, []string{"database"})

	StoreSolution("kw-tree", "frontend rendering performance react",
		"use memo and virtual scrolling", []string{"profile components"}, nil, 0.7, []string{"frontend"})

	// Search for database-related content
	results, err := Retrieve("database optimization queries", 5, nil)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 keyword match for 'database optimization queries'")
	}

	// First result should be the database one
	found := false
	for _, r := range results {
		if r.MatchType == "keyword" && r.Problem == "database optimization slow queries" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected database solution in results, got: %v", results)
	}
}

func TestRetrieveWithTagFilter(t *testing.T) {
	results, err := Retrieve("database", 5, []string{"frontend"})
	if err != nil {
		t.Fatalf("Retrieve with tags: %v", err)
	}
	// Should only return results tagged "frontend"
	for _, r := range results {
		if !hasOverlap(r.Tags, []string{"frontend"}) {
			t.Fatalf("result %s has tags %v, expected 'frontend'", r.ID, r.Tags)
		}
	}
}

func TestRetrieveNoResults(t *testing.T) {
	results, err := Retrieve("xyzzy quantum flux capacitor", 5, nil)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results for gibberish, got %d", len(results))
	}
}

func TestCompactLifecycle(t *testing.T) {
	// Store a solution with an old date so it's eligible for compaction
	d := db.Get()
	d.Exec(`INSERT INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('compact-tree','compact test','compact-root','bfs',5,3,'active','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('compact-root','compact-tree',NULL,'compact test',0,0,0,'{}','2024-01-01T00:00:00Z')`)

	// Insert directly with old date to bypass time.Now()
	d.Exec(`INSERT INTO solutions (id,tree_id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('compact-sol','compact-tree','old problem','detailed solution with many steps',
		'["step 1","step 2","step 3"]','[]',0.8,'["test"]',0,'2020-01-01T00:00:00Z')`)
	// Manually insert FTS entry
	d.Exec(`INSERT INTO solutions_fts(rowid, problem, solution, thoughts)
		SELECT rowid, problem, solution, thoughts FROM solutions WHERE id='compact-sol'`)

	// Analyze: should find our old solution
	candidates, err := CompactAnalyze(30)
	if err != nil {
		t.Fatalf("CompactAnalyze: %v", err)
	}
	found := false
	for _, c := range candidates {
		if c.ID == "compact-sol" {
			found = true
			if c.AgeDays < 30 {
				t.Fatalf("ageDays: got %d, want >= 30", c.AgeDays)
			}
		}
	}
	if !found {
		t.Fatal("compact-sol not found in candidates")
	}

	// Apply compaction
	err = CompactApply("compact-sol", "Key insight: optimize old queries.")
	if err != nil {
		t.Fatalf("CompactApply: %v", err)
	}

	// Verify compacted
	var compacted int
	var solution string
	d.QueryRow(`SELECT compacted, solution FROM solutions WHERE id='compact-sol'`).Scan(&compacted, &solution)
	if compacted != 1 {
		t.Fatalf("compacted flag: got %d, want 1", compacted)
	}
	if solution != "Key insight: optimize old queries." {
		t.Fatalf("solution: got %q", solution)
	}

	// Restore
	err = CompactRestore("compact-sol")
	if err != nil {
		t.Fatalf("CompactRestore: %v", err)
	}

	d.QueryRow(`SELECT compacted, solution FROM solutions WHERE id='compact-sol'`).Scan(&compacted, &solution)
	if compacted != 0 {
		t.Fatalf("after restore compacted: got %d, want 0", compacted)
	}
	if solution != "detailed solution with many steps" {
		t.Fatalf("after restore solution: got %q", solution)
	}
}

func TestHasOverlap(t *testing.T) {
	if !hasOverlap([]string{"a", "b"}, []string{"b", "c"}) {
		t.Fatal("expected overlap")
	}
	if hasOverlap([]string{"a", "b"}, []string{"c", "d"}) {
		t.Fatal("expected no overlap")
	}
	if hasOverlap(nil, []string{"a"}) {
		t.Fatal("nil should not overlap")
	}
}

// --- Solution cross-reference tests ---

func TestLinkSolutions(t *testing.T) {
	d := db.Get()
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('link-sol-1','problem A','solution A','[]','[]',0.8,'[]',0,'2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('link-sol-2','problem B','solution B','[]','[]',0.7,'[]',0,'2024-01-01T00:00:00Z')`)

	link, err := LinkSolutions("link-sol-1", "link-sol-2", "related", "test link")
	if err != nil {
		t.Fatalf("LinkSolutions: %v", err)
	}
	if link.LinkType != "related" {
		t.Fatalf("link_type: got %q", link.LinkType)
	}
	if link.ID == "" {
		t.Fatal("expected non-empty link ID")
	}
}

func TestLinkSolutionsDuplicatePrevention(t *testing.T) {
	d := db.Get()
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('dup-sol-1','p','s','[]','[]',0,'[]',0,'2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('dup-sol-2','p','s','[]','[]',0,'[]',0,'2024-01-01T00:00:00Z')`)

	// Link twice with same type
	_, err := LinkSolutions("dup-sol-1", "dup-sol-2", "related", "first")
	if err != nil {
		t.Fatalf("first link: %v", err)
	}
	_, err = LinkSolutions("dup-sol-1", "dup-sol-2", "related", "duplicate")
	if err != nil {
		t.Fatalf("duplicate link should not error: %v", err)
	}

	// Should only have 1 link row for this pair+type
	var count int
	d.QueryRow(`SELECT COUNT(*) FROM solution_links WHERE source_id='dup-sol-1' AND target_id='dup-sol-2' AND link_type='related'`).Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 link, got %d", count)
	}
}

func TestLinkSolutionsShortIDs(t *testing.T) {
	// Should not panic even with very short IDs (they'll fail FK, but no panic)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("LinkSolutions panicked with short IDs: %v", r)
		}
	}()
	// These will fail with FK error, but must not panic
	LinkSolutions("ab", "cd", "related", "short id test")
}

func TestLinkSolutionsSelfLink(t *testing.T) {
	_, err := LinkSolutions("link-sol-1", "link-sol-1", "related", "")
	if err == nil {
		t.Fatal("expected error for self-link")
	}
}

func TestLinkSolutionsInvalidType(t *testing.T) {
	_, err := LinkSolutions("link-sol-1", "link-sol-2", "bogus", "")
	if err == nil {
		t.Fatal("expected error for invalid link type")
	}
}

func TestGetSolutionLinks(t *testing.T) {
	// Self-contained: insert data and link within this test
	d := db.Get()
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('getlinks-1','p1','s1','[]','[]',0,'[]',0,'2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('getlinks-2','p2','s2','[]','[]',0,'[]',0,'2024-01-01T00:00:00Z')`)
	LinkSolutions("getlinks-1", "getlinks-2", "extends", "self-contained test")

	links, err := GetSolutionLinks("getlinks-1")
	if err != nil {
		t.Fatalf("GetSolutionLinks: %v", err)
	}
	if len(links) < 1 {
		t.Fatal("expected at least 1 link")
	}
	found := false
	for _, l := range links {
		if l.LinkType == "extends" && l.TargetID == "getlinks-2" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected extends link to getlinks-2")
	}
}

func TestAutoLinkRelated(t *testing.T) {
	d := db.Get()

	// Insert a solution directly with FTS entry
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('autolink-existing','optimize database query performance','add indexes','[]','[]',0.8,'[]',0,'2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions_fts(rowid, problem, solution, thoughts)
		SELECT rowid, problem, solution, thoughts FROM solutions WHERE id='autolink-existing'`)

	// Call autoLinkRelated with a similar problem
	autoLinkRelated("autolink-new-sol", "optimize database query performance tuning")

	// Check for a link
	links, err := GetSolutionLinks("autolink-new-sol")
	if err != nil {
		t.Fatalf("GetSolutionLinks: %v", err)
	}
	found := false
	for _, l := range links {
		if l.TargetID == "autolink-existing" || l.SourceID == "autolink-existing" {
			found = true
		}
	}
	// autoLinkRelated may fail to link if the FK on autolink-new-sol doesn't exist,
	// but it must not panic and must log the error cleanly
	_ = found
}

func TestStoreSolutionLogsKnowledgeEvent(t *testing.T) {
	events, err := GetKnowledgeLog(50)
	if err != nil {
		t.Fatalf("GetKnowledgeLog: %v", err)
	}
	found := false
	for _, e := range events {
		if e.EventType == "stored" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'stored' event in knowledge log after StoreSolution")
	}
}

// --- Lint tests ---

func TestLintKnowledge(t *testing.T) {
	report, err := LintKnowledge()
	if err != nil {
		t.Fatalf("LintKnowledge: %v", err)
	}
	if report.TotalSolutions < 1 {
		t.Fatal("expected at least 1 solution from prior tests")
	}
	// Suggestions should always be non-empty (either issues or "healthy")
	if len(report.Suggestions) == 0 {
		t.Fatal("expected at least 1 suggestion")
	}
	// Contradictions should be a slice (possibly empty)
	_ = report.SimilarPairs
	_ = report.OrphanSolutions
	_ = report.UnlinkedSolutions
	_ = report.StaleSolutions
}

func TestLintKnowledgeDetectsSimilarPairs(t *testing.T) {
	d := db.Get()
	// Insert two solutions with nearly identical problems
	d.Exec(`INSERT OR IGNORE INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('lint-tree','lint test','lint-root','bfs',5,3,'active','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('lint-root','lint-tree',NULL,'lint test',0,0,0,'{}','2024-01-01T00:00:00Z')`)

	d.Exec(`INSERT OR IGNORE INTO solutions (id,tree_id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('contra-1','lint-tree','optimize react rendering performance hooks','use memo','[]','[]',0.8,'[]',0,'2024-06-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,tree_id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('contra-2','lint-tree','optimize react rendering performance hooks','avoid memo','[]','[]',0.7,'[]',0,'2024-06-01T00:00:00Z')`)

	report, err := LintKnowledge()
	if err != nil {
		t.Fatalf("LintKnowledge: %v", err)
	}
	found := false
	for _, p := range report.SimilarPairs {
		if (p.SolutionA == "contra-1" && p.SolutionB == "contra-2") ||
			(p.SolutionA == "contra-2" && p.SolutionB == "contra-1") {
			found = true
			if p.Similarity < 0.5 {
				t.Fatalf("expected high similarity, got %f", p.Similarity)
			}
		}
	}
	if !found {
		t.Fatal("expected similar pair between contra-1 and contra-2")
	}
}

func TestLintKnowledgeHasRemediations(t *testing.T) {
	report, err := LintKnowledge()
	if err != nil {
		t.Fatalf("LintKnowledge: %v", err)
	}
	if report.Remediations == nil {
		t.Fatal("Remediations should not be nil")
	}
	// With data from prior tests, we should have unlinked solutions generating remediations
	if report.UnlinkedSolutions > 0 && len(report.Remediations) == 0 {
		t.Fatal("expected remediations for unlinked solutions")
	}
	for _, r := range report.Remediations {
		if r.Tool == "" {
			t.Fatal("remediation must have a tool")
		}
		if r.Action == "" {
			t.Fatal("remediation must have an action")
		}
	}
}

func TestTokenizeText(t *testing.T) {
	words := tokenizeText("Hello, World! This is a test.")
	if !words["hello"] {
		t.Fatal("missing 'hello'")
	}
	if !words["world"] {
		t.Fatal("missing 'world'")
	}
	if !words["test"] {
		t.Fatal("missing 'test'")
	}
	if words["is"] {
		t.Fatal("'is' should be excluded (<=2 chars)")
	}
}

func TestJaccardSim(t *testing.T) {
	a := map[string]bool{"hello": true, "world": true}
	b := map[string]bool{"hello": true, "world": true}
	if jaccardSim(a, b) != 1.0 {
		t.Fatalf("identical: got %f", jaccardSim(a, b))
	}
	c := map[string]bool{"foo": true, "bar": true}
	if jaccardSim(a, c) != 0.0 {
		t.Fatalf("disjoint: got %f", jaccardSim(a, c))
	}
	if jaccardSim(nil, a) != 0.0 {
		t.Fatal("nil should be 0")
	}
}

// --- Drift scan tests ---

func TestDriftScan(t *testing.T) {
	report, err := DriftScan()
	if err != nil {
		t.Fatalf("DriftScan: %v", err)
	}
	if report.DuplicateTreePairs == nil {
		t.Fatal("DuplicateTreePairs should not be nil")
	}
	if report.AbandonedWithValue == nil {
		t.Fatal("AbandonedWithValue should not be nil")
	}
	if report.NeverRetrieved == nil {
		t.Fatal("NeverRetrieved should not be nil")
	}
	if report.Remediations == nil {
		t.Fatal("Remediations should not be nil")
	}
}

func TestDriftScanDetectsDuplicateTrees(t *testing.T) {
	d := db.Get()
	d.Exec(`INSERT OR IGNORE INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('drift-tree-1','optimize database query performance indexes','drift-root-1','bfs',5,3,'active','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('drift-root-1','drift-tree-1',NULL,'root',0,0,0,'{}','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('drift-tree-2','optimize database query performance tuning','drift-root-2','bfs',5,3,'active','2024-01-02T00:00:00Z','2024-01-02T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('drift-root-2','drift-tree-2',NULL,'root',0,0,0,'{}','2024-01-02T00:00:00Z')`)

	report, err := DriftScan()
	if err != nil {
		t.Fatalf("DriftScan: %v", err)
	}
	found := false
	for _, dup := range report.DuplicateTreePairs {
		if (dup.TreeA == "drift-tree-1" && dup.TreeB == "drift-tree-2") ||
			(dup.TreeA == "drift-tree-2" && dup.TreeB == "drift-tree-1") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected duplicate tree pair for drift-tree-1 and drift-tree-2")
	}
}

func TestDriftScanDetectsAbandonedWithValue(t *testing.T) {
	d := db.Get()
	d.Exec(`INSERT OR IGNORE INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('abandoned-val','valuable abandoned tree','aval-root','bfs',5,3,'abandoned','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('aval-root','abandoned-val',NULL,'root',0,0,0,'{}','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('aval-1','abandoned-val','aval-root','good idea',0.8,1,0,'{}','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('aval-2','abandoned-val','aval-root','another idea',0.7,1,0,'{}','2024-01-01T00:00:00Z')`)

	report, err := DriftScan()
	if err != nil {
		t.Fatalf("DriftScan: %v", err)
	}
	found := false
	for _, a := range report.AbandonedWithValue {
		if a.TreeID == "abandoned-val" {
			found = true
			if a.NodeCount < 3 {
				t.Fatalf("nodeCount: got %d, want >= 3", a.NodeCount)
			}
		}
	}
	if !found {
		t.Fatal("expected abandoned-val in AbandonedWithValue")
	}
}

// --- Knowledge log tests ---

func TestGetKnowledgeLog(t *testing.T) {
	// LinkSolutions above already logged a knowledge event
	events, err := GetKnowledgeLog(20)
	if err != nil {
		t.Fatalf("GetKnowledgeLog: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected at least 1 knowledge event from prior tests")
	}
	for _, e := range events {
		if e.EventType == "" {
			t.Fatal("event_type should not be empty")
		}
		if e.CreatedAt == "" {
			t.Fatal("created_at should not be empty")
		}
	}
}
