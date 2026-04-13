package resources

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/experiment"
	"github.com/tot-mcp/tot-mcp-go/internal/retrieval"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

// Register adds all resource templates to the MCP server.
func Register(s *server.MCPServer) {
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://trees", "List all reasoning trees"),
		readTreeList,
	)
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://tree/{id}", "Full tree state with nodes, stats, and best path"),
		readTree,
	)
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://tree/{id}/frontier", "Frontier nodes available for expansion"),
		readFrontier,
	)
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://tree/{id}/experiments", "Experiment history and metrics"),
		readExperiments,
	)
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://tree/{id}/status", "Tree lifecycle status"),
		readStatus,
	)
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://solutions", "Retrieval store overview"),
		readSolutions,
	)
	s.AddResourceTemplate(
		mcp.NewResourceTemplate("tot://solution/{id}", "Single solution detail"),
		readSolution,
	)
}

func j(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

func argString(req mcp.ReadResourceRequest, key string) string {
	if req.Params.Arguments == nil {
		return ""
	}
	v, ok := req.Params.Arguments[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func textResource(uri, content string) []mcp.ResourceContents {
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "application/json",
			Text:     content,
		},
	}
}

func readTreeList(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	trees, err := tree.ListTrees()
	if err != nil {
		return nil, err
	}
	items := make([]map[string]any, len(trees))
	for i, t := range trees {
		items[i] = map[string]any{
			"id": t.ID, "problem": t.Problem, "status": t.Status,
			"strategy": t.SearchStrategy, "nodeCount": tree.NodeCount(t.ID),
		}
	}
	return textResource(req.Params.URI, j(items)), nil
}

func readTree(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	treeID := argString(req, "id")
	if treeID == "" {
		return nil, fmt.Errorf("missing tree ID")
	}
	t, err := tree.GetTree(treeID)
	if err != nil {
		return nil, err
	}
	summary, _ := tree.Summary(treeID)
	bestPath, _ := tree.GetBestPath(treeID)
	data := map[string]any{"tree": t, "summary": summary, "bestPath": bestPath}
	return textResource(req.Params.URI, j(data)), nil
}

func readFrontier(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	treeID := argString(req, "id")
	if treeID == "" {
		return nil, fmt.Errorf("missing tree ID")
	}
	nodes, err := tree.GetFrontier(treeID)
	if err != nil {
		return nil, err
	}
	return textResource(req.Params.URI, j(nodes)), nil
}

func readExperiments(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	treeID := argString(req, "id")
	if treeID == "" {
		return nil, fmt.Errorf("missing tree ID")
	}
	history := experiment.History(treeID)
	return textResource(req.Params.URI, j(history)), nil
}

func readStatus(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	treeID := argString(req, "id")
	if treeID == "" {
		return nil, fmt.Errorf("missing tree ID")
	}
	t, err := tree.GetTree(treeID)
	if err != nil {
		return nil, err
	}
	data := map[string]any{"treeId": treeID, "status": t.Status, "updatedAt": t.UpdatedAt}
	return textResource(req.Params.URI, j(data)), nil
}

func readSolutions(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	stats := retrieval.Stats()
	return textResource(req.Params.URI, j(stats)), nil
}

func readSolution(_ context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	solID := argString(req, "id")
	if solID == "" {
		return nil, fmt.Errorf("missing solution ID")
	}
	d := db.Get()
	var prob, sol, tags, created string
	var score float64
	err := d.QueryRow(`SELECT problem, solution, tags, score, created_at FROM solutions WHERE id=?`, solID).
		Scan(&prob, &sol, &tags, &score, &created)
	if err != nil {
		return nil, fmt.Errorf("solution not found")
	}
	data := map[string]any{"id": solID, "problem": prob, "solution": sol, "tags": tags, "score": score, "createdAt": created}
	return textResource(req.Params.URI, j(data)), nil
}
