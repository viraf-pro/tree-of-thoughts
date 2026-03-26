package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/dashboard"
	"github.com/tot-mcp/tot-mcp-go/internal/embeddings"
	"github.com/tot-mcp/tot-mcp-go/internal/experiment"
	"github.com/tot-mcp/tot-mcp-go/internal/retrieval"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

func main() {
	if _, err := db.Init(os.Getenv("TOT_DB_PATH")); err != nil {
		log.Fatal(err)
	}

	// Initialize embedding provider (local on-device is the default)
	ep := embeddings.Get()
	if embeddings.Active() {
		log.Printf("Embedding provider: %T (%d dimensions)", ep, ep.Dimensions())
	} else {
		log.Printf("Embedding provider: none (FTS5 keyword search only)")
	}

	// Start dashboard HTTP server
	port := 4545
	if p := os.Getenv("TOT_DASHBOARD_PORT"); p != "" {
		fmt.Sscanf(p, "%d", &port)
	}
	if os.Getenv("TOT_NO_DASHBOARD") == "" {
		url, err := dashboard.Start(port)
		if err != nil {
			log.Printf("Dashboard failed to start: %v", err)
		} else {
			log.Printf("Dashboard: %s", url)
			dashboardURL = url
		}
	}

	s := server.NewMCPServer("tot-mcp-server", "2.0.0", server.WithToolCapabilities(false))

	registerTreeTools(s)
	registerRetrievalTools(s)
	registerExperimentTools(s)

	// dashboard tool
	s.AddTool(mcp.NewTool("open_dashboard",
		mcp.WithDescription("Get the URL for the live visual dashboard. Opens in a browser. Shows the reasoning tree, experiment history, metric chart, and retrieval store."),
		mcp.WithString("tree_id", mcp.Description("Optional tree ID to link directly to")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if dashboardURL == "" {
			return mcp.NewToolResultText("Dashboard is not running. Set TOT_DASHBOARD_PORT or remove TOT_NO_DASHBOARD to enable."), nil
		}
		tid := optString(req, "tree_id", "")
		url := dashboardURL
		if tid != "" {
			url += "#tree/" + tid
		}
		return textResult(map[string]any{
			"message":      "Dashboard is live. Open this URL in your browser.",
			"url":          url,
			"autoRefresh":  "every 10 seconds",
		}), nil
	})

	if err := server.ServeStdio(s); err != nil {
		log.Fatal(err)
	}

	// Graceful shutdown: clean up local embedding provider if active
	if lp, ok := ep.(*embeddings.LocalProvider); ok {
		lp.Destroy()
	}
}

func j(v any) string { b, _ := json.MarshalIndent(v, "", "  "); return string(b) }

func textResult(v any) *mcp.CallToolResult { return mcp.NewToolResultText(j(v)) }

var dashboardURL string

// ============================================================================
// Tree tools
// ============================================================================

func registerTreeTools(s *server.MCPServer) {
	// create_tree
	s.AddTool(mcp.NewTool("create_tree",
		mcp.WithDescription("Initialize a new Tree of Thoughts for a problem."),
		mcp.WithString("problem", mcp.Required(), mcp.Description("Problem statement")),
		mcp.WithString("search_strategy", mcp.Description("bfs, dfs, or beam"), mcp.DefaultString("bfs")),
		mcp.WithNumber("max_depth", mcp.Description("Max tree depth"), mcp.DefaultNumber(5)),
		mcp.WithNumber("branching_factor", mcp.Description("Candidates per node"), mcp.DefaultNumber(3)),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		problem, _ := req.RequireString("problem")
		strategy := optString(req, "search_strategy", "bfs")
		maxD := optInt(req, "max_depth", 5)
		bf := optInt(req, "branching_factor", 3)

		t, root, err := tree.CreateTree(problem, strategy, maxD, bf)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(map[string]any{"message": "Tree created.", "treeId": t.ID, "rootNode": root, "config": map[string]any{"searchStrategy": t.SearchStrategy, "maxDepth": t.MaxDepth, "branchingFactor": t.BranchingFactor}}), nil
	})

	// generate_thoughts
	s.AddTool(mcp.NewTool("generate_thoughts",
		mcp.WithDescription("Add candidate thoughts as children of a parent node."),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("parent_id", mcp.Required()),
		mcp.WithArray("thoughts", mcp.Required(), mcp.Description("Array of {thought, metadata} objects")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		parentID, _ := req.RequireString("parent_id")
		raw := req.Params.Arguments["thoughts"]

		thoughtsJSON, _ := json.Marshal(raw)
		var items []struct {
			Thought  string         `json:"thought"`
			Metadata map[string]any `json:"metadata"`
		}
		json.Unmarshal(thoughtsJSON, &items)

		var created []any
		for _, item := range items {
			if item.Metadata == nil {
				item.Metadata = map[string]any{}
			}
			node, err := tree.AddThought(treeID, parentID, item.Thought, item.Metadata)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			created = append(created, node)
		}
		return textResult(map[string]any{"message": fmt.Sprintf("Added %d thoughts.", len(created)), "parentId": parentID, "newNodes": created}), nil
	})

	// evaluate_thought
	s.AddTool(mcp.NewTool("evaluate_thought",
		mcp.WithDescription(`Evaluate a thought as "sure", "maybe", or "impossible".`),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("node_id", mcp.Required()),
		mcp.WithString("evaluation", mcp.Required(), mcp.Description("sure, maybe, or impossible")),
		mcp.WithNumber("score", mcp.Description("Optional 0-1 score")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		nodeID, _ := req.RequireString("node_id")
		eval, _ := req.RequireString("evaluation")
		var scorePtr *float64
		if s, ok := req.Params.Arguments["score"]; ok {
			if v, ok := s.(float64); ok {
				scorePtr = &v
			}
		}
		node, err := tree.EvaluateThought(treeID, nodeID, eval, scorePtr)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(map[string]any{"node": node}), nil
	})

	// search_step
	s.AddTool(mcp.NewTool("search_step",
		mcp.WithDescription("Get the next node to expand (BFS/DFS/Beam)."),
		mcp.WithString("tree_id", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		node, err := tree.SearchStep(treeID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if node == nil {
			return textResult(map[string]any{"message": "No expandable nodes remain.", "nextNode": nil}), nil
		}
		path, _ := tree.GetPathToNode(treeID, node.ID)
		return textResult(map[string]any{"message": "Expand this node.", "nextNode": node, "pathToHere": path.Thoughts, "pathScore": path.AverageScore}), nil
	})

	// backtrack
	s.AddTool(mcp.NewTool("backtrack",
		mcp.WithDescription("Prune a subtree and return to parent."),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("node_id", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		nodeID, _ := req.RequireString("node_id")
		parent, err := tree.Backtrack(treeID, nodeID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(map[string]any{"message": "Subtree pruned.", "parentNode": parent}), nil
	})

	// mark_solution
	s.AddTool(mcp.NewTool("mark_solution",
		mcp.WithDescription("Mark a node as the terminal solution."),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("node_id", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		nodeID, _ := req.RequireString("node_id")
		node, err := tree.MarkTerminal(treeID, nodeID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		path, _ := tree.GetPathToNode(treeID, node.ID)
		return textResult(map[string]any{"message": "Solution marked.", "node": node, "path": path.Thoughts}), nil
	})

	// get_best_path
	s.AddTool(mcp.NewTool("get_best_path",
		mcp.WithDescription("Extract the highest-scoring path through the tree."),
		mcp.WithString("tree_id", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		path, err := tree.GetBestPath(treeID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if path == nil {
			return textResult(map[string]any{"message": "No valid paths found.", "path": nil}), nil
		}
		return textResult(map[string]any{"message": "Best path extracted.", "path": path}), nil
	})

	// get_tree_summary
	s.AddTool(mcp.NewTool("get_tree_summary",
		mcp.WithDescription("View tree stats."),
		mcp.WithString("tree_id", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		summary, err := tree.Summary(treeID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(summary), nil
	})

	// inspect_node
	s.AddTool(mcp.NewTool("inspect_node",
		mcp.WithDescription("Inspect a node, its children, and path from root."),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("node_id", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		nodeID, _ := req.RequireString("node_id")
		node, err := tree.GetNode(treeID, nodeID)
		if err != nil {
			return mcp.NewToolResultError("Node not found"), nil
		}
		children, _ := tree.GetChildren(treeID, nodeID)
		path, _ := tree.GetPathToNode(treeID, nodeID)
		return textResult(map[string]any{"node": node, "children": children, "pathFromRoot": path.Thoughts, "pathScore": path.AverageScore}), nil
	})

	// list_trees
	s.AddTool(mcp.NewTool("list_trees",
		mcp.WithDescription("List all reasoning trees."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		trees, _ := tree.ListTrees()
		items := make([]map[string]any, len(trees))
		for i, t := range trees {
			items[i] = map[string]any{
				"id": t.ID, "problem": truncate(t.Problem, 120),
				"status": t.Status, "nodeCount": tree.NodeCount(t.ID),
				"strategy": t.SearchStrategy, "createdAt": t.CreatedAt,
			}
		}
		return textResult(map[string]any{"count": len(trees), "trees": items}), nil
	})

	// route_problem — decides whether to continue an existing tree or create a new one
	s.AddTool(mcp.NewTool("route_problem",
		mcp.WithDescription("CALL THIS BEFORE create_tree. Checks if a new problem should continue an existing tree or start a new one. Returns routing decision with action='continue' (resume existing tree) or action='create' (make a new tree). Prevents duplicate trees for the same topic."),
		mcp.WithString("problem", mcp.Required(), mcp.Description("The problem statement you're about to work on")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		problem, _ := req.RequireString("problem")

		result, err := tree.RouteProblem(problem, nil)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(result), nil
	})

	// abandon_tree — explicitly close a tree
	s.AddTool(mcp.NewTool("abandon_tree",
		mcp.WithDescription("Mark a tree as abandoned. Use when the problem is no longer relevant or the approach is wrong. The tree stays in the database for reference but is excluded from route_problem matching."),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("reason", mcp.Description("Why the tree is being abandoned")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		reason := optString(req, "reason", "")

		if err := tree.SetStatus(treeID, "abandoned"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(map[string]any{
			"message": "Tree abandoned.",
			"treeId":  treeID,
			"reason":  reason,
			"note":    "Tree is preserved in the database. Call resume_tree to reactivate.",
		}), nil
	})

	// resume_tree — reactivate a paused or abandoned tree
	s.AddTool(mcp.NewTool("resume_tree",
		mcp.WithDescription("Reactivate a paused or abandoned tree. Use when route_problem returns action='continue' with a paused tree, or when revisiting an old analysis."),
		mcp.WithString("tree_id", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")

		if err := tree.SetStatus(treeID, "active"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		tree.TouchTree(treeID)

		summary, err := tree.Summary(treeID)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		summary["message"] = "Tree resumed."
		return textResult(summary), nil
	})
}

// ============================================================================
// Retrieval tools
// ============================================================================

func registerRetrievalTools(s *server.MCPServer) {
	// retrieve_context
	s.AddTool(mcp.NewTool("retrieve_context",
		mcp.WithDescription("Hybrid search past solutions (vector + keyword)."),
		mcp.WithString("query", mcp.Required()),
		mcp.WithNumber("top_k", mcp.Description("Results to return"), mcp.DefaultNumber(3)),
		mcp.WithArray("tags", mcp.Description("Optional tag filter")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query, _ := req.RequireString("query")
		topK := optInt(req, "top_k", 3)
		var tags []string
		if raw, ok := req.Params.Arguments["tags"]; ok {
			b, _ := json.Marshal(raw)
			json.Unmarshal(b, &tags)
		}
		results, err := retrieval.Retrieve(query, topK, tags)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		msg := "No relevant past solutions found."
		if len(results) > 0 {
			msg = fmt.Sprintf("Found %d relevant solutions.", len(results))
		}
		return textResult(map[string]any{"message": msg, "results": results}), nil
	})

	// store_solution
	s.AddTool(mcp.NewTool("store_solution",
		mcp.WithDescription("Save a solution for future retrieval."),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("solution", mcp.Required()),
		mcp.WithArray("tags"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		solution, _ := req.RequireString("solution")
		var tags []string
		if raw, ok := req.Params.Arguments["tags"]; ok {
			b, _ := json.Marshal(raw)
			json.Unmarshal(b, &tags)
		}

		t, err := tree.GetTree(treeID)
		if err != nil {
			return mcp.NewToolResultError("Tree not found"), nil
		}
		best, _ := tree.GetBestPath(treeID)
		var thoughts, pathIDs []string
		var score float64
		if best != nil {
			thoughts = best.Thoughts
			pathIDs = best.NodeIDs
			score = best.AverageScore
		}
		id, err := retrieval.StoreSolution(treeID, t.Problem, solution, thoughts, pathIDs, score, tags)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(map[string]any{"message": "Solution stored.", "entryId": id}), nil
	})

	// retrieval_stats
	s.AddTool(mcp.NewTool("retrieval_stats",
		mcp.WithDescription("View retrieval store statistics."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return textResult(retrieval.Stats()), nil
	})
}

// ============================================================================
// Experiment tools
// ============================================================================

func registerExperimentTools(s *server.MCPServer) {
	// configure_experiment
	s.AddTool(mcp.NewTool("configure_experiment",
		mcp.WithDescription("Configure the experiment runner for a tree."),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("target_file", mcp.Required(), mcp.Description(`File to modify (e.g. "train.py")`)),
		mcp.WithString("run_command", mcp.Required(), mcp.Description(`Command to run (e.g. "uv run train.py")`)),
		mcp.WithString("metric_regex", mcp.Required(), mcp.Description("Regex with one capture group for the metric")),
		mcp.WithString("metric_direction", mcp.Description(`"lower" or "higher"`), mcp.DefaultString("lower")),
		mcp.WithNumber("timeout_seconds", mcp.DefaultNumber(600)),
		mcp.WithString("work_dir", mcp.Required(), mcp.Description("Absolute path to experiment directory")),
		mcp.WithString("git_branch_prefix", mcp.DefaultString("autoresearch")),
		mcp.WithString("log_file", mcp.DefaultString("run.log")),
		mcp.WithString("memory_regex"),
		mcp.WithNumber("baseline_metric"),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		cfg := experiment.Config{
			TargetFile:      mustString(req, "target_file"),
			RunCommand:      mustString(req, "run_command"),
			MetricRegex:     mustString(req, "metric_regex"),
			MetricDirection: optString(req, "metric_direction", "lower"),
			TimeoutSeconds:  optInt(req, "timeout_seconds", 600),
			WorkDir:         mustString(req, "work_dir"),
			GitBranchPrefix: optString(req, "git_branch_prefix", "autoresearch"),
			LogFile:         optString(req, "log_file", "run.log"),
			MemoryRegex:     optString(req, "memory_regex", ""),
		}
		if v, ok := req.Params.Arguments["baseline_metric"]; ok {
			if f, ok := v.(float64); ok {
				cfg.BaselineMetric = &f
			}
		}
		if err := experiment.SetConfig(treeID, cfg); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(map[string]any{"message": "Config saved.", "targetFile": cfg.TargetFile, "runCommand": cfg.RunCommand}), nil
	})

	// prepare_experiment
	s.AddTool(mcp.NewTool("prepare_experiment",
		mcp.WithDescription("Apply code changes and git commit."),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("node_id", mcp.Required()),
		mcp.WithString("patch_content", mcp.Required(), mcp.Description("Full file content to write")),
		mcp.WithString("commit_message", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		content, _ := req.RequireString("patch_content")
		msg, _ := req.RequireString("commit_message")

		result, err := experiment.Prepare(treeID, content, msg)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(map[string]any{"message": "Prepared. Call execute_experiment next.", "commitHash": result.CommitHash, "previousHash": result.PreviousHash, "branch": result.Branch}), nil
	})

	// execute_experiment
	s.AddTool(mcp.NewTool("execute_experiment",
		mcp.WithDescription("Run training, parse metric, auto-evaluate, keep/discard."),
		mcp.WithString("tree_id", mcp.Required()),
		mcp.WithString("node_id", mcp.Required()),
		mcp.WithString("previous_hash", mcp.Required(), mcp.Description("Hash to reset to on failure")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		nodeID, _ := req.RequireString("node_id")
		prevHash, _ := req.RequireString("previous_hash")

		result, err := experiment.Execute(treeID, nodeID, prevHash)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return textResult(result), nil
	})

	// experiment_history
	s.AddTool(mcp.NewTool("experiment_history",
		mcp.WithDescription("View experiment stats for a tree."),
		mcp.WithString("tree_id", mcp.Required()),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		treeID, _ := req.RequireString("tree_id")
		return textResult(experiment.History(treeID)), nil
	})
}

// ============================================================================
// Helpers
// ============================================================================

func mustString(req mcp.CallToolRequest, key string) string {
	v, _ := req.RequireString(key)
	return v
}

func optString(req mcp.CallToolRequest, key, fallback string) string {
	if v, ok := req.Params.Arguments[key]; ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return fallback
}

func optInt(req mcp.CallToolRequest, key string, fallback int) int {
	if v, ok := req.Params.Arguments[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return fallback
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
