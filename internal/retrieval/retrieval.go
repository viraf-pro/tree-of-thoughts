package retrieval

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/embeddings"
)

// Result is a retrieved solution with match info.
type Result struct {
	ID         string   `json:"id"`
	Problem    string   `json:"problem"`
	Solution   string   `json:"solution"`
	Thoughts   []string `json:"thoughts"`
	Score      float64  `json:"score"`
	Tags       []string `json:"tags"`
	Similarity float64  `json:"similarity"`
	MatchType  string   `json:"matchType"`
}

// StoreSolution saves a solution with an optional embedding.
func StoreSolution(treeID, problem, solution string, thoughts, pathIDs []string, score float64, tags []string) (string, error) {
	d := db.Get()
	id := uuid.NewString()
	ts := time.Now().UTC().Format(time.RFC3339)

	thoughtsJSON, _ := json.Marshal(thoughts)
	pathJSON, _ := json.Marshal(pathIDs)
	tagsJSON, _ := json.Marshal(tags)

	var embBlob []byte
	if embeddings.Active() {
		text := problem + " " + strings.Join(thoughts, " ") + " " + solution
		vec, err := embeddings.Get().Embed(text)
		if err == nil && len(vec) > 0 {
			embBlob = float32ToBytes(vec)
		}
	}

	_, err := d.Exec(`INSERT INTO solutions (id,tree_id,problem,solution,thoughts,path_ids,score,tags,embedding,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, treeID, problem, solution, string(thoughtsJSON), string(pathJSON), score, string(tagsJSON), embBlob, ts)
	return id, err
}

// Retrieve performs hybrid vector + keyword search.
func Retrieve(query string, topK int, filterTags []string) ([]Result, error) {
	results := map[string]*Result{}

	// Vector search
	if embeddings.Active() {
		vectorResults, err := vectorSearch(query, topK*2)
		if err == nil {
			for _, r := range vectorResults {
				if len(filterTags) > 0 && !hasOverlap(r.Tags, filterTags) {
					continue
				}
				results[r.ID] = &r
			}
		}
	}

	// Keyword search
	kwResults, _ := keywordSearch(query, topK*2)
	for _, r := range kwResults {
		if len(filterTags) > 0 && !hasOverlap(r.Tags, filterTags) {
			continue
		}
		if existing, ok := results[r.ID]; ok {
			existing.Similarity = math.Min(existing.Similarity*1.2, 1.0)
			existing.MatchType = "hybrid"
		} else {
			results[r.ID] = &r
		}
	}

	// Sort by similarity
	sorted := make([]Result, 0, len(results))
	for _, r := range results {
		sorted = append(sorted, *r)
	}
	// Simple insertion sort (small N)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].Similarity > sorted[j-1].Similarity; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	if len(sorted) > topK {
		sorted = sorted[:topK]
	}
	return sorted, nil
}

func vectorSearch(query string, limit int) ([]Result, error) {
	queryVec, err := embeddings.Get().Embed(query)
	if err != nil || len(queryVec) == 0 {
		return nil, err
	}

	d := db.Get()
	rows, err := d.Query(`SELECT id,problem,solution,thoughts,score,tags,embedding FROM solutions WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		Result
		sim float64
	}
	var candidates []scored

	for rows.Next() {
		var id, problem, solution, thoughtsStr, tagsStr string
		var sc float64
		var embBlob []byte
		rows.Scan(&id, &problem, &solution, &thoughtsStr, &sc, &tagsStr, &embBlob)

		if len(embBlob) == 0 {
			continue
		}
		storedVec := bytesToFloat32(embBlob)
		sim := embeddings.CosineSimilarity(queryVec, storedVec)

		var thoughts []string
		var tags []string
		json.Unmarshal([]byte(thoughtsStr), &thoughts)
		json.Unmarshal([]byte(tagsStr), &tags)

		candidates = append(candidates, scored{
			Result: Result{
				ID: id, Problem: problem, Solution: solution,
				Thoughts: thoughts, Score: sc, Tags: tags,
				Similarity: sim, MatchType: "vector",
			},
			sim: sim,
		})
	}

	// Sort descending by similarity
	for i := 1; i < len(candidates); i++ {
		for j := i; j > 0 && candidates[j].sim > candidates[j-1].sim; j-- {
			candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
		}
	}
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	out := make([]Result, len(candidates))
	for i, c := range candidates {
		out[i] = c.Result
	}
	return out, nil
}

func keywordSearch(query string, limit int) ([]Result, error) {
	// Sanitize for FTS5
	re := regexp.MustCompile(`[^\w\s]`)
	safe := re.ReplaceAllString(query, " ")
	words := strings.Fields(safe)
	var filtered []string
	for _, w := range words {
		if len(w) > 2 {
			filtered = append(filtered, w)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	ftsQuery := strings.Join(filtered, " OR ")

	d := db.Get()
	rows, err := d.Query(`SELECT s.id,s.problem,s.solution,s.thoughts,s.score,s.tags,f.rank
		FROM solutions_fts f JOIN solutions s ON s.rowid=f.rowid
		WHERE solutions_fts MATCH ? ORDER BY f.rank LIMIT ?`, ftsQuery, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Result
	for rows.Next() {
		var id, problem, solution, thoughtsStr, tagsStr string
		var sc, rank float64
		rows.Scan(&id, &problem, &solution, &thoughtsStr, &sc, &tagsStr, &rank)

		var thoughts []string
		var tags []string
		json.Unmarshal([]byte(thoughtsStr), &thoughts)
		json.Unmarshal([]byte(tagsStr), &tags)

		sim := math.Max(0, 1-math.Abs(rank)/20)
		out = append(out, Result{
			ID: id, Problem: problem, Solution: solution,
			Thoughts: thoughts, Score: sc, Tags: tags,
			Similarity: sim, MatchType: "keyword",
		})
	}
	return out, nil
}

// Stats returns retrieval store stats.
func Stats() map[string]any {
	d := db.Get()
	var total, withEmb, compacted int
	var avg float64
	d.QueryRow(`SELECT COUNT(*) FROM solutions`).Scan(&total)
	d.QueryRow(`SELECT COUNT(*) FROM solutions WHERE embedding IS NOT NULL`).Scan(&withEmb)
	d.QueryRow(`SELECT COALESCE(AVG(score),0) FROM solutions`).Scan(&avg)
	d.QueryRow(`SELECT COUNT(*) FROM solutions WHERE compacted=1`).Scan(&compacted)

	return map[string]any{
		"totalSolutions":        total,
		"withEmbeddings":        withEmb,
		"compactedSolutions":    compacted,
		"averageScore":          avg,
		"vectorSearchAvailable": embeddings.Active(),
	}
}

// --- Compaction (memory decay) ---

// CompactCandidate is a solution eligible for compaction.
type CompactCandidate struct {
	ID        string   `json:"id"`
	Problem   string   `json:"problem"`
	Solution  string   `json:"solution"`
	Thoughts  []string `json:"thoughts"`
	Tags      []string `json:"tags"`
	Score     float64  `json:"score"`
	CreatedAt string   `json:"createdAt"`
	AgeDays   int      `json:"ageDays"`
}

// CompactAnalyze finds solutions older than minAgeDays that haven't been compacted yet.
// Returns candidates with full content so the LLM can generate summaries.
func CompactAnalyze(minAgeDays int) ([]CompactCandidate, error) {
	d := db.Get()
	cutoff := time.Now().UTC().AddDate(0, 0, -minAgeDays).Format(time.RFC3339)

	rows, err := d.Query(`SELECT id, problem, solution, thoughts, tags, score, created_at
		FROM solutions WHERE compacted=0 AND created_at < ? ORDER BY created_at ASC`, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CompactCandidate
	for rows.Next() {
		var c CompactCandidate
		var thoughtsStr, tagsStr string
		rows.Scan(&c.ID, &c.Problem, &c.Solution, &thoughtsStr, &tagsStr, &c.Score, &c.CreatedAt)
		json.Unmarshal([]byte(thoughtsStr), &c.Thoughts)
		json.Unmarshal([]byte(tagsStr), &c.Tags)

		created, _ := time.Parse(time.RFC3339, c.CreatedAt)
		c.AgeDays = int(time.Since(created).Hours() / 24)
		out = append(out, c)
	}
	return out, nil
}

// CompactApply replaces a solution's detailed thoughts with a summary.
// Keeps the problem, solution (one-liner), tags, and embedding intact.
// The original thoughts are archived in the solution_archive table.
func CompactApply(solutionID, summary string) error {
	d := db.Get()

	// Get original data for archival
	var origThoughts, origSolution string
	err := d.QueryRow(`SELECT solution, thoughts FROM solutions WHERE id=?`, solutionID).Scan(&origSolution, &origThoughts)
	if err != nil {
		return err
	}

	ts := time.Now().UTC().Format(time.RFC3339)

	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Archive original content
	tx.Exec(`INSERT OR REPLACE INTO solution_archive (solution_id, original_solution, original_thoughts, archived_at)
		VALUES (?,?,?,?)`, solutionID, origSolution, origThoughts, ts)

	// Replace with compacted version
	compactedThoughts, _ := json.Marshal([]string{summary})
	tx.Exec(`UPDATE solutions SET solution=?, thoughts=?, compacted=1 WHERE id=?`,
		summary, string(compactedThoughts), solutionID)

	return tx.Commit()
}

// CompactRestore restores a compacted solution to its original full content.
func CompactRestore(solutionID string) error {
	d := db.Get()

	var origSolution, origThoughts string
	err := d.QueryRow(`SELECT original_solution, original_thoughts FROM solution_archive WHERE solution_id=?`,
		solutionID).Scan(&origSolution, &origThoughts)
	if err != nil {
		return err
	}

	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	tx.Exec(`UPDATE solutions SET solution=?, thoughts=?, compacted=0 WHERE id=?`,
		origSolution, origThoughts, solutionID)
	tx.Exec(`DELETE FROM solution_archive WHERE solution_id=?`, solutionID)

	return tx.Commit()
}

func hasOverlap(a, b []string) bool {
	set := map[string]bool{}
	for _, v := range a {
		set[v] = true
	}
	for _, v := range b {
		if set[v] {
			return true
		}
	}
	return false
}

func float32ToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func bytesToFloat32(b []byte) []float32 {
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}
