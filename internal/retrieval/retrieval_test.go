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
	links, err := GetSolutionLinks("link-sol-1")
	if err != nil {
		t.Fatalf("GetSolutionLinks: %v", err)
	}
	if len(links) < 1 {
		t.Fatal("expected at least 1 link")
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
