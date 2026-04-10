package retrieval

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/embeddings"
	"github.com/tot-mcp/tot-mcp-go/internal/encoding"
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
			embBlob = encoding.Float32ToBytes(vec)
		}
	}

	_, err := d.Exec(`INSERT INTO solutions (id,tree_id,problem,solution,thoughts,path_ids,score,tags,embedding,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		id, treeID, problem, solution, string(thoughtsJSON), string(pathJSON), score, string(tagsJSON), embBlob, ts)
	if err == nil {
		autoLinkRelated(id, problem)
		LogKnowledgeEvent("stored", id, fmt.Sprintf("tree=%s tags=%v", treeID, tags))
	}
	return id, err
}

// autoLinkRelated finds existing solutions similar to the newly stored one and creates links.
func autoLinkRelated(newID, problem string) {
	results, err := keywordSearch(problem, 5)
	if err != nil {
		log.Printf("autoLinkRelated: keyword search failed: %v", err)
		return
	}
	for _, r := range results {
		if r.ID == newID || r.Similarity < 0.3 {
			continue
		}
		if _, err := LinkSolutions(newID, r.ID, "related", "auto-linked on store"); err != nil {
			log.Printf("autoLinkRelated: link failed: %v", err)
		}
	}
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
	if len(sorted) > 0 {
		LogKnowledgeEvent("retrieved", "", fmt.Sprintf("matched=%d", len(sorted)))
	}
	return sorted, nil
}

func vectorSearch(query string, limit int) ([]Result, error) {
	queryVec, err := embeddings.Get().Embed(query)
	if err != nil || len(queryVec) == 0 {
		return nil, err
	}

	d := db.Get()
	// Pure Go cosine similarity requires loading embeddings into memory.
	// Cap at 1000 most recent to bound memory usage (~1.5MB at 384-dim).
	rows, err := d.Query(`SELECT id,problem,solution,thoughts,score,tags,embedding FROM solutions WHERE embedding IS NOT NULL ORDER BY created_at DESC LIMIT 1000`)
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
		storedVec := encoding.BytesToFloat32(embBlob)
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
	if _, err = tx.Exec(`INSERT OR REPLACE INTO solution_archive (solution_id, original_solution, original_thoughts, archived_at)
		VALUES (?,?,?,?)`, solutionID, origSolution, origThoughts, ts); err != nil {
		return fmt.Errorf("archive insert failed: %w", err)
	}

	// Replace with compacted version
	compactedThoughts, _ := json.Marshal([]string{summary})
	if _, err = tx.Exec(`UPDATE solutions SET solution=?, thoughts=?, compacted=1 WHERE id=?`,
		summary, string(compactedThoughts), solutionID); err != nil {
		return fmt.Errorf("compact update failed: %w", err)
	}

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

	if _, err = tx.Exec(`UPDATE solutions SET solution=?, thoughts=?, compacted=0 WHERE id=?`,
		origSolution, origThoughts, solutionID); err != nil {
		return fmt.Errorf("restore update failed: %w", err)
	}
	if _, err = tx.Exec(`DELETE FROM solution_archive WHERE solution_id=?`, solutionID); err != nil {
		return fmt.Errorf("archive delete failed: %w", err)
	}

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

// --- Solution Cross-References ---

// SolutionLink represents a cross-reference between two solutions.
type SolutionLink struct {
	ID        string `json:"id"`
	SourceID  string `json:"sourceId"`
	TargetID  string `json:"targetId"`
	LinkType  string `json:"linkType"`
	Note      string `json:"note"`
	CreatedAt string `json:"createdAt"`
}

var validSolutionLinkTypes = map[string]bool{
	"related": true, "supersedes": true, "contradicts": true, "extends": true,
}

// LinkSolutions creates a cross-reference between two solutions.
func LinkSolutions(sourceID, targetID, linkType, note string) (*SolutionLink, error) {
	if sourceID == targetID {
		return nil, fmt.Errorf("cannot link a solution to itself")
	}
	if !validSolutionLinkTypes[linkType] {
		return nil, fmt.Errorf("invalid link type %q (valid: related, supersedes, contradicts, extends)", linkType)
	}

	d := db.Get()
	id := uuid.NewString()
	ts := time.Now().UTC().Format(time.RFC3339)

	res, err := d.Exec(`INSERT OR IGNORE INTO solution_links (id,source_id,target_id,link_type,note,created_at)
		VALUES (?,?,?,?,?,?)`, id, sourceID, targetID, linkType, note, ts)
	if err != nil {
		return nil, err
	}
	// If the link already exists (UNIQUE constraint), return without logging
	if n, _ := res.RowsAffected(); n == 0 {
		return &SolutionLink{
			ID: id, SourceID: sourceID, TargetID: targetID,
			LinkType: linkType, Note: note, CreatedAt: ts,
		}, nil
	}

	LogKnowledgeEvent("linked", sourceID, fmt.Sprintf("%s -> %s (%s)", truncID(sourceID), truncID(targetID), linkType))

	return &SolutionLink{
		ID: id, SourceID: sourceID, TargetID: targetID,
		LinkType: linkType, Note: note, CreatedAt: ts,
	}, nil
}

// GetSolutionLinks returns all links where the given solution is source or target.
func GetSolutionLinks(solutionID string) ([]SolutionLink, error) {
	d := db.Get()
	rows, err := d.Query(`SELECT id,source_id,target_id,link_type,note,created_at
		FROM solution_links WHERE source_id=? OR target_id=? ORDER BY created_at DESC`,
		solutionID, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SolutionLink, 0)
	for rows.Next() {
		var l SolutionLink
		if err := rows.Scan(&l.ID, &l.SourceID, &l.TargetID, &l.LinkType, &l.Note, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan solution_links: %w", err)
		}
		out = append(out, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating solution_links: %w", err)
	}
	return out, nil
}

// --- Knowledge Event Log ---

// KnowledgeEvent is a single knowledge log entry.
type KnowledgeEvent struct {
	ID         int    `json:"id"`
	EventType  string `json:"eventType"`
	SolutionID string `json:"solutionId,omitempty"`
	Detail     string `json:"detail"`
	CreatedAt  string `json:"createdAt"`
}

// LogKnowledgeEvent appends to the knowledge_log table.
func LogKnowledgeEvent(eventType, solutionID, detail string) {
	d := db.Get()
	ts := time.Now().UTC().Format(time.RFC3339)
	if _, err := d.Exec(`INSERT INTO knowledge_log (event_type,solution_id,detail,created_at)
		VALUES (?,?,?,?)`, eventType, solutionID, detail, ts); err != nil {
		log.Printf("knowledge_log insert failed: %v", err)
	}
}

// GetKnowledgeLog returns recent knowledge events.
func GetKnowledgeLog(limit int) ([]KnowledgeEvent, error) {
	d := db.Get()
	rows, err := d.Query(`SELECT id,event_type,COALESCE(solution_id,''),detail,created_at
		FROM knowledge_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]KnowledgeEvent, 0)
	for rows.Next() {
		var e KnowledgeEvent
		if err := rows.Scan(&e.ID, &e.EventType, &e.SolutionID, &e.Detail, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge_log: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating knowledge_log: %w", err)
	}
	return out, nil
}

// --- Knowledge Lint ---

// Remediation is an actionable fix suggestion with the exact tool call to make.
type Remediation struct {
	Issue  string         `json:"issue"`
	Action string         `json:"action"`
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args,omitempty"`
}

// LintReport summarizes the health of the knowledge store.
type LintReport struct {
	TotalSolutions    int            `json:"totalSolutions"`
	OrphanSolutions   int            `json:"orphanSolutions"`
	UnlinkedSolutions int            `json:"unlinkedSolutions"`
	StaleSolutions    int            `json:"staleSolutions"`
	SimilarPairs      []SimilarPair  `json:"similarPairs"`
	Suggestions       []string       `json:"suggestions"`
	Remediations      []Remediation  `json:"remediations"`
}

// SimilarPair represents two solutions with highly similar problem statements.
// The LLM should review whether they actually contradict, extend, or duplicate each other.
type SimilarPair struct {
	SolutionA  string  `json:"solutionA"`
	SolutionB  string  `json:"solutionB"`
	ProblemA   string  `json:"problemA"`
	ProblemB   string  `json:"problemB"`
	Similarity float64 `json:"similarity"`
}

// LintKnowledge health-checks the knowledge store.
func LintKnowledge() (*LintReport, error) {
	d := db.Get()
	report := &LintReport{
		SimilarPairs: make([]SimilarPair, 0),
		Suggestions:  make([]string, 0),
		Remediations: make([]Remediation, 0),
	}

	d.QueryRow(`SELECT COUNT(*) FROM solutions`).Scan(&report.TotalSolutions)

	// Orphan solutions: from abandoned trees or trees that were deleted
	d.QueryRow(`SELECT COUNT(*) FROM solutions s
		LEFT JOIN trees t ON s.tree_id=t.id
		WHERE s.tree_id IS NOT NULL AND s.tree_id != ''
		AND (t.id IS NULL OR t.status='abandoned')`).Scan(&report.OrphanSolutions)

	// Unlinked solutions: no entries in solution_links
	d.QueryRow(`SELECT COUNT(*) FROM solutions s
		WHERE NOT EXISTS (SELECT 1 FROM solution_links sl WHERE sl.source_id=s.id OR sl.target_id=s.id)`).Scan(&report.UnlinkedSolutions)

	// Stale solutions: older than 60 days, not compacted, not linked
	d.QueryRow(`SELECT COUNT(*) FROM solutions
		WHERE compacted=0
		AND created_at < datetime('now', '-60 days')
		AND NOT EXISTS (SELECT 1 FROM solution_links sl WHERE sl.source_id=id OR sl.target_id=id)`).Scan(&report.StaleSolutions)

	if report.UnlinkedSolutions > 0 {
		report.Suggestions = append(report.Suggestions,
			fmt.Sprintf("%d solutions have no cross-references. Run retrieve_context on their problems to find connections.", report.UnlinkedSolutions))
		// Generate specific remediations for up to 5 unlinked solutions
		unlinkedRows, err := d.Query(`SELECT id, problem FROM solutions s
			WHERE NOT EXISTS (SELECT 1 FROM solution_links sl WHERE sl.source_id=s.id OR sl.target_id=s.id)
			LIMIT 5`)
		if err == nil {
			defer unlinkedRows.Close()
			for unlinkedRows.Next() {
				var solID, problem string
				if err := unlinkedRows.Scan(&solID, &problem); err != nil {
					continue
				}
				report.Remediations = append(report.Remediations, Remediation{
					Issue:  fmt.Sprintf("Solution %s has no cross-references", truncID(solID)),
					Action: "Search for related solutions and create links",
					Tool:   "retrieve_context",
					Args:   map[string]any{"query": problem, "top_k": 3},
				})
			}
		}
	}
	if report.StaleSolutions > 0 {
		report.Suggestions = append(report.Suggestions,
			fmt.Sprintf("%d solutions are stale (60+ days, no links). Consider compact_analyze or re-evaluation.", report.StaleSolutions))
	}
	if report.OrphanSolutions > 0 {
		report.Suggestions = append(report.Suggestions,
			fmt.Sprintf("%d solutions are from abandoned trees. Review whether they're still relevant.", report.OrphanSolutions))
		report.Remediations = append(report.Remediations, Remediation{
			Issue:  fmt.Sprintf("%d solutions from abandoned trees", report.OrphanSolutions),
			Action: "Review orphan solutions and decide if they should be kept or compacted",
			Tool:   "compact_analyze",
			Args:   map[string]any{"min_age_days": 30},
		})
	}

	report.SimilarPairs = findSimilarPairs()

	for _, pair := range report.SimilarPairs {
		report.Remediations = append(report.Remediations, Remediation{
			Issue:  fmt.Sprintf("Solutions %s and %s have %.0f%% similar problems", truncID(pair.SolutionA), truncID(pair.SolutionB), pair.Similarity*100),
			Action: "Review and link these solutions with the appropriate relationship type",
			Tool:   "link_solutions",
			Args:   map[string]any{"source_id": pair.SolutionA, "target_id": pair.SolutionB, "link_type": "related"},
		})
	}

	if len(report.SimilarPairs) > 0 {
		report.Suggestions = append(report.Suggestions,
			fmt.Sprintf("%d solution pairs have very similar problems. Review whether they contradict, extend, or duplicate each other.", len(report.SimilarPairs)))
	}

	if len(report.Suggestions) == 0 {
		report.Suggestions = append(report.Suggestions, "Knowledge store looks healthy.")
	}

	LogKnowledgeEvent("lint", "", fmt.Sprintf("total=%d orphan=%d unlinked=%d stale=%d similar=%d",
		report.TotalSolutions, report.OrphanSolutions, report.UnlinkedSolutions, report.StaleSolutions, len(report.SimilarPairs)))

	return report, nil
}

const maxSimilarPairs = 20

// findSimilarPairs finds solution pairs with very similar problem statements.
func findSimilarPairs() []SimilarPair {
	d := db.Get()
	rows, err := d.Query(`SELECT id, problem FROM solutions WHERE compacted=0 ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return make([]SimilarPair, 0)
	}
	defer rows.Close()

	type sol struct {
		id, problem string
		tokens      map[string]bool
	}
	var sols []sol
	for rows.Next() {
		var s sol
		if err := rows.Scan(&s.id, &s.problem); err != nil {
			continue
		}
		s.tokens = tokenizeText(s.problem)
		sols = append(sols, s)
	}

	out := make([]SimilarPair, 0)
	for i := 0; i < len(sols) && len(out) < maxSimilarPairs; i++ {
		for j := i + 1; j < len(sols) && len(out) < maxSimilarPairs; j++ {
			sim := jaccardSim(sols[i].tokens, sols[j].tokens)
			if sim >= 0.5 {
				out = append(out, SimilarPair{
					SolutionA:  sols[i].id,
					SolutionB:  sols[j].id,
					ProblemA:   sols[i].problem,
					ProblemB:   sols[j].problem,
					Similarity: sim,
				})
			}
		}
	}
	return out
}

// --- Drift Scan (entropy management) ---

// DriftReport summarizes entropy and drift in the knowledge base.
type DriftReport struct {
	DuplicateTreePairs []DuplicateTreePair `json:"duplicateTreePairs"`
	AbandonedWithValue []AbandonedTree     `json:"abandonedWithValue"`
	NeverRetrieved     []UnusedSolution    `json:"neverRetrieved"`
	Remediations       []Remediation       `json:"remediations"`
}

// DuplicateTreePair represents two trees with very similar problems.
type DuplicateTreePair struct {
	TreeA      string  `json:"treeA"`
	TreeB      string  `json:"treeB"`
	ProblemA   string  `json:"problemA"`
	ProblemB   string  `json:"problemB"`
	Similarity float64 `json:"similarity"`
}

// AbandonedTree represents an abandoned tree that has useful explored content.
type AbandonedTree struct {
	TreeID    string  `json:"treeId"`
	Problem   string  `json:"problem"`
	NodeCount int     `json:"nodeCount"`
	MaxScore  float64 `json:"maxScore"`
}

// UnusedSolution is a solution that has never been retrieved.
type UnusedSolution struct {
	ID        string `json:"id"`
	Problem   string `json:"problem"`
	CreatedAt string `json:"createdAt"`
	AgeDays   int    `json:"ageDays"`
}

// DriftScan detects knowledge entropy that accumulates over time.
func DriftScan() (*DriftReport, error) {
	report := &DriftReport{
		DuplicateTreePairs: make([]DuplicateTreePair, 0),
		AbandonedWithValue: make([]AbandonedTree, 0),
		NeverRetrieved:     make([]UnusedSolution, 0),
		Remediations:       make([]Remediation, 0),
	}

	d := db.Get()

	// 1. Duplicate trees: active/paused trees with very similar problems
	treeRows, err := d.Query(`SELECT id, problem FROM trees WHERE status IN ('active', 'paused') ORDER BY created_at DESC LIMIT 50`)
	if err == nil {
		defer treeRows.Close()
		type treeSol struct {
			id, problem string
			tokens      map[string]bool
		}
		var trees []treeSol
		for treeRows.Next() {
			var t treeSol
			if err := treeRows.Scan(&t.id, &t.problem); err != nil {
				continue
			}
			t.tokens = tokenizeText(t.problem)
			trees = append(trees, t)
		}
		for i := 0; i < len(trees); i++ {
			for j := i + 1; j < len(trees); j++ {
				sim := jaccardSim(trees[i].tokens, trees[j].tokens)
				if sim >= 0.4 {
					report.DuplicateTreePairs = append(report.DuplicateTreePairs, DuplicateTreePair{
						TreeA: trees[i].id, TreeB: trees[j].id,
						ProblemA: trees[i].problem, ProblemB: trees[j].problem,
						Similarity: sim,
					})
				}
			}
		}
	}

	// 2. Abandoned trees with useful content (high scores, many nodes)
	abandonedRows, err := d.Query(`SELECT t.id, t.problem, COUNT(n.id), COALESCE(MAX(n.score), 0)
		FROM trees t JOIN nodes n ON n.tree_id=t.id
		WHERE t.status='abandoned'
		GROUP BY t.id HAVING COUNT(n.id) >= 3 AND MAX(n.score) >= 0.5
		ORDER BY MAX(n.score) DESC LIMIT 10`)
	if err == nil {
		defer abandonedRows.Close()
		for abandonedRows.Next() {
			var a AbandonedTree
			if err := abandonedRows.Scan(&a.TreeID, &a.Problem, &a.NodeCount, &a.MaxScore); err != nil {
				continue
			}
			report.AbandonedWithValue = append(report.AbandonedWithValue, a)
		}
	}

	// 3. Never-retrieved solutions
	unusedRows, err := d.Query(`SELECT s.id, s.problem, s.created_at FROM solutions s
		WHERE NOT EXISTS (
			SELECT 1 FROM knowledge_log kl
			WHERE kl.event_type='retrieved' AND kl.detail LIKE '%%' || s.id || '%%'
		)
		ORDER BY s.created_at ASC LIMIT 10`)
	if err == nil {
		defer unusedRows.Close()
		for unusedRows.Next() {
			var u UnusedSolution
			if err := unusedRows.Scan(&u.ID, &u.Problem, &u.CreatedAt); err != nil {
				continue
			}
			created, _ := time.Parse(time.RFC3339, u.CreatedAt)
			u.AgeDays = int(time.Since(created).Hours() / 24)
			report.NeverRetrieved = append(report.NeverRetrieved, u)
		}
	}

	// Build remediations
	for _, dup := range report.DuplicateTreePairs {
		report.Remediations = append(report.Remediations, Remediation{
			Issue:  fmt.Sprintf("Trees %s and %s have %.0f%% similar problems", truncID(dup.TreeA), truncID(dup.TreeB), dup.Similarity*100),
			Action: "Consider linking these trees or abandoning the duplicate",
			Tool:   "link_trees",
			Args:   map[string]any{"source_tree": dup.TreeA, "target_tree": dup.TreeB, "link_type": "related"},
		})
	}
	for _, a := range report.AbandonedWithValue {
		report.Remediations = append(report.Remediations, Remediation{
			Issue:  fmt.Sprintf("Abandoned tree %s has %d nodes with max score %.2f", truncID(a.TreeID), a.NodeCount, a.MaxScore),
			Action: "Extract valuable insights. Resume or store the best path as a solution.",
			Tool:   "resume_tree",
			Args:   map[string]any{"tree_id": a.TreeID},
		})
	}

	LogKnowledgeEvent("drift_scan", "", fmt.Sprintf("dup_trees=%d abandoned_valuable=%d never_retrieved=%d",
		len(report.DuplicateTreePairs), len(report.AbandonedWithValue), len(report.NeverRetrieved)))

	return report, nil
}

// tokenizeText splits text into lowercase word tokens (>2 chars).
func tokenizeText(text string) map[string]bool {
	words := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(text)) {
		clean := strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(clean) > 2 {
			words[clean] = true
		}
	}
	return words
}

func truncID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// jaccardSim computes |intersection|/|union| of two word sets.
func jaccardSim(a, b map[string]bool) float64 {
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

