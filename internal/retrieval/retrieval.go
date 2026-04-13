package retrieval

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/embeddings"
	"github.com/tot-mcp/tot-mcp-go/internal/encoding"
	"github.com/tot-mcp/tot-mcp-go/internal/events"
)

// Result is a retrieved solution with match info.
type Result struct {
	ID         string   `json:"id"`
	Problem    string   `json:"problem"`
	Solution   string   `json:"solution"`
	Thoughts   []string `json:"thoughts"`
	Score      float64  `json:"score"`
	Tags       []string `json:"tags"`
	Rationale  string   `json:"rationale,omitempty"`
	Similarity float64  `json:"similarity"`
	MatchType  string   `json:"matchType"`
}

// StoreSolution saves a solution with an optional embedding.
// rationale is optional — pass "" to skip.
func StoreSolution(treeID, problem, solution string, thoughts, pathIDs []string, score float64, tags []string, rationale ...string) (string, error) {
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

	rat := ""
	if len(rationale) > 0 {
		rat = rationale[0]
	}

	_, err := d.Exec(`INSERT INTO solutions (id,tree_id,problem,solution,thoughts,path_ids,score,tags,rationale,embedding,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, treeID, problem, solution, string(thoughtsJSON), string(pathJSON), score, string(tagsJSON), rat, embBlob, ts)
	if err == nil {
		autoLinkRelated(id, problem)
		LogKnowledgeEvent("stored", id, fmt.Sprintf("tree=%s tags=%v", treeID, tags))
		events.Get().Publish(events.Event{
			Type: events.SolutionStored, TreeID: treeID,
			Timestamp: time.Now(),
			Payload: map[string]any{"solutionId": id, "tags": tags},
		})
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
		if _, err := LinkSolutionsWithConfidence(newID, r.ID, "related", "auto-linked on store", r.Similarity, "auto"); err != nil {
			log.Printf("autoLinkRelated: link failed: %v", err)
		}
	}
}

// Retrieve performs hybrid vector + keyword search.
// maxTokens: if > 0, truncates solution text to fit within approximate token budget.
func Retrieve(query string, topK int, filterTags []string, maxTokens ...int) ([]Result, error) {
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
	// Apply token budget if specified
	if len(maxTokens) > 0 && maxTokens[0] > 0 {
		budget := maxTokens[0]
		sorted = truncateResults(sorted, budget)
	}

	if len(sorted) > 0 {
		LogKnowledgeEvent("retrieved", "", fmt.Sprintf("matched=%d", len(sorted)))
	}
	return sorted, nil
}

// truncateResults trims solution text to fit within an approximate token budget.
// Tokens are estimated at ~4 chars per token.
func truncateResults(results []Result, maxTokens int) []Result {
	charsPerToken := 4
	totalBudget := maxTokens * charsPerToken
	used := 0

	out := make([]Result, 0, len(results))
	for _, r := range results {
		size := len(r.Problem) + len(r.Solution)
		for _, t := range r.Thoughts {
			size += len(t)
		}
		if used+size > totalBudget && len(out) > 0 {
			break
		}
		// If single result exceeds budget, truncate its solution
		if size > totalBudget {
			maxSol := totalBudget - len(r.Problem) - 100 // reserve space for problem + overhead
			if maxSol > 0 && len(r.Solution) > maxSol {
				r.Solution = r.Solution[:maxSol] + "... (truncated)"
			}
			r.Thoughts = nil // drop thoughts to save tokens
		}
		out = append(out, r)
		used += size
	}
	return out
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

	if err := tx.Commit(); err != nil {
		return err
	}
	events.Get().Publish(events.Event{
		Type:      events.SolutionCompacted,
		Timestamp: time.Now(),
		Payload:   map[string]any{"solutionId": solutionID},
	})
	return nil
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
	ID         string  `json:"id"`
	SourceID   string  `json:"sourceId"`
	TargetID   string  `json:"targetId"`
	LinkType   string  `json:"linkType"`
	Note       string  `json:"note"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"` // "manual" or "auto"
	CreatedAt  string  `json:"createdAt"`
}

var validSolutionLinkTypes = map[string]bool{
	"related": true, "supersedes": true, "contradicts": true, "extends": true,
}

// LinkSolutions creates a cross-reference between two solutions.
// confidence: 0.0-1.0 (default 1.0 for manual links)
// source: "manual" or "auto"
func LinkSolutions(sourceID, targetID, linkType, note string) (*SolutionLink, error) {
	return LinkSolutionsWithConfidence(sourceID, targetID, linkType, note, 1.0, "manual")
}

// LinkSolutionsWithConfidence creates a link with explicit confidence and source.
func LinkSolutionsWithConfidence(sourceID, targetID, linkType, note string, confidence float64, source string) (*SolutionLink, error) {
	if sourceID == targetID {
		return nil, fmt.Errorf("cannot link a solution to itself")
	}
	if !validSolutionLinkTypes[linkType] {
		return nil, fmt.Errorf("invalid link type %q (valid: related, supersedes, contradicts, extends)", linkType)
	}

	d := db.Get()
	id := uuid.NewString()
	ts := time.Now().UTC().Format(time.RFC3339)

	res, err := d.Exec(`INSERT OR IGNORE INTO solution_links (id,source_id,target_id,link_type,note,confidence,source,created_at)
		VALUES (?,?,?,?,?,?,?,?)`, id, sourceID, targetID, linkType, note, confidence, source, ts)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return &SolutionLink{
			ID: id, SourceID: sourceID, TargetID: targetID,
			LinkType: linkType, Note: note, Confidence: confidence,
			Source: source, CreatedAt: ts,
		}, nil
	}

	LogKnowledgeEvent("linked", sourceID, fmt.Sprintf("%s -> %s (%s conf=%.2f)", truncID(sourceID), truncID(targetID), linkType, confidence))
	events.Get().Publish(events.Event{
		Type:      events.SolutionLinked,
		Timestamp: time.Now(),
		Payload:   map[string]any{"sourceId": sourceID, "targetId": targetID, "linkType": linkType, "confidence": confidence},
	})

	return &SolutionLink{
		ID: id, SourceID: sourceID, TargetID: targetID,
		LinkType: linkType, Note: note, Confidence: confidence,
		Source: source, CreatedAt: ts,
	}, nil
}

// GetSolutionLinks returns all links where the given solution is source or target.
func GetSolutionLinks(solutionID string) ([]SolutionLink, error) {
	d := db.Get()
	rows, err := d.Query(`SELECT id,source_id,target_id,link_type,note,confidence,source,created_at
		FROM solution_links WHERE source_id=? OR target_id=? ORDER BY created_at DESC`,
		solutionID, solutionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]SolutionLink, 0)
	for rows.Next() {
		var l SolutionLink
		if err := rows.Scan(&l.ID, &l.SourceID, &l.TargetID, &l.LinkType, &l.Note, &l.Confidence, &l.Source, &l.CreatedAt); err != nil {
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

// --- Graph Topology Analysis ---

// GraphAnalysis contains topology metrics for the knowledge graph.
type GraphAnalysis struct {
	TotalNodes  int            `json:"totalNodes"`
	TotalEdges  int            `json:"totalEdges"`
	GodNodes    []GodNode      `json:"godNodes"`
	Communities []Community    `json:"communities"`
	Bridges     []BridgeEdge   `json:"bridges"`
}

// GodNode is a solution with the highest link degree (most connected).
type GodNode struct {
	SolutionID string `json:"solutionId"`
	Problem    string `json:"problem"`
	Degree     int    `json:"degree"`
}

// Community is a cluster of connected solutions.
type Community struct {
	ID        int      `json:"id"`
	Solutions []string `json:"solutions"`
	Tags      []string `json:"tags"`
	Size      int      `json:"size"`
}

// BridgeEdge is a link connecting solutions in different communities.
type BridgeEdge struct {
	SourceID    string `json:"sourceId"`
	TargetID    string `json:"targetId"`
	LinkType    string `json:"linkType"`
	CommunityA  int   `json:"communityA"`
	CommunityB  int   `json:"communityB"`
}

// AnalyzeKnowledgeGraph computes topology metrics on the solution link graph.
func AnalyzeKnowledgeGraph() (*GraphAnalysis, error) {
	d := db.Get()
	analysis := &GraphAnalysis{
		GodNodes:    make([]GodNode, 0),
		Communities: make([]Community, 0),
		Bridges:     make([]BridgeEdge, 0),
	}

	// Count nodes and edges
	d.QueryRow(`SELECT COUNT(*) FROM solutions`).Scan(&analysis.TotalNodes)
	d.QueryRow(`SELECT COUNT(*) FROM solution_links`).Scan(&analysis.TotalEdges)

	if analysis.TotalNodes == 0 {
		return analysis, nil
	}

	// God nodes: solutions with highest link degree
	godRows, err := d.Query(`SELECT s.id, s.problem, COUNT(*) as degree
		FROM solutions s
		JOIN solution_links sl ON sl.source_id=s.id OR sl.target_id=s.id
		GROUP BY s.id ORDER BY degree DESC LIMIT 10`)
	if err == nil {
		defer godRows.Close()
		for godRows.Next() {
			var g GodNode
			if err := godRows.Scan(&g.SolutionID, &g.Problem, &g.Degree); err != nil {
				continue
			}
			analysis.GodNodes = append(analysis.GodNodes, g)
		}
	}

	// Communities via connected components (BFS on solution_links)
	// Build adjacency list
	adjRows, err := d.Query(`SELECT source_id, target_id FROM solution_links`)
	if err != nil {
		return analysis, nil
	}
	defer adjRows.Close()

	adj := map[string][]string{}
	allNodes := map[string]bool{}
	for adjRows.Next() {
		var src, tgt string
		adjRows.Scan(&src, &tgt)
		adj[src] = append(adj[src], tgt)
		adj[tgt] = append(adj[tgt], src)
		allNodes[src] = true
		allNodes[tgt] = true
	}

	// Also include unlinked solutions as isolated nodes
	isolatedRows, err := d.Query(`SELECT id FROM solutions WHERE id NOT IN (
		SELECT source_id FROM solution_links UNION SELECT target_id FROM solution_links)`)
	if err == nil {
		defer isolatedRows.Close()
		for isolatedRows.Next() {
			var id string
			isolatedRows.Scan(&id)
			allNodes[id] = true
		}
	}

	// BFS connected components
	visited := map[string]bool{}
	communityOf := map[string]int{}
	communityID := 0
	for node := range allNodes {
		if visited[node] {
			continue
		}
		// BFS
		queue := []string{node}
		var members []string
		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			if visited[curr] {
				continue
			}
			visited[curr] = true
			communityOf[curr] = communityID
			members = append(members, curr)
			for _, neighbor := range adj[curr] {
				if !visited[neighbor] {
					queue = append(queue, neighbor)
				}
			}
		}
		if len(members) > 0 {
			// Get common tags for this community
			tags := communityTags(d, members)
			analysis.Communities = append(analysis.Communities, Community{
				ID: communityID, Solutions: members, Tags: tags, Size: len(members),
			})
			communityID++
		}
	}

	// Bridges: links between different communities
	linkRows, err := d.Query(`SELECT source_id, target_id, link_type FROM solution_links`)
	if err == nil {
		defer linkRows.Close()
		for linkRows.Next() {
			var src, tgt, lt string
			linkRows.Scan(&src, &tgt, &lt)
			ca, cb := communityOf[src], communityOf[tgt]
			if ca != cb {
				analysis.Bridges = append(analysis.Bridges, BridgeEdge{
					SourceID: src, TargetID: tgt, LinkType: lt,
					CommunityA: ca, CommunityB: cb,
				})
			}
		}
	}

	// Sort communities by size descending
	for i := 1; i < len(analysis.Communities); i++ {
		for j := i; j > 0 && analysis.Communities[j].Size > analysis.Communities[j-1].Size; j-- {
			analysis.Communities[j], analysis.Communities[j-1] = analysis.Communities[j-1], analysis.Communities[j]
		}
	}

	LogKnowledgeEvent("graph_analysis", "", fmt.Sprintf("nodes=%d edges=%d communities=%d gods=%d bridges=%d",
		analysis.TotalNodes, analysis.TotalEdges, len(analysis.Communities), len(analysis.GodNodes), len(analysis.Bridges)))

	return analysis, nil
}

// communityTags finds the most common tags among a set of solution IDs.
func communityTags(d *sql.DB, solutionIDs []string) []string {
	tagCount := map[string]int{}
	for _, id := range solutionIDs {
		var tagsStr string
		d.QueryRow(`SELECT tags FROM solutions WHERE id=?`, id).Scan(&tagsStr)
		var tags []string
		json.Unmarshal([]byte(tagsStr), &tags)
		for _, t := range tags {
			tagCount[t]++
		}
	}
	// Return tags that appear in >50% of solutions, or top 3
	var result []string
	for tag, count := range tagCount {
		if count > len(solutionIDs)/2 || len(result) < 3 {
			result = append(result, tag)
		}
	}
	return result
}

// --- Knowledge Report ---

// KnowledgeReportData is a structured overview of the knowledge base,
// designed as the "map" agents read before querying.
type KnowledgeReportData struct {
	TopSolutions    []GodNode     `json:"topSolutions"`
	TagCoverage     []TagCoverage `json:"tagCoverage"`
	RecentEvents    []KnowledgeEvent `json:"recentEvents"`
	GraphSummary    GraphSummary  `json:"graphSummary"`
	SuggestedQueries []string     `json:"suggestedQueries"`
}

// TagCoverage shows how many solutions exist per tag.
type TagCoverage struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// GraphSummary is a compact overview of graph structure.
type GraphSummary struct {
	TotalSolutions int `json:"totalSolutions"`
	TotalLinks     int `json:"totalLinks"`
	Communities    int `json:"communities"`
	Bridges        int `json:"bridges"`
}

// KnowledgeReport generates a structured overview of the knowledge base.
func KnowledgeReport() (*KnowledgeReportData, error) {
	report := &KnowledgeReportData{
		TopSolutions:     make([]GodNode, 0),
		TagCoverage:      make([]TagCoverage, 0),
		RecentEvents:     make([]KnowledgeEvent, 0),
		SuggestedQueries: make([]string, 0),
	}

	// Graph analysis for god nodes and structure
	analysis, err := AnalyzeKnowledgeGraph()
	if err != nil {
		return report, nil
	}

	// Top 5 god nodes
	limit := 5
	if len(analysis.GodNodes) < limit {
		limit = len(analysis.GodNodes)
	}
	report.TopSolutions = analysis.GodNodes[:limit]

	report.GraphSummary = GraphSummary{
		TotalSolutions: analysis.TotalNodes,
		TotalLinks:     analysis.TotalEdges,
		Communities:    len(analysis.Communities),
		Bridges:        len(analysis.Bridges),
	}

	// Tag coverage
	d := db.Get()
	tagRows, err := d.Query(`SELECT tags FROM solutions WHERE compacted=0`)
	if err == nil {
		defer tagRows.Close()
		tagCount := map[string]int{}
		for tagRows.Next() {
			var tagsStr string
			tagRows.Scan(&tagsStr)
			var tags []string
			json.Unmarshal([]byte(tagsStr), &tags)
			for _, t := range tags {
				tagCount[t]++
			}
		}
		for tag, count := range tagCount {
			report.TagCoverage = append(report.TagCoverage, TagCoverage{Tag: tag, Count: count})
		}
		// Sort by count descending
		for i := 1; i < len(report.TagCoverage); i++ {
			for j := i; j > 0 && report.TagCoverage[j].Count > report.TagCoverage[j-1].Count; j-- {
				report.TagCoverage[j], report.TagCoverage[j-1] = report.TagCoverage[j-1], report.TagCoverage[j]
			}
		}
	}

	// Recent events
	report.RecentEvents, _ = GetKnowledgeLog(10)

	// Suggested queries based on god nodes
	for _, g := range report.TopSolutions {
		report.SuggestedQueries = append(report.SuggestedQueries,
			fmt.Sprintf("How does '%s' relate to other solutions?", truncate(g.Problem, 60)))
	}

	LogKnowledgeEvent("report", "", fmt.Sprintf("solutions=%d tags=%d", report.GraphSummary.TotalSolutions, len(report.TagCoverage)))

	return report, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
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

// --- Obsidian Export ---

// ExportObsidian generates an Obsidian vault with interlinked markdown files.
func ExportObsidian(outDir string) error {
	d := db.Get()

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	// Get all solutions
	rows, err := d.Query(`SELECT id, problem, solution, thoughts, tags, score, rationale, created_at FROM solutions ORDER BY created_at DESC`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type solData struct {
		id, problem, solution, rationale, createdAt string
		thoughts, tags                              []string
		score                                       float64
	}
	var solutions []solData
	for rows.Next() {
		var s solData
		var thoughtsStr, tagsStr string
		rows.Scan(&s.id, &s.problem, &s.solution, &thoughtsStr, &tagsStr, &s.score, &s.rationale, &s.createdAt)
		json.Unmarshal([]byte(thoughtsStr), &s.thoughts)
		json.Unmarshal([]byte(tagsStr), &s.tags)
		solutions = append(solutions, s)
	}

	// Get all links for cross-referencing
	linkRows, err := d.Query(`SELECT source_id, target_id, link_type FROM solution_links`)
	if err != nil {
		return err
	}
	defer linkRows.Close()

	links := map[string][]struct{ target, linkType string }{}
	for linkRows.Next() {
		var src, tgt, lt string
		linkRows.Scan(&src, &tgt, &lt)
		links[src] = append(links[src], struct{ target, linkType string }{tgt, lt})
		links[tgt] = append(links[tgt], struct{ target, linkType string }{src, lt})
	}

	// Write each solution as a markdown file
	for _, s := range solutions {
		filename := sanitizeFilename(s.problem) + ".md"
		var content strings.Builder

		// YAML frontmatter
		content.WriteString("---\n")
		fmt.Fprintf(&content, "id: %s\n", s.id)
		fmt.Fprintf(&content, "score: %.2f\n", s.score)
		if len(s.tags) > 0 {
			content.WriteString("tags:\n")
			for _, t := range s.tags {
				fmt.Fprintf(&content, "  - %s\n", t)
			}
		}
		fmt.Fprintf(&content, "created: %s\n", s.createdAt)
		content.WriteString("---\n\n")

		// Content
		fmt.Fprintf(&content, "# %s\n\n", s.problem)
		fmt.Fprintf(&content, "## Solution\n\n%s\n\n", s.solution)

		if s.rationale != "" {
			fmt.Fprintf(&content, "## Rationale\n\n%s\n\n", s.rationale)
		}

		if len(s.thoughts) > 0 {
			content.WriteString("## Reasoning Path\n\n")
			for i, t := range s.thoughts {
				fmt.Fprintf(&content, "%d. %s\n", i+1, t)
			}
			content.WriteString("\n")
		}

		// Wiki-links to related solutions
		if related, ok := links[s.id]; ok && len(related) > 0 {
			content.WriteString("## Related Solutions\n\n")
			for _, r := range related {
				// Find the linked solution's problem text for display
				var linkedProblem string
				for _, sol := range solutions {
					if sol.id == r.target {
						linkedProblem = sol.problem
						break
					}
				}
				if linkedProblem != "" {
					fmt.Fprintf(&content, "- %s: [[%s]]\n", r.linkType, sanitizeFilename(linkedProblem))
				}
			}
			content.WriteString("\n")
		}

		if err := os.WriteFile(filepath.Join(outDir, filename), []byte(content.String()), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", filename, err)
		}
	}

	// Write index page
	var index strings.Builder
	index.WriteString("# Knowledge Base Index\n\n")
	// Group by tags
	tagSolutions := map[string][]string{}
	for _, s := range solutions {
		for _, t := range s.tags {
			tagSolutions[t] = append(tagSolutions[t], sanitizeFilename(s.problem))
		}
		if len(s.tags) == 0 {
			tagSolutions["untagged"] = append(tagSolutions["untagged"], sanitizeFilename(s.problem))
		}
	}
	for tag, sols := range tagSolutions {
		fmt.Fprintf(&index, "## %s\n\n", tag)
		for _, sol := range sols {
			fmt.Fprintf(&index, "- [[%s]]\n", sol)
		}
		index.WriteString("\n")
	}

	if err := os.WriteFile(filepath.Join(outDir, "Index.md"), []byte(index.String()), 0o644); err != nil {
		return fmt.Errorf("write Index.md: %w", err)
	}

	LogKnowledgeEvent("export_obsidian", "", fmt.Sprintf("solutions=%d dir=%s", len(solutions), outDir))

	return nil
}

// sanitizeFilename creates a safe filename from a problem statement.
func sanitizeFilename(s string) string {
	// Replace non-alphanumeric chars with hyphens, trim, limit length
	var result []byte
	for _, c := range []byte(strings.ToLower(s)) {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			result = append(result, c)
		} else if len(result) > 0 && result[len(result)-1] != '-' {
			result = append(result, '-')
		}
	}
	// Trim trailing hyphens
	for len(result) > 0 && result[len(result)-1] == '-' {
		result = result[:len(result)-1]
	}
	if len(result) > 80 {
		result = result[:80]
	}
	if len(result) == 0 {
		return "untitled"
	}
	return string(result)
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

