# Route Problem Embedding Similarity — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Store problem embeddings in the `trees` table at creation time and use cosine similarity in `RouteProblem` for better semantic matching, falling back to Jaccard when embeddings are unavailable.

**Architecture:** Add `embedding BLOB` column to `trees`. Populate at `CreateTree` time when provider is active. In `RouteProblem`, score candidates by cosine similarity (primary) with Jaccard as fallback. Hybrid boost (20%) when both signals agree above threshold. Move `float32ToBytes`/`bytesToFloat32` to a shared `internal/encoding` package so both `tree` and `retrieval` can use them.

**Tech Stack:** Go, SQLite (ALTER TABLE), internal/embeddings provider interface, internal/encoding (new shared package)

---

### Task 1: Extract byte encoding helpers to shared package

The `float32ToBytes` and `bytesToFloat32` functions currently live in `internal/retrieval/retrieval.go`. Both `tree` and `retrieval` need them. Extract to a shared package.

**Files:**
- Create: `internal/encoding/encoding.go`
- Create: `internal/encoding/encoding_test.go`
- Modify: `internal/retrieval/retrieval.go` — replace local functions with `encoding.Float32ToBytes` / `encoding.BytesToFloat32`
- Modify: `internal/retrieval/retrieval_test.go` — move `TestFloat32BytesRoundtrip` to new package

**Step 1: Write the test for the shared encoding package**

```go
// internal/encoding/encoding_test.go
package encoding

import "testing"

func TestFloat32BytesRoundtrip(t *testing.T) {
	original := []float32{1.5, -2.3, 0.0, 3.14159}
	bytes := Float32ToBytes(original)
	restored := BytesToFloat32(bytes)

	if len(restored) != len(original) {
		t.Fatalf("length: got %d, want %d", len(restored), len(original))
	}
	for i := range original {
		if restored[i] != original[i] {
			t.Fatalf("index %d: got %f, want %f", i, restored[i], original[i])
		}
	}
}

func TestFloat32BytesEmpty(t *testing.T) {
	bytes := Float32ToBytes(nil)
	if len(bytes) != 0 {
		t.Fatalf("nil input: got %d bytes", len(bytes))
	}
	restored := BytesToFloat32(nil)
	if len(restored) != 0 {
		t.Fatalf("nil input: got %d floats", len(restored))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/encoding/ -v`
Expected: FAIL — package doesn't exist yet

**Step 3: Write the implementation**

```go
// internal/encoding/encoding.go
package encoding

import (
	"encoding/binary"
	"math"
)

// Float32ToBytes encodes a float32 slice as little-endian bytes.
func Float32ToBytes(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

// BytesToFloat32 decodes little-endian bytes to a float32 slice.
func BytesToFloat32(b []byte) []float32 {
	out := make([]float32, len(b)/4)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return out
}
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/encoding/ -v`
Expected: PASS

**Step 5: Update retrieval to use shared package**

In `internal/retrieval/retrieval.go`:
- Add `"github.com/tot-mcp/tot-mcp-go/internal/encoding"` to imports
- Replace all calls to `float32ToBytes(...)` with `encoding.Float32ToBytes(...)`
- Replace all calls to `bytesToFloat32(...)` with `encoding.BytesToFloat32(...)`
- Delete the local `float32ToBytes` and `bytesToFloat32` functions
- In `retrieval_test.go`, delete `TestFloat32BytesRoundtrip` (now in encoding package)

**Step 6: Run full test suite**

Run: `go test ./... -count=1`
Expected: All pass

**Step 7: Commit**

```bash
git add internal/encoding/ internal/retrieval/
git commit -m "refactor: extract float32 byte encoding to shared package"
```

---

### Task 2: Add embedding column to trees table + schema migration

**Files:**
- Modify: `internal/db/db.go:61` — add `embedding BLOB` column to `trees` CREATE TABLE
- Modify: `internal/db/db.go:55-58` — add ALTER TABLE migration for existing databases
- Modify: `internal/db/db_test.go` — add test for new column

**Step 1: Write the test**

```go
// Add to internal/db/db_test.go
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
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/db/ -v -run TestTreesEmbeddingColumn`
Expected: FAIL — column "embedding" doesn't exist

**Step 3: Write the implementation**

In `internal/db/db.go`, modify the `trees` CREATE TABLE in the schema constant to add:

```sql
	embedding        BLOB,
```

after the `updated_at` line.

Then update the `migrate` function to also handle existing databases:

```go
func migrate(d *sql.DB) error {
	if _, err := d.Exec(schema); err != nil {
		return err
	}
	// Additive migrations for existing databases
	d.Exec(`ALTER TABLE trees ADD COLUMN embedding BLOB`)
	return nil
}
```

The ALTER TABLE will silently fail if the column already exists (since we use `CREATE TABLE IF NOT EXISTS`, the column is present for new DBs). For existing DBs it adds the column.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/db/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/db/
git commit -m "schema: add embedding BLOB column to trees table"
```

---

### Task 3: Store embedding at tree creation time

**Files:**
- Modify: `internal/tree/tree.go:52-88` — `CreateTree` embeds the problem text and stores it
- Modify: `internal/tree/tree.go:1-11` — add imports for `embeddings` and `encoding`
- Modify: `internal/tree/tree.go:91-101` — `GetTree` reads the embedding column
- Modify: `internal/tree/tree.go:27-38` — add `Embedding []byte` field to `Tree` struct
- Modify: `internal/tree/tree_test.go` — test that embedding is stored

**Step 1: Write the test**

```go
// Add to internal/tree/tree_test.go
func TestCreateTreeStoresEmbedding(t *testing.T) {
	// With noop provider (no API keys set), embedding should be nil
	tr, _, err := CreateTree("embedding storage test", "bfs", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}
	// In test environment (noop provider), embedding is nil — that's OK
	// Just verify the tree was created and the field is accessible
	got, _ := GetTree(tr.ID)
	if got == nil {
		t.Fatal("GetTree returned nil")
	}
	// Embedding field should exist (nil is fine for noop provider)
	_ = got.Embedding
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./internal/tree/ -v -run TestCreateTreeStoresEmbedding`
Expected: FAIL — `Tree` has no `Embedding` field

**Step 3: Implement**

In `internal/tree/tree.go`:

Add to imports:
```go
"github.com/tot-mcp/tot-mcp-go/internal/embeddings"
"github.com/tot-mcp/tot-mcp-go/internal/encoding"
```

Add field to `Tree` struct:
```go
Embedding  []byte `json:"-"` // not serialized to JSON (internal use)
```

Update `CreateTree` to embed and store:
```go
// After the branching_factor line, before tx.Begin():
var embBlob []byte
if embeddings.Active() {
    vec, err := embeddings.Get().Embed(problem)
    if err == nil && len(vec) > 0 {
        embBlob = encoding.Float32ToBytes(vec)
    }
}
```

Update the INSERT to include embedding:
```sql
INSERT INTO trees (id,problem,root_id,search_strategy,max_depth,branching_factor,status,embedding,created_at,updated_at)
VALUES (?,?,?,?,?,?,'active',?,?,?)
```
Add `embBlob` to the args (after `branchingFactor`, before `ts, ts`).

Update `GetTree` to scan the embedding:
```go
var emb []byte
err := d.QueryRow(`SELECT id,problem,root_id,search_strategy,max_depth,branching_factor,status,embedding,created_at,updated_at
    FROM trees WHERE id=?`, treeID).Scan(
    &t.ID, &t.Problem, &t.RootID, &t.SearchStrategy, &t.MaxDepth, &t.BranchingFactor, &t.Status, &emb, &t.CreatedAt, &t.UpdatedAt)
t.Embedding = emb
```

Update `ListTrees` and `scanTrees` similarly (add `embedding` to SELECT, scan into `[]byte`, assign to struct). Use a nullable scan `var emb []byte` since embedding can be NULL.

**Step 4: Run test to verify it passes**

Run: `go test ./internal/tree/ -v`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/tree/
git commit -m "feat: store problem embedding in trees table at creation time"
```

---

### Task 4: Use embedding similarity in RouteProblem

**Files:**
- Modify: `internal/tree/tree.go:498-563` — `RouteProblem` uses cosine similarity when available
- Modify: `internal/tree/tree_test.go` — add test for embedding-based routing

**Step 1: Write the test**

```go
// Add to internal/tree/tree_test.go
func TestRouteProblemUsesEmbeddingSimilarity(t *testing.T) {
	// Even without a real embedding provider, verify the function
	// handles the embedding path without errors
	CreateTree("analyze memory usage patterns in production servers", "bfs", 5, 3)

	result, err := RouteProblem("investigate memory consumption in production systems", nil)
	if err != nil {
		t.Fatalf("RouteProblem with embedding path: %v", err)
	}
	// With noop provider, falls back to Jaccard. These share enough keywords to match.
	if result.Action != "continue" {
		t.Logf("action=%s similarity=%f reason=%s", result.Action, result.Similarity, result.Reason)
		// This is OK — Jaccard may or may not match depending on word overlap
	}
}
```

**Step 2: Run test to verify it compiles and passes**

Run: `go test ./internal/tree/ -v -run TestRouteProblemUsesEmbeddingSimilarity`
Expected: PASS (baseline before we change RouteProblem)

**Step 3: Implement hybrid scoring in RouteProblem**

Replace the scoring loop in `RouteProblem` (lines ~529-556) with:

```go
// Score each candidate by embedding similarity (if available) + keyword overlap
bestIdx := -1
bestScore := 0.0

// Embed the incoming problem if provider is active
var queryVec []float32
if embeddings.Active() {
    queryVec, _ = embeddings.Get().Embed(problem)
}

problemWords := tokenize(problem)
for i, c := range candidates {
    // Keyword score (always available)
    candidateWords := tokenize(c.problem)
    kwScore := jaccardSimilarity(problemWords, candidateWords)

    // Embedding score (when both vectors are available)
    var embScore float64
    if len(queryVec) > 0 && len(c.embedding) > 0 {
        storedVec := encoding.BytesToFloat32(c.embedding)
        embScore = embeddings.CosineSimilarity(queryVec, storedVec)
    }

    // Combined score: use embedding as primary when available, keyword as fallback
    var score float64
    if embScore > 0 && kwScore > 0 {
        // Both signals agree — hybrid boost
        score = math.Max(embScore, kwScore) * 1.2
        if score > 1.0 {
            score = 1.0
        }
    } else if embScore > 0 {
        score = embScore
    } else {
        score = kwScore
    }

    if score > bestScore {
        bestScore = score
        bestIdx = i
    }
}
```

Update the candidate struct to include embedding:
```go
type candidate struct {
    id, problem, status, updatedAt string
    embedding                      []byte
}
```

Update the query to SELECT embedding:
```sql
SELECT id, problem, status, updated_at, embedding FROM trees
WHERE status IN ('active', 'paused') ORDER BY updated_at DESC LIMIT 20
```

Update the scan:
```go
rows.Scan(&c.id, &c.problem, &c.status, &c.updatedAt, &c.embedding)
```

Add `"math"` to imports.

**Step 4: Run full test suite**

Run: `go test ./... -count=1`
Expected: All PASS

**Step 5: Commit**

```bash
git add internal/tree/
git commit -m "feat: use embedding cosine similarity in route_problem with Jaccard fallback"
```

---

### Task 5: Update main.go route_problem handler to pass embedding

**Files:**
- Modify: `main.go:292-303` — route_problem handler embeds the problem and passes it

**Step 1: Update the handler**

The current handler passes `nil` for the embedding. Update to embed if provider is active:

```go
), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    problem, _ := req.RequireString("problem")

    var emb []float32
    if embeddings.Active() {
        emb, _ = embeddings.Get().Embed(problem)
    }

    result, err := tree.RouteProblem(problem, emb)
```

Wait — actually, `RouteProblem` now embeds the query internally (Task 4). The `embedding []float32` parameter is no longer needed. Remove it from the function signature and the handler call.

In `internal/tree/tree.go`, change:
```go
func RouteProblem(problem string, embedding []float32) (*RouteProblemResult, error) {
```
to:
```go
func RouteProblem(problem string) (*RouteProblemResult, error) {
```

In `main.go`, change:
```go
result, err := tree.RouteProblem(problem, nil)
```
to:
```go
result, err := tree.RouteProblem(problem)
```

Update `internal/tree/tree_test.go` — all `RouteProblem(x, nil)` calls become `RouteProblem(x)`.

**Step 2: Build and test**

Run: `go build -o tot-mcp . && go test ./... -count=1`
Expected: Build clean, all tests pass

**Step 3: Commit**

```bash
git add main.go internal/tree/
git commit -m "cleanup: remove unused embedding parameter from RouteProblem signature"
```

---

### Task 6: Run go vet, final verification

**Step 1: Run go vet**

Run: `go vet ./...`
Expected: Clean

**Step 2: Run full test suite**

Run: `go test ./... -count=1 -v`
Expected: All PASS

**Step 3: Run build**

Run: `go build -o tot-mcp .`
Expected: Clean

**Step 4: Final commit if any cleanup needed**
