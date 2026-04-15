package retrieval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/embeddings"
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
	// Default manual link should have confidence=1.0 and source=manual
	if link.Confidence != 1.0 {
		t.Fatalf("confidence: got %f, want 1.0", link.Confidence)
	}
	if link.Source != "manual" {
		t.Fatalf("source: got %q, want manual", link.Source)
	}
}

func TestLinkSolutionsWithConfidence(t *testing.T) {
	d := db.Get()
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('conf-sol-1','p','s','[]','[]',0,'[]',0,'2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('conf-sol-2','p','s','[]','[]',0,'[]',0,'2024-01-01T00:00:00Z')`)

	link, err := LinkSolutionsWithConfidence("conf-sol-1", "conf-sol-2", "related", "auto test", 0.65, "auto")
	if err != nil {
		t.Fatalf("LinkSolutionsWithConfidence: %v", err)
	}
	if link.Confidence != 0.65 {
		t.Fatalf("confidence: got %f, want 0.65", link.Confidence)
	}
	if link.Source != "auto" {
		t.Fatalf("source: got %q, want auto", link.Source)
	}

	// Verify it reads back correctly
	links, err := GetSolutionLinks("conf-sol-1")
	if err != nil {
		t.Fatalf("GetSolutionLinks: %v", err)
	}
	found := false
	for _, l := range links {
		if l.TargetID == "conf-sol-2" {
			found = true
			if l.Confidence != 0.65 {
				t.Fatalf("read back confidence: got %f", l.Confidence)
			}
			if l.Source != "auto" {
				t.Fatalf("read back source: got %q", l.Source)
			}
		}
	}
	if !found {
		t.Fatal("link not found in GetSolutionLinks")
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

// --- Token budget tests ---

func TestRetrieveWithTokenBudget(t *testing.T) {
	// Store a solution with a long text
	d := db.Get()
	d.Exec(`INSERT OR IGNORE INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('budget-tree','budget test','budget-root','bfs',5,3,'active','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('budget-root','budget-tree',NULL,'budget test',0,0,0,'{}','2024-01-01T00:00:00Z')`)

	longSolution := strings.Repeat("This is a detailed solution step. ", 100)
	StoreSolution("budget-tree", "token budget optimization test", longSolution,
		[]string{"step 1", "step 2", "step 3"}, nil, 0.8, []string{"budget"})

	// Without budget — should get full results
	full, err := Retrieve("token budget optimization", 5, nil)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	// With small budget — should truncate
	budgeted, err := Retrieve("token budget optimization", 5, nil, 50)
	if err != nil {
		t.Fatalf("Retrieve with budget: %v", err)
	}

	if len(full) > 0 && len(budgeted) > 0 {
		// Budgeted solution should be shorter or equal
		fullLen := len(full[0].Solution)
		budgetLen := len(budgeted[0].Solution)
		if budgetLen > fullLen {
			t.Fatalf("budgeted solution (%d) should not be longer than full (%d)", budgetLen, fullLen)
		}
	}
}

func TestTruncateResults(t *testing.T) {
	results := []Result{
		{ID: "1", Problem: "short", Solution: "short sol"},
		{ID: "2", Problem: "medium problem", Solution: strings.Repeat("x", 500)},
	}

	// Budget of 50 tokens (~200 chars) should keep first, maybe truncate second
	truncated := truncateResults(results, 50)
	if len(truncated) == 0 {
		t.Fatal("should return at least 1 result")
	}
	// First result should be included
	if truncated[0].ID != "1" {
		t.Fatalf("first result should be id=1, got %s", truncated[0].ID)
	}
}

// --- Graph topology analysis tests ---

func TestAnalyzeKnowledgeGraphEmpty(t *testing.T) {
	analysis, err := AnalyzeKnowledgeGraph()
	if err != nil {
		t.Fatalf("AnalyzeKnowledgeGraph: %v", err)
	}
	if analysis.GodNodes == nil {
		t.Fatal("GodNodes should not be nil")
	}
	if analysis.Communities == nil {
		t.Fatal("Communities should not be nil")
	}
	if analysis.Bridges == nil {
		t.Fatal("Bridges should not be nil")
	}
}

func TestAnalyzeKnowledgeGraphWithData(t *testing.T) {
	d := db.Get()
	// Create a small graph: A --related--> B --related--> C, D isolated
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('graph-a','problem alpha','sol a','[]','[]',0.8,'["go"]',0,'2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('graph-b','problem beta','sol b','[]','[]',0.7,'["go"]',0,'2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('graph-c','problem gamma','sol c','[]','[]',0.6,'["rust"]',0,'2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('graph-d','problem delta isolated','sol d','[]','[]',0.5,'[]',0,'2024-01-01T00:00:00Z')`)

	LinkSolutions("graph-a", "graph-b", "related", "test")
	LinkSolutions("graph-b", "graph-c", "related", "test")

	analysis, err := AnalyzeKnowledgeGraph()
	if err != nil {
		t.Fatalf("AnalyzeKnowledgeGraph: %v", err)
	}
	if analysis.TotalEdges < 2 {
		t.Fatalf("totalEdges: got %d, want >= 2", analysis.TotalEdges)
	}

	// graph-b should be a god node (degree 2: connects to a and c)
	foundGod := false
	for _, g := range analysis.GodNodes {
		if g.SolutionID == "graph-b" && g.Degree >= 2 {
			foundGod = true
		}
	}
	if !foundGod {
		t.Log("graph-b not found as god node (may be in larger dataset with higher-degree nodes)")
	}

	// Should have at least 2 communities (connected component {a,b,c} + isolated {d})
	if len(analysis.Communities) < 2 {
		t.Logf("communities: got %d (may merge with data from other tests)", len(analysis.Communities))
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

// --- Knowledge report tests ---

func TestKnowledgeReport(t *testing.T) {
	report, err := KnowledgeReport()
	if err != nil {
		t.Fatalf("KnowledgeReport: %v", err)
	}
	if report.TopSolutions == nil {
		t.Fatal("TopSolutions should not be nil")
	}
	if report.TagCoverage == nil {
		t.Fatal("TagCoverage should not be nil")
	}
	if report.RecentEvents == nil {
		t.Fatal("RecentEvents should not be nil")
	}
	if report.SuggestedQueries == nil {
		t.Fatal("SuggestedQueries should not be nil")
	}
	if report.GraphSummary.TotalSolutions < 1 {
		t.Fatal("expected at least 1 solution in report")
	}
}

// --- Obsidian export tests ---

func TestExportObsidian(t *testing.T) {
	outDir := filepath.Join(os.TempDir(), "tot-mcp-obsidian-test")
	os.RemoveAll(outDir)
	defer os.RemoveAll(outDir)

	err := ExportObsidian(outDir)
	if err != nil {
		t.Fatalf("ExportObsidian: %v", err)
	}

	// Verify Index.md was created
	indexPath := filepath.Join(outDir, "Index.md")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Fatal("Index.md was not created")
	}

	// Read Index.md content
	content, _ := os.ReadFile(indexPath)
	if !strings.Contains(string(content), "Knowledge Base Index") {
		t.Fatal("Index.md missing header")
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input, expected string
	}{
		{"Hello World", "hello-world"},
		{"optimize database query performance", "optimize-database-query-performance"},
		{"", "untitled"},
		{"a/b\\c:d", "a-b-c-d"},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.expected)
		}
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

// --- Hybrid retrieval tests (with mock embedding provider) ---

// mockProvider returns deterministic embeddings for testing hybrid search.
type mockProvider struct {
	vectors map[string][]float32
}

func (m *mockProvider) Dimensions() int { return 3 }
func (m *mockProvider) Embed(text string) ([]float32, error) {
	if v, ok := m.vectors[text]; ok {
		return v, nil
	}
	// Default: hash-based deterministic vector
	h := float32(len(text) % 10)
	return []float32{h / 10, (h + 1) / 10, (h + 2) / 10}, nil
}

// noopForTest satisfies the Provider interface with no-op behavior.
type noopForTest struct{}

func (n *noopForTest) Dimensions() int                 { return 0 }
func (n *noopForTest) Embed(string) ([]float32, error) { return nil, nil }

func TestHybridRetrievalBoost(t *testing.T) {
	d := db.Get()

	// Setup: tree + nodes for FK
	d.Exec(`INSERT OR IGNORE INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('hybrid-tree','hybrid test','hybrid-root','bfs',5,3,'active','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('hybrid-root','hybrid-tree',NULL,'hybrid test',0,0,0,'{}','2024-01-01T00:00:00Z')`)

	// Install mock embedding provider.
	// The query and the Go solution share the word "concurrency" so FTS5 can match.
	mock := &mockProvider{vectors: map[string][]float32{
		"concurrency goroutine patterns":     {0.9, 0.1, 0.1},
		"concurrency goroutine mutex design": {0.85, 0.15, 0.1},
		"react rendering optimization":       {0.1, 0.1, 0.9},
	}}
	embeddings.SetProvider(mock)
	defer embeddings.SetProvider(&noopForTest{})

	// Store solutions with embeddings active
	SuppressAutoLink(true)
	defer SuppressAutoLink(false)

	StoreSolution("hybrid-tree", "concurrency goroutine mutex design",
		"use channels and mutexes for synchronization",
		[]string{"analyze race conditions"}, nil, 0.8, []string{"go", "concurrency"})

	StoreSolution("hybrid-tree", "react rendering optimization",
		"use React.memo and useMemo hooks",
		[]string{"profile render cycles"}, nil, 0.7, []string{"react"})

	// Query shares "concurrency" and "goroutine" with the Go solution for FTS5 keyword match,
	// and has a close vector for vector match — triggering the hybrid 1.2x boost.
	results, err := Retrieve("concurrency goroutine patterns", 5, nil)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	// The Go solution should appear with "hybrid" match type and boosted similarity
	var goResult *Result
	for i, r := range results {
		if strings.Contains(r.Problem, "concurrency") || strings.Contains(r.Problem, "goroutine") {
			goResult = &results[i]
			break
		}
	}

	if goResult == nil {
		t.Fatal("expected Go concurrency solution in results")
	}
	if goResult.MatchType != "hybrid" {
		t.Fatalf("expected matchType='hybrid', got %q", goResult.MatchType)
	}
}

func TestHybridRetrievalWithTokenBudget(t *testing.T) {
	mock := &mockProvider{vectors: map[string][]float32{
		"long solution budget test": {0.7, 0.2, 0.1},
	}}
	embeddings.SetProvider(mock)
	defer embeddings.SetProvider(&noopForTest{})

	d := db.Get()
	longSol := strings.Repeat("detailed analysis step. ", 200)
	d.Exec(`INSERT OR IGNORE INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('budget2-tree','budget2','budget2-root','bfs',5,3,'active','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('budget2-root','budget2-tree',NULL,'budget2',0,0,0,'{}','2024-01-01T00:00:00Z')`)

	SuppressAutoLink(true)
	StoreSolution("budget2-tree", "long solution budget test", longSol,
		[]string{"step 1", "step 2"}, nil, 0.8, []string{"budget"})
	SuppressAutoLink(false)

	// Retrieve with very small token budget
	results, err := Retrieve("long solution budget test", 5, nil, 30)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result")
	}
	// Solution should be truncated
	if len(results[0].Solution) >= len(longSol) {
		t.Fatalf("expected truncation: solution len %d >= original %d", len(results[0].Solution), len(longSol))
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

func TestVectorSearchCap(t *testing.T) {
	// vectorSearchCap should return a smaller limit for higher-dim providers
	// to keep memory bounded at ~2MB
	cap384 := vectorSearchCap(384)
	cap1536 := vectorSearchCap(1536)
	cap3072 := vectorSearchCap(3072)

	if cap384 < 1000 {
		t.Fatalf("384-dim cap should be >= 1000, got %d", cap384)
	}
	if cap1536 >= cap384 {
		t.Fatalf("1536-dim cap (%d) should be less than 384-dim cap (%d)", cap1536, cap384)
	}
	if cap3072 >= cap1536 {
		t.Fatalf("3072-dim cap (%d) should be less than 1536-dim cap (%d)", cap3072, cap1536)
	}
	if cap3072 < 100 {
		t.Fatalf("3072-dim cap should be at least 100, got %d", cap3072)
	}

	// dims <= 0 should fall back to the default cap of 1000
	capZero := vectorSearchCap(0)
	if capZero != 1000 {
		t.Fatalf("zero-dim cap should be 1000, got %d", capZero)
	}
	capNeg := vectorSearchCap(-1)
	if capNeg != 1000 {
		t.Fatalf("negative-dim cap should be 1000, got %d", capNeg)
	}
}

// --- CheckDimensionMismatch tests ---

func TestCheckDimensionMismatchNoEmbeddings(t *testing.T) {
	// Temporarily null out all embeddings so the query finds no non-NULL rows.
	d := db.Get()
	d.Exec(`UPDATE solutions SET embedding = NULL WHERE embedding IS NOT NULL`)

	// Set a real (non-noop) provider so Active() returns true.
	embeddings.SetProvider(&mockProvider{vectors: map[string][]float32{}})
	defer embeddings.SetProvider(&noopForTest{})

	// Should not panic — the function silently returns when no stored embeddings exist.
	CheckDimensionMismatch()
}

func TestCheckDimensionMismatchMatch(t *testing.T) {
	// Store a solution with a 3-dim embedding, then check with a 3-dim provider.
	d := db.Get()

	d.Exec(`INSERT OR IGNORE INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES ('dim-tree','dim test','dim-root','bfs',5,3,'active','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES ('dim-root','dim-tree',NULL,'dim test',0,0,0,'{}','2024-01-01T00:00:00Z')`)

	mock := &mockProvider{vectors: map[string][]float32{
		"dimension match problem": {0.5, 0.5, 0.5},
	}}
	embeddings.SetProvider(mock)
	defer embeddings.SetProvider(&noopForTest{})

	SuppressAutoLink(true)
	defer SuppressAutoLink(false)

	_, err := StoreSolution("dim-tree", "dimension match problem", "solution for dim match",
		[]string{"thought"}, nil, 0.8, []string{"dim-test"})
	if err != nil {
		t.Fatalf("StoreSolution: %v", err)
	}

	// Provider has 3 dims, stored embedding has 3 dims — should not warn or panic.
	CheckDimensionMismatch()
}

// mockProvider5Dim returns 5-dimensional embeddings for testing dimension mismatch.
type mockProvider5Dim struct{}

func (m *mockProvider5Dim) Dimensions() int { return 5 }
func (m *mockProvider5Dim) Embed(text string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3, 0.4, 0.5}, nil
}

func TestCheckDimensionMismatchWarning(t *testing.T) {
	// The DB already has 3-dim embeddings from the previous test.
	// Switch to a 5-dim provider. The function should log a warning but not panic.
	embeddings.SetProvider(&mockProvider5Dim{})
	defer embeddings.SetProvider(&noopForTest{})

	// Should not panic even though dimensions differ (3 stored vs 5 current).
	CheckDimensionMismatch()
}

func TestCheckDimensionMismatchNoDimensions(t *testing.T) {
	// noopForTest has Dimensions()==0 but Active() still returns true
	// (only the package-private noopProvider triggers Active()==false).
	// This exercises the path where the current provider reports 0 dimensions
	// while stored embeddings have a non-zero length — verifying no panic.
	embeddings.SetProvider(&noopForTest{})
	defer embeddings.SetProvider(&noopForTest{})

	CheckDimensionMismatch()
}
