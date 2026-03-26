package tree

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/tot-mcp/tot-mcp-go/internal/db"
)

// Node represents a single thought in the tree.
type Node struct {
	ID         string  `json:"id"`
	TreeID     string  `json:"treeId"`
	ParentID   *string `json:"parentId"`
	Thought    string  `json:"thought"`
	Evaluation *string `json:"evaluation"`
	Score      float64 `json:"score"`
	Depth      int     `json:"depth"`
	IsTerminal bool    `json:"isTerminal"`
	Metadata   string  `json:"metadata"`
	CreatedAt  string  `json:"createdAt"`
}

// Tree represents a reasoning tree.
type Tree struct {
	ID              string `json:"id"`
	Problem         string `json:"problem"`
	RootID          string `json:"rootId"`
	SearchStrategy  string `json:"searchStrategy"`
	MaxDepth        int    `json:"maxDepth"`
	BranchingFactor int    `json:"branchingFactor"`
	Status          string `json:"status"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

// PathResult holds a path from root to a node.
type PathResult struct {
	NodeIDs      []string `json:"nodeIds"`
	Thoughts     []string `json:"thoughts"`
	TotalScore   float64  `json:"totalScore"`
	AverageScore float64  `json:"averageScore"`
	Depth        int      `json:"depth"`
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// CreateTree initializes a new reasoning tree.
func CreateTree(problem, strategy string, maxDepth, branchingFactor int) (*Tree, *Node, error) {
	d := db.Get()
	treeID := uuid.NewString()
	rootID := uuid.NewString()
	ts := now()

	tx, err := d.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at)
		VALUES (?,?,?,?,?,?,'active',?,?)`, treeID, problem, rootID, strategy, maxDepth, branchingFactor, ts, ts)
	if err != nil {
		return nil, nil, err
	}

	_, err = tx.Exec(`INSERT INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES (?,?,NULL,?,0,0,0,'{}',?)`, rootID, treeID, problem, ts)
	if err != nil {
		return nil, nil, err
	}

	_, err = tx.Exec(`INSERT INTO frontier (tree_id,node_id,priority) VALUES (?,?,0)`, treeID, rootID)
	if err != nil {
		return nil, nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	tree, _ := GetTree(treeID)
	node, _ := GetNode(treeID, rootID)
	return tree, node, nil
}

// GetTree returns a tree by ID.
func GetTree(treeID string) (*Tree, error) {
	d := db.Get()
	t := &Tree{}
	err := d.QueryRow(`SELECT id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at
		FROM trees WHERE id=?`, treeID).Scan(
		&t.ID, &t.Problem, &t.RootID, &t.SearchStrategy, &t.MaxDepth, &t.BranchingFactor, &t.Status, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// ListTrees returns all trees.
func ListTrees() ([]Tree, error) {
	d := db.Get()
	rows, err := d.Query(`SELECT id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at
		FROM trees ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Tree
	for rows.Next() {
		var t Tree
		rows.Scan(&t.ID, &t.Problem, &t.RootID, &t.SearchStrategy, &t.MaxDepth, &t.BranchingFactor, &t.Status, &t.CreatedAt, &t.UpdatedAt)
		out = append(out, t)
	}
	return out, nil
}

// GetNode returns a node by ID.
func GetNode(treeID, nodeID string) (*Node, error) {
	d := db.Get()
	n := &Node{}
	var parentID sql.NullString
	var eval sql.NullString
	var isTerm int
	err := d.QueryRow(`SELECT id,tree_id,parent_id,thought,evaluation,score,depth,is_terminal,metadata,created_at
		FROM nodes WHERE id=? AND tree_id=?`, nodeID, treeID).Scan(
		&n.ID, &n.TreeID, &parentID, &n.Thought, &eval, &n.Score, &n.Depth, &isTerm, &n.Metadata, &n.CreatedAt)
	if err != nil {
		return nil, err
	}
	if parentID.Valid {
		n.ParentID = &parentID.String
	}
	if eval.Valid {
		n.Evaluation = &eval.String
	}
	n.IsTerminal = isTerm == 1
	return n, nil
}

// GetChildren returns children of a node.
func GetChildren(treeID, nodeID string) ([]Node, error) {
	d := db.Get()
	rows, err := d.Query(`SELECT id,tree_id,parent_id,thought,evaluation,score,depth,is_terminal,metadata,created_at
		FROM nodes WHERE tree_id=? AND parent_id=? ORDER BY created_at`, treeID, nodeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNodes(rows)
}

func scanNodes(rows *sql.Rows) ([]Node, error) {
	var out []Node
	for rows.Next() {
		var n Node
		var parentID, eval sql.NullString
		var isTerm int
		rows.Scan(&n.ID, &n.TreeID, &parentID, &n.Thought, &eval, &n.Score, &n.Depth, &isTerm, &n.Metadata, &n.CreatedAt)
		if parentID.Valid {
			n.ParentID = &parentID.String
		}
		if eval.Valid {
			n.Evaluation = &eval.String
		}
		n.IsTerminal = isTerm == 1
		out = append(out, n)
	}
	return out, nil
}

// AddThought adds a child thought to a parent node.
func AddThought(treeID, parentID, thought string, metadata map[string]any) (*Node, error) {
	d := db.Get()
	parent, err := GetNode(treeID, parentID)
	if err != nil {
		return nil, fmt.Errorf("parent node not found: %w", err)
	}
	tree, err := GetTree(treeID)
	if err != nil {
		return nil, err
	}
	newDepth := parent.Depth + 1
	if newDepth > tree.MaxDepth {
		return nil, fmt.Errorf("max depth %d reached", tree.MaxDepth)
	}

	nodeID := uuid.NewString()
	ts := now()
	meta, _ := json.Marshal(metadata)

	tx, err := d.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO nodes (id,tree_id,parent_id,thought,score,depth,is_terminal,metadata,created_at)
		VALUES (?,?,?,?,0,?,0,?,?)`, nodeID, treeID, parentID, thought, newDepth, string(meta), ts)
	if err != nil {
		return nil, err
	}
	tx.Exec(`INSERT INTO frontier (tree_id,node_id,priority) VALUES (?,?,0)`, treeID, nodeID)
	tx.Exec(`UPDATE trees SET updated_at=? WHERE id=?`, ts, treeID)
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return GetNode(treeID, nodeID)
}

// EvaluateThought scores a node.
func EvaluateThought(treeID, nodeID, evaluation string, customScore *float64) (*Node, error) {
	d := db.Get()
	score := evalToScore(evaluation)
	if customScore != nil {
		score = *customScore
	}

	tx, err := d.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	tx.Exec(`UPDATE nodes SET evaluation=?, score=? WHERE id=? AND tree_id=?`, evaluation, score, nodeID, treeID)
	if evaluation == "impossible" {
		tx.Exec(`DELETE FROM frontier WHERE tree_id=? AND node_id=?`, treeID, nodeID)
	} else {
		tx.Exec(`UPDATE frontier SET priority=? WHERE tree_id=? AND node_id=?`, score, treeID, nodeID)
	}
	tx.Exec(`UPDATE trees SET updated_at=? WHERE id=?`, now(), treeID)
	tx.Commit()
	return GetNode(treeID, nodeID)
}

func evalToScore(e string) float64 {
	switch e {
	case "sure":
		return 1.0
	case "maybe":
		return 0.5
	default:
		return 0.0
	}
}

// MarkTerminal marks a node as a solution.
func MarkTerminal(treeID, nodeID string) (*Node, error) {
	d := db.Get()
	d.Exec(`UPDATE nodes SET is_terminal=1 WHERE id=? AND tree_id=?`, nodeID, treeID)
	d.Exec(`DELETE FROM frontier WHERE tree_id=? AND node_id=?`, treeID, nodeID)
	d.Exec(`UPDATE trees SET updated_at=? WHERE id=?`, now(), treeID)
	return GetNode(treeID, nodeID)
}

// SearchStep returns the next node to expand.
func SearchStep(treeID string) (*Node, error) {
	d := db.Get()
	tree, err := GetTree(treeID)
	if err != nil {
		return nil, err
	}

	// Clean frontier
	d.Exec(`DELETE FROM frontier WHERE tree_id=? AND node_id IN (
		SELECT id FROM nodes WHERE tree_id=? AND (evaluation='impossible' OR is_terminal=1))`, treeID, treeID)

	var query string
	switch tree.SearchStrategy {
	case "dfs":
		query = `SELECT n.id FROM frontier f JOIN nodes n ON n.id=f.node_id WHERE f.tree_id=? ORDER BY n.depth DESC, n.score DESC LIMIT 1`
	case "beam":
		query = `SELECT n.id FROM frontier f JOIN nodes n ON n.id=f.node_id WHERE f.tree_id=? ORDER BY n.score DESC LIMIT 1`
	default: // bfs
		query = `SELECT n.id FROM frontier f JOIN nodes n ON n.id=f.node_id WHERE f.tree_id=? ORDER BY n.depth ASC, n.score DESC LIMIT 1`
	}

	var nextID string
	err = d.QueryRow(query, treeID).Scan(&nextID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Exec(`DELETE FROM frontier WHERE tree_id=? AND node_id=?`, treeID, nextID)
	return GetNode(treeID, nextID)
}

// Backtrack prunes a node and all descendants.
func Backtrack(treeID, nodeID string) (*Node, error) {
	d := db.Get()
	node, err := GetNode(treeID, nodeID)
	if err != nil {
		return nil, err
	}

	// Get all descendants via recursive CTE
	rows, err := d.Query(`WITH RECURSIVE sub(id) AS (
		SELECT id FROM nodes WHERE id=? AND tree_id=?
		UNION ALL
		SELECT n.id FROM nodes n JOIN sub s ON n.parent_id=s.id WHERE n.tree_id=?
	) SELECT id FROM sub`, nodeID, treeID, treeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []any
	var placeholders string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
		if placeholders != "" {
			placeholders += ","
		}
		placeholders += "?"
	}
	if len(ids) > 0 {
		d.Exec(fmt.Sprintf(`UPDATE nodes SET evaluation='impossible', score=0 WHERE id IN (%s)`, placeholders), ids...)
		args := append([]any{treeID}, ids...)
		d.Exec(fmt.Sprintf(`DELETE FROM frontier WHERE tree_id=? AND node_id IN (%s)`, placeholders), args...)
	}
	d.Exec(`UPDATE trees SET updated_at=? WHERE id=?`, now(), treeID)

	if node.ParentID != nil {
		return GetNode(treeID, *node.ParentID)
	}
	return nil, nil
}

// GetPathToNode walks from a node to root.
func GetPathToNode(treeID, nodeID string) (*PathResult, error) {
	d := db.Get()
	rows, err := d.Query(`WITH RECURSIVE path(id,parent_id,thought,score,depth) AS (
		SELECT id,parent_id,thought,score,depth FROM nodes WHERE id=? AND tree_id=?
		UNION ALL
		SELECT n.id,n.parent_id,n.thought,n.score,n.depth FROM nodes n JOIN path p ON n.id=p.parent_id WHERE n.tree_id=?
	) SELECT id,thought,score,depth FROM path ORDER BY depth ASC`, nodeID, treeID, treeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	r := &PathResult{}
	for rows.Next() {
		var id, thought string
		var score float64
		var depth int
		rows.Scan(&id, &thought, &score, &depth)
		r.NodeIDs = append(r.NodeIDs, id)
		r.Thoughts = append(r.Thoughts, thought)
		r.TotalScore += score
		r.Depth = depth
	}
	if len(r.NodeIDs) > 0 {
		r.AverageScore = r.TotalScore / float64(len(r.NodeIDs))
	}
	return r, nil
}

// GetBestPath finds the highest-scoring path to a terminal/leaf node.
func GetBestPath(treeID string) (*PathResult, error) {
	d := db.Get()
	var candidateID string
	err := d.QueryRow(`SELECT id FROM nodes WHERE tree_id=? AND evaluation!='impossible'
		AND (is_terminal=1 OR id NOT IN (SELECT DISTINCT parent_id FROM nodes WHERE tree_id=? AND parent_id IS NOT NULL))
		ORDER BY is_terminal DESC, score DESC, depth DESC LIMIT 1`, treeID, treeID).Scan(&candidateID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return GetPathToNode(treeID, candidateID)
}

// NodeCount returns the number of nodes in a tree.
func NodeCount(treeID string) int {
	d := db.Get()
	var cnt int
	d.QueryRow(`SELECT COUNT(*) FROM nodes WHERE tree_id=?`, treeID).Scan(&cnt)
	return cnt
}

// Summary returns tree stats.
func Summary(treeID string) (map[string]any, error) {
	d := db.Get()
	tree, err := GetTree(treeID)
	if err != nil {
		return nil, err
	}

	var total, terminal, pruned, maxD int
	d.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN is_terminal=1 THEN 1 ELSE 0 END),
		SUM(CASE WHEN evaluation='impossible' THEN 1 ELSE 0 END), MAX(depth)
		FROM nodes WHERE tree_id=?`, treeID).Scan(&total, &terminal, &pruned, &maxD)

	var frontier int
	d.QueryRow(`SELECT COUNT(*) FROM frontier WHERE tree_id=?`, treeID).Scan(&frontier)

	best, _ := GetBestPath(treeID)

	return map[string]any{
		"treeId":          tree.ID,
		"problem":         tree.Problem,
		"status":          tree.Status,
		"searchStrategy":  tree.SearchStrategy,
		"config":          map[string]int{"maxDepth": tree.MaxDepth, "branchingFactor": tree.BranchingFactor},
		"stats":           map[string]int{"totalNodes": total, "terminalNodes": terminal, "prunedNodes": pruned, "activeNodes": total - pruned - terminal, "maxDepthReached": maxD, "frontierSize": frontier},
		"bestPath":        best,
		"createdAt":       tree.CreatedAt,
		"updatedAt":       tree.UpdatedAt,
	}, nil
}

// --- Tree lifecycle management ---

// SetStatus transitions a tree to a new state.
// Valid transitions:
//
//	active  → solved | abandoned | paused
//	paused  → active | abandoned
//	abandoned → active
//	solved  → (terminal, no transitions out)
func SetStatus(treeID, newStatus string) error {
	d := db.Get()
	tree, err := GetTree(treeID)
	if err != nil {
		return fmt.Errorf("tree %s not found", treeID)
	}

	valid := map[string][]string{
		"active":    {"solved", "abandoned", "paused"},
		"paused":    {"active", "abandoned"},
		"abandoned": {"active"},
	}
	allowed, ok := valid[tree.Status]
	if !ok {
		return fmt.Errorf("tree %s is %s and cannot be transitioned", treeID, tree.Status)
	}
	found := false
	for _, s := range allowed {
		if s == newStatus {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("cannot transition tree from %s to %s", tree.Status, newStatus)
	}

	_, err = d.Exec(`UPDATE trees SET status=?, updated_at=? WHERE id=?`, newStatus, now(), treeID)
	return err
}

// TouchTree updates the updated_at timestamp. Call on any tree interaction
// so auto-pause can detect stale trees.
func TouchTree(treeID string) {
	d := db.Get()
	d.Exec(`UPDATE trees SET updated_at=? WHERE id=?`, now(), treeID)
}

// AutoPause marks active trees as paused if they haven't been touched
// in staleMinutes. Call from list_trees and route_problem to keep the
// tree list clean. Returns number of trees paused.
func AutoPause(staleMinutes int) int {
	d := db.Get()
	cutoff := time.Now().UTC().Add(-time.Duration(staleMinutes) * time.Minute).Format(time.RFC3339)
	res, err := d.Exec(`UPDATE trees SET status='paused', updated_at=?
		WHERE status='active' AND updated_at < ?`, now(), cutoff)
	if err != nil {
		return 0
	}
	n, _ := res.RowsAffected()
	return int(n)
}

// RouteProblemResult holds the routing decision for a new problem.
type RouteProblemResult struct {
	Action    string  `json:"action"`    // "continue" or "create"
	TreeID    string  `json:"treeId"`    // existing tree to continue (if action=continue)
	Problem   string  `json:"problem"`   // existing tree's problem text
	Status    string  `json:"status"`    // existing tree's status
	NodeCount int     `json:"nodeCount"` // how many nodes it has
	Similarity float64 `json:"similarity"` // how similar the new problem is (0-1)
	Reason    string  `json:"reason"`    // human-readable explanation
}

// RouteProblem checks if a new problem should continue an existing tree
// or create a new one. It compares the problem text against all active
// and paused trees using embedding similarity (if available) and keyword
// overlap as a fallback.
func RouteProblem(problem string, embedding []float32) (*RouteProblemResult, error) {
	d := db.Get()

	// Auto-pause stale trees first
	AutoPause(30)

	// Get all non-abandoned, non-solved trees
	rows, err := d.Query(`SELECT id, problem, status, updated_at FROM trees
		WHERE status IN ('active', 'paused') ORDER BY updated_at DESC LIMIT 20`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type candidate struct {
		id, problem, status, updatedAt string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		rows.Scan(&c.id, &c.problem, &c.status, &c.updatedAt)
		candidates = append(candidates, c)
	}

	if len(candidates) == 0 {
		return &RouteProblemResult{
			Action: "create",
			Reason: "No active or paused trees found.",
		}, nil
	}

	// Score each candidate by keyword overlap (always available)
	bestIdx := -1
	bestScore := 0.0

	problemWords := tokenize(problem)
	for i, c := range candidates {
		candidateWords := tokenize(c.problem)
		score := jaccardSimilarity(problemWords, candidateWords)
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	// Threshold: 0.3 jaccard overlap = "same topic"
	if bestIdx >= 0 && bestScore >= 0.3 {
		c := candidates[bestIdx]
		nc := NodeCount(c.id)
		return &RouteProblemResult{
			Action:     "continue",
			TreeID:     c.id,
			Problem:    c.problem,
			Status:     c.status,
			NodeCount:  nc,
			Similarity: bestScore,
			Reason:     fmt.Sprintf("Existing tree matches (%.0f%% keyword overlap). Resume instead of creating a new tree.", bestScore*100),
		}, nil
	}

	return &RouteProblemResult{
		Action:     "create",
		Similarity: bestScore,
		Reason:     fmt.Sprintf("No existing tree is similar enough (best match: %.0f%%). Create a new tree.", bestScore*100),
	}, nil
}

// tokenize splits text into lowercase word tokens for jaccard comparison.
func tokenize(text string) map[string]bool {
	words := make(map[string]bool)
	current := []byte{}
	for i := 0; i < len(text); i++ {
		c := text[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			current = append(current, c)
		} else if c >= 'A' && c <= 'Z' {
			current = append(current, c+32) // lowercase
		} else {
			if len(current) > 2 { // skip short words
				words[string(current)] = true
			}
			current = current[:0]
		}
	}
	if len(current) > 2 {
		words[string(current)] = true
	}
	return words
}

// jaccardSimilarity computes |intersection| / |union| of two word sets.
func jaccardSimilarity(a, b map[string]bool) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	intersection := 0
	for w := range a {
		if b[w] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
