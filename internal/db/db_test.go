package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	tmp := filepath.Join(os.TempDir(), "tot-mcp-db-test.db")
	os.Remove(tmp)
	if _, err := Init(tmp); err != nil {
		panic(err)
	}
	code := m.Run()
	os.Remove(tmp)
	os.Exit(code)
}

func TestInitAndGet(t *testing.T) {
	d := Get()
	if d == nil {
		t.Fatal("Get() returned nil after Init()")
	}
	// Verify tables exist
	var name string
	err := d.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='trees'`).Scan(&name)
	if err != nil {
		t.Fatalf("trees table not found: %v", err)
	}
}

func TestLogAudit(t *testing.T) {
	LogAudit("tree-1", "node-1", "test_tool", map[string]any{"key": "value"}, "ok")

	entries, err := GetAuditLog("tree-1", 10)
	if err != nil {
		t.Fatalf("GetAuditLog: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least 1 audit entry")
	}

	e := entries[0]
	if e.TreeID != "tree-1" {
		t.Fatalf("treeId: got %q", e.TreeID)
	}
	if e.NodeID != "node-1" {
		t.Fatalf("nodeId: got %q", e.NodeID)
	}
	if e.Tool != "test_tool" {
		t.Fatalf("tool: got %q", e.Tool)
	}
}

func TestGetAuditLogAll(t *testing.T) {
	LogAudit("", "", "global_tool", nil, "global result")

	entries, err := GetAuditLog("", 100)
	if err != nil {
		t.Fatalf("GetAuditLog all: %v", err)
	}
	if len(entries) < 1 {
		t.Fatal("expected entries")
	}
}

func TestTreesEmbeddingColumn(t *testing.T) {
	d := Get()
	// Verify the embedding column exists by inserting a row with it
	_, err := d.Exec(`INSERT INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,embedding,created_at,updated_at)
		VALUES ('emb-test','test','root-emb','bfs',5,3,'active',X'00000000','2024-01-01T00:00:00Z','2024-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert with embedding: %v", err)
	}
	var emb []byte
	d.QueryRow(`SELECT embedding FROM trees WHERE id='emb-test'`).Scan(&emb)
	if len(emb) != 4 {
		t.Fatalf("embedding length: got %d, want 4", len(emb))
	}
}

func TestAuditResultTruncation(t *testing.T) {
	long := make([]byte, 1000)
	for i := range long {
		long[i] = 'x'
	}
	LogAudit("", "", "truncate_test", nil, string(long))

	entries, _ := GetAuditLog("", 1)
	if len(entries) == 0 {
		t.Fatal("expected entry")
	}
	if len(entries[0].Result) > 504 { // 500 + "..."
		t.Fatalf("result not truncated: len=%d", len(entries[0].Result))
	}
}

func TestSolutionLinksTable(t *testing.T) {
	d := Get()
	// Insert referenced solutions first (FK constraint)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('src-1','p','s','[]','[]',0,'[]',0,'2024-01-01T00:00:00Z')`)
	d.Exec(`INSERT OR IGNORE INTO solutions (id,problem,solution,thoughts,path_ids,score,tags,compacted,created_at)
		VALUES ('tgt-1','p','s','[]','[]',0,'[]',0,'2024-01-01T00:00:00Z')`)
	_, err := d.Exec(`INSERT INTO solution_links (id,source_id,target_id,link_type,note,created_at)
		VALUES ('sl-test','src-1','tgt-1','related','test','2024-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert solution_links: %v", err)
	}
	var lt string
	d.QueryRow(`SELECT link_type FROM solution_links WHERE id='sl-test'`).Scan(&lt)
	if lt != "related" {
		t.Fatalf("link_type: got %q", lt)
	}
}

func TestKnowledgeLogTable(t *testing.T) {
	d := Get()
	_, err := d.Exec(`INSERT INTO knowledge_log (event_type,solution_id,detail,created_at)
		VALUES ('stored','sol-1','test detail','2024-01-01T00:00:00Z')`)
	if err != nil {
		t.Fatalf("insert knowledge_log: %v", err)
	}
	var et string
	d.QueryRow(`SELECT event_type FROM knowledge_log WHERE solution_id='sol-1'`).Scan(&et)
	if et != "stored" {
		t.Fatalf("event_type: got %q", et)
	}
}
