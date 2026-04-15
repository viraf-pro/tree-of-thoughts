package dashboard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/events"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

var srv *http.Server

// Start launches the dashboard on the given port. Non-blocking.
func Start(port int) (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/trees", handleTrees)
	mux.HandleFunc("/api/tree/", handleTreeDetail)
	mux.HandleFunc("/api/experiments/", handleExperiments)
	mux.HandleFunc("/api/retrieval/", handleRetrieval)
	mux.HandleFunc("/api/events", handleSSE)

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// Try any available port
		ln, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return "", err
		}
		addr = ln.Addr().String()
	}

	srv = &http.Server{Handler: mux}
	go func() {
		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Printf("dashboard server error: %v", err)
		}
	}()

	url := "http://" + addr
	log.Printf("Dashboard running at %s", url)
	return url, nil
}

// Stop shuts down the dashboard server.
func Stop() {
	if srv != nil {
		srv.Shutdown(context.Background())
	}
}

// --- API handlers ---

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/tree/") {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(indexHTML))
}

func handleTrees(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		handleCreateTree(w, r)
		return
	}
	d := db.Get()
	rows, err := d.Query(`SELECT t.id, t.problem, t.search_strategy, t.status, t.created_at,
		(SELECT COUNT(*) FROM nodes WHERE tree_id=t.id) as node_count,
		(SELECT COUNT(*) FROM experiment_results WHERE tree_id=t.id) as exp_count
		FROM trees t ORDER BY t.created_at DESC`)
	if err != nil {
		jsonErr(w, err)
		return
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, problem, strategy, status, created string
		var nodes, exps int
		rows.Scan(&id, &problem, &strategy, &status, &created, &nodes, &exps)
		out = append(out, map[string]any{
			"id": id, "problem": trunc(problem, 100), "strategy": strategy,
			"status": status, "createdAt": created, "nodeCount": nodes, "experimentCount": exps,
		})
	}
	jsonOK(w, out)
}

func handleCreateTree(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Problem  string `json:"problem"`
		Strategy string `json:"strategy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonErr(w, fmt.Errorf("invalid JSON body"))
		return
	}
	if body.Problem == "" {
		jsonErr(w, fmt.Errorf("problem is required"))
		return
	}
	if body.Strategy == "" {
		body.Strategy = "beam"
	}

	t, root, err := tree.CreateTree(body.Problem, body.Strategy, 5, 3)
	if err != nil {
		jsonErr(w, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonOK(w, map[string]any{
		"id":       t.ID,
		"problem":  t.Problem,
		"strategy": t.SearchStrategy,
		"rootId":   root.ID,
	})
}

func handleTreeDetail(w http.ResponseWriter, r *http.Request) {
	treeID := strings.TrimPrefix(r.URL.Path, "/api/tree/")
	d := db.Get()

	// Tree metadata
	var id, problem, rootID, strategy, status, created, updated string
	var maxD, bf int
	err := d.QueryRow(`SELECT id,problem,root_id,search_strategy,max_depth,branching_factor,status,created_at,updated_at
		FROM trees WHERE id=?`, treeID).Scan(&id, &problem, &rootID, &strategy, &maxD, &bf, &status, &created, &updated)
	if err != nil {
		jsonErr(w, fmt.Errorf("tree not found"))
		return
	}

	// All nodes
	nodeRows, err2 := d.Query(`SELECT id, parent_id, thought, evaluation, score, depth, is_terminal
		FROM nodes WHERE tree_id=? ORDER BY depth, created_at`, treeID)
	if err2 != nil {
		jsonErr(w, err2)
		return
	}
	defer nodeRows.Close()

	var nodes []map[string]any
	for nodeRows.Next() {
		var nid, thought string
		var parentID, eval sql.NullString
		var score float64
		var depth, term int
		nodeRows.Scan(&nid, &parentID, &thought, &eval, &score, &depth, &term)
		node := map[string]any{
			"id": nid, "thought": thought, "score": score,
			"depth": depth, "isTerminal": term == 1,
		}
		if parentID.Valid {
			node["parentId"] = parentID.String
		}
		if eval.Valid {
			node["evaluation"] = eval.String
		}
		nodes = append(nodes, node)
	}

	// Stats
	var total, terminal, pruned, maxDepth int
	d.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN is_terminal=1 THEN 1 ELSE 0 END),
		SUM(CASE WHEN evaluation='impossible' THEN 1 ELSE 0 END), COALESCE(MAX(depth),0)
		FROM nodes WHERE tree_id=?`, treeID).Scan(&total, &terminal, &pruned, &maxDepth)

	var frontier int
	d.QueryRow(`SELECT COUNT(*) FROM frontier WHERE tree_id=?`, treeID).Scan(&frontier)

	// Best path
	bestPath := findBestPath(d, treeID)

	jsonOK(w, map[string]any{
		"tree": map[string]any{
			"id": id, "problem": problem, "rootId": rootID, "strategy": strategy,
			"maxDepth": maxD, "branchingFactor": bf, "status": status,
		},
		"nodes": nodes,
		"stats": map[string]any{
			"total": total, "terminal": terminal, "pruned": pruned,
			"active": total - terminal - pruned, "maxDepth": maxDepth, "frontier": frontier,
		},
		"bestPath": bestPath,
	})
}

func handleExperiments(w http.ResponseWriter, r *http.Request) {
	treeID := strings.TrimPrefix(r.URL.Path, "/api/experiments/")
	d := db.Get()

	rows, err := d.Query(`SELECT node_id, status, metric, memory_mb, duration_seconds, commit_hash, kept, created_at
		FROM experiment_results WHERE tree_id=? ORDER BY created_at`, treeID)
	if err != nil {
		jsonErr(w, err)
		return
	}
	defer rows.Close()

	var exps []map[string]any
	for rows.Next() {
		var nodeID, status, created string
		var commitHash sql.NullString
		var metric, mem, dur sql.NullFloat64
		var kept int
		rows.Scan(&nodeID, &status, &metric, &mem, &dur, &commitHash, &kept, &created)
		e := map[string]any{
			"nodeId": nodeID, "status": status, "kept": kept == 1, "createdAt": created,
		}
		if metric.Valid {
			e["metric"] = metric.Float64
		}
		if mem.Valid {
			e["memoryMb"] = mem.Float64
		}
		if dur.Valid {
			e["durationSeconds"] = dur.Float64
		}
		if commitHash.Valid {
			e["commitHash"] = commitHash.String
		}
		exps = append(exps, e)
	}

	// Stats
	var total, improved, crashed int
	d.QueryRow(`SELECT COUNT(*) FROM experiment_results WHERE tree_id=?`, treeID).Scan(&total)
	d.QueryRow(`SELECT COUNT(*) FROM experiment_results WHERE tree_id=? AND status='improved'`, treeID).Scan(&improved)
	d.QueryRow(`SELECT COUNT(*) FROM experiment_results WHERE tree_id=? AND status='crashed'`, treeID).Scan(&crashed)

	jsonOK(w, map[string]any{
		"experiments": exps,
		"stats": map[string]any{
			"total": total, "improved": improved, "crashed": crashed,
			"discarded": total - improved - crashed,
			"successRate": pct(improved, total),
		},
	})
}

func handleRetrieval(w http.ResponseWriter, r *http.Request) {
	treeID := strings.TrimPrefix(r.URL.Path, "/api/retrieval/")
	d := db.Get()

	// Get the tree problem for context
	var problem string
	d.QueryRow(`SELECT problem FROM trees WHERE id=?`, treeID).Scan(&problem)

	// Get solutions filtered by tree, or all if no tree specified
	var rows *sql.Rows
	var queryErr error
	if treeID != "" {
		rows, queryErr = d.Query(`SELECT id, problem, solution, thoughts, score, tags, (embedding IS NOT NULL) as has_emb, created_at
			FROM solutions WHERE tree_id=? ORDER BY created_at DESC LIMIT 20`, treeID)
	} else {
		rows, queryErr = d.Query(`SELECT id, problem, solution, thoughts, score, tags, (embedding IS NOT NULL) as has_emb, created_at
			FROM solutions ORDER BY created_at DESC LIMIT 20`)
	}
	if queryErr != nil {
		http.Error(w, queryErr.Error(), 500)
		return
	}
	defer rows.Close()

	var sols []map[string]any
	for rows.Next() {
		var sid, prob, sol, thoughtsStr, tagsStr, created string
		var sc float64
		var hasEmb int
		rows.Scan(&sid, &prob, &sol, &thoughtsStr, &sc, &tagsStr, &hasEmb, &created)
		var tags []string
		if err := json.Unmarshal([]byte(tagsStr), &tags); err != nil { log.Printf("unmarshal tags: %v", err) }
		sols = append(sols, map[string]any{
			"id": sid, "problem": prob, "solution": sol, "tags": tags,
			"score": sc, "hasEmbedding": hasEmb == 1, "createdAt": created,
		})
	}

	var totalSols, withEmb int
	d.QueryRow(`SELECT COUNT(*) FROM solutions`).Scan(&totalSols)
	d.QueryRow(`SELECT COUNT(*) FROM solutions WHERE embedding IS NOT NULL`).Scan(&withEmb)

	jsonOK(w, map[string]any{
		"solutions": sols,
		"stats":     map[string]any{"total": totalSols, "withEmbeddings": withEmb},
	})
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	bus := events.Get()
	id, ch := bus.Subscribe()
	defer bus.Unsubscribe(id)

	// Send initial keepalive
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-r.Context().Done():
			return
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
			if err != nil {
				log.Printf("marshal SSE event: %v", err)
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
			flusher.Flush()
		}
	}
}

// --- Helpers ---

func findBestPath(d *sql.DB, treeID string) []string {
	var candidateID string
	err := d.QueryRow(`SELECT id FROM nodes WHERE tree_id=? AND evaluation!='impossible'
		AND (is_terminal=1 OR id NOT IN (SELECT DISTINCT parent_id FROM nodes WHERE tree_id=? AND parent_id IS NOT NULL))
		ORDER BY is_terminal DESC, score DESC LIMIT 1`, treeID, treeID).Scan(&candidateID)
	if err != nil {
		return nil
	}

	rows, err := d.Query(`WITH RECURSIVE path(id,parent_id) AS (
		SELECT id,parent_id FROM nodes WHERE id=? AND tree_id=?
		UNION ALL SELECT n.id,n.parent_id FROM nodes n JOIN path p ON n.id=p.parent_id WHERE n.tree_id=?
	) SELECT id FROM path`, candidateID, treeID, treeID)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id)
		ids = append(ids, id)
	}
	// Reverse (path CTE goes child→root)
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(400)
	json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func pct(a, b int) int {
	if b == 0 {
		return 0
	}
	return a * 100 / b
}
