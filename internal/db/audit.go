package db

import (
	"encoding/json"
	"time"
)

// AuditEntry is a single audit log row.
type AuditEntry struct {
	ID        int    `json:"id"`
	TreeID    string `json:"treeId,omitempty"`
	NodeID    string `json:"nodeId,omitempty"`
	Tool      string `json:"tool"`
	Input     string `json:"input"`
	Result    string `json:"result"`
	CreatedAt string `json:"createdAt"`
}

// LogAudit records a tool call in the audit log.
func LogAudit(treeID, nodeID, tool string, input any, result string) {
	d := Get()
	inputJSON, _ := json.Marshal(input)
	ts := time.Now().UTC().Format(time.RFC3339)
	// Truncate result to 500 chars to avoid bloating the log
	if len(result) > 500 {
		result = result[:500] + "..."
	}
	d.Exec(`INSERT INTO audit_log (tree_id,node_id,tool,input,result,created_at)
		VALUES (?,?,?,?,?,?)`, treeID, nodeID, tool, string(inputJSON), result, ts)
}

// GetAuditLog returns the last N entries for a tree (or all trees if treeID is empty).
func GetAuditLog(treeID string, limit int) ([]AuditEntry, error) {
	d := Get()
	var query string
	var args []any

	if treeID != "" {
		query = `SELECT id,COALESCE(tree_id,''),COALESCE(node_id,''),tool,input,result,created_at
			FROM audit_log WHERE tree_id=? ORDER BY id DESC LIMIT ?`
		args = []any{treeID, limit}
	} else {
		query = `SELECT id,COALESCE(tree_id,''),COALESCE(node_id,''),tool,input,result,created_at
			FROM audit_log ORDER BY id DESC LIMIT ?`
		args = []any{limit}
	}

	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AuditEntry
	for rows.Next() {
		var e AuditEntry
		rows.Scan(&e.ID, &e.TreeID, &e.NodeID, &e.Tool, &e.Input, &e.Result, &e.CreatedAt)
		out = append(out, e)
	}
	return out, nil
}
