package resources

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/retrieval"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

var testTreeID string
var testSolID string

func TestMain(m *testing.M) {
	tmp := filepath.Join(os.TempDir(), "tot-mcp-resources-test.db")
	os.Remove(tmp)
	if _, err := db.Init(tmp); err != nil {
		panic(err)
	}
	// Create fixtures
	t, _, err := tree.CreateTree("resource test problem", "bfs", 5, 3)
	if err != nil {
		panic(err)
	}
	testTreeID = t.ID

	sid, err := retrieval.StoreSolution(testTreeID, "resource test problem", "test solution text", nil, nil, 0.9, []string{"test"})
	if err != nil {
		panic(err)
	}
	testSolID = sid

	code := m.Run()
	os.Remove(tmp)
	os.Exit(code)
}

func TestRegister(t *testing.T) {
	s := server.NewMCPServer("test", "0.0.0",
		server.WithResourceCapabilities(true, true),
	)
	Register(s)
	// If Register panics or errors, the test fails. No assertion needed.
}

func TestReadTreeList(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://trees"

	contents, err := readTreeList(context.Background(), req)
	if err != nil {
		t.Fatalf("readTreeList: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
	text, ok := contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("expected TextResourceContents, got %T", contents[0])
	}
	if text.MIMEType != "application/json" {
		t.Fatalf("MIME type: got %q", text.MIMEType)
	}
	if text.Text == "" || text.Text == "null" {
		t.Fatal("readTreeList returned empty content")
	}
}

func TestReadTree(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://tree/" + testTreeID
	req.Params.Arguments = map[string]any{"id": testTreeID}

	contents, err := readTree(context.Background(), req)
	if err != nil {
		t.Fatalf("readTree: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
	text := contents[0].(mcp.TextResourceContents)
	if text.Text == "" {
		t.Fatal("readTree returned empty content")
	}
}

func TestReadTreeMissingID(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://tree/"
	req.Params.Arguments = map[string]any{}

	_, err := readTree(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing tree ID")
	}
}

func TestReadTreeNotFound(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://tree/nonexistent"
	req.Params.Arguments = map[string]any{"id": "nonexistent"}

	_, err := readTree(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nonexistent tree")
	}
}

func TestReadFrontier(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://tree/" + testTreeID + "/frontier"
	req.Params.Arguments = map[string]any{"id": testTreeID}

	contents, err := readFrontier(context.Background(), req)
	if err != nil {
		t.Fatalf("readFrontier: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
}

func TestReadFrontierMissingID(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://tree//frontier"
	req.Params.Arguments = map[string]any{}

	_, err := readFrontier(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing tree ID")
	}
}

func TestReadExperiments(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://tree/" + testTreeID + "/experiments"
	req.Params.Arguments = map[string]any{"id": testTreeID}

	contents, err := readExperiments(context.Background(), req)
	if err != nil {
		t.Fatalf("readExperiments: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
}

func TestReadExperimentsMissingID(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://tree//experiments"
	req.Params.Arguments = map[string]any{}

	_, err := readExperiments(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing tree ID")
	}
}

func TestReadStatus(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://tree/" + testTreeID + "/status"
	req.Params.Arguments = map[string]any{"id": testTreeID}

	contents, err := readStatus(context.Background(), req)
	if err != nil {
		t.Fatalf("readStatus: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
}

func TestReadStatusNotFound(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://tree/nonexistent/status"
	req.Params.Arguments = map[string]any{"id": "nonexistent"}

	_, err := readStatus(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nonexistent tree")
	}
}

func TestReadSolutions(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://solutions"

	contents, err := readSolutions(context.Background(), req)
	if err != nil {
		t.Fatalf("readSolutions: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
}

func TestReadSolution(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://solution/" + testSolID
	req.Params.Arguments = map[string]any{"id": testSolID}

	contents, err := readSolution(context.Background(), req)
	if err != nil {
		t.Fatalf("readSolution: %v", err)
	}
	if len(contents) != 1 {
		t.Fatalf("expected 1 content block, got %d", len(contents))
	}
	text := contents[0].(mcp.TextResourceContents)
	if text.Text == "" {
		t.Fatal("readSolution returned empty content")
	}
}

func TestReadSolutionMissingID(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://solution/"
	req.Params.Arguments = map[string]any{}

	_, err := readSolution(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for missing solution ID")
	}
}

func TestReadSolutionNotFound(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.URI = "tot://solution/nonexistent"
	req.Params.Arguments = map[string]any{"id": "nonexistent"}

	_, err := readSolution(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for nonexistent solution")
	}
}

func TestArgStringNilArguments(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	// Params.Arguments is nil by default
	v := argString(req, "id")
	if v != "" {
		t.Fatalf("expected empty string for nil arguments, got %q", v)
	}
}

func TestArgStringMissingKey(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.Arguments = map[string]any{"other": "value"}
	v := argString(req, "id")
	if v != "" {
		t.Fatalf("expected empty string for missing key, got %q", v)
	}
}

func TestArgStringWrongType(t *testing.T) {
	req := mcp.ReadResourceRequest{}
	req.Params.Arguments = map[string]any{"id": 123}
	v := argString(req, "id")
	if v != "" {
		t.Fatalf("expected empty string for non-string value, got %q", v)
	}
}

func TestTextResourceHelper(t *testing.T) {
	result := textResource("tot://test", `{"key":"value"}`)
	if len(result) != 1 {
		t.Fatalf("expected 1 item, got %d", len(result))
	}
	text := result[0].(mcp.TextResourceContents)
	if text.URI != "tot://test" {
		t.Fatalf("URI: got %q", text.URI)
	}
	if text.MIMEType != "application/json" {
		t.Fatalf("MIME: got %q", text.MIMEType)
	}
	if text.Text != `{"key":"value"}` {
		t.Fatalf("Text: got %q", text.Text)
	}
}
