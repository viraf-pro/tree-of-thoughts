package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/retrieval"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

// runCLI handles command-line invocations. Returns true if it handled the args.
// Uses ~1-2k tokens of context (just the output) vs 10-50k for MCP schemas.
func runCLI(args []string) bool {
	if len(args) < 2 {
		return false
	}

	cmd := args[1]

	// Initialize DB for CLI mode
	if cmd != "help" && cmd != "--help" && cmd != "-h" {
		if _, err := db.Init(os.Getenv("TOT_DB_PATH")); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("tot-mcp %s\n", version)
		return true
	case "help", "--help", "-h":
		printHelp()
	case "suggest":
		cliSuggest()
	case "list":
		cliList()
	case "show":
		if len(args) < 3 {
			fatal("Usage: tot-mcp show <tree_id>")
		}
		cliShow(args[2])
	case "route":
		if len(args) < 3 {
			fatal("Usage: tot-mcp route <problem>")
		}
		cliRoute(strings.Join(args[2:], " "))
	case "create":
		if len(args) < 3 {
			fatal("Usage: tot-mcp create <problem> [--strategy beam|bfs|dfs]")
		}
		strategy := "beam"
		problem := []string{}
		for i := 2; i < len(args); i++ {
			if args[i] == "--strategy" && i+1 < len(args) {
				strategy = args[i+1]
				i++
			} else {
				problem = append(problem, args[i])
			}
		}
		cliCreate(strings.Join(problem, " "), strategy)
	case "ready":
		cliReady()
	case "audit":
		treeID := ""
		if len(args) >= 3 {
			treeID = args[2]
		}
		cliAudit(treeID)
	case "stats":
		cliStats()
	case "compact":
		cliCompact()
	case "lint":
		cliLint()
	case "health":
		cliHealth()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\nRun 'tot-mcp help' for usage.\n", cmd)
		os.Exit(1)
	}

	return true
}

func printHelp() {
	fmt.Printf("tot-mcp %s — Tree of Thoughts MCP server and CLI\n\n", version)
	fmt.Println(`USAGE:
  tot-mcp                    Start MCP server (stdio) + dashboard
  tot-mcp <command> [args]   Run a CLI command

COMMANDS:
  suggest                    What should I work on next?
  list                       List all trees
  show <tree_id>             Show tree summary and best path
  route <problem>            Check if problem matches an existing tree
  create <problem>           Create a new tree (default: beam search)
  ready                      Show active trees with frontier nodes
  audit [tree_id]            View audit trail (last 20 entries)
  stats                      Retrieval store statistics
  compact                    Find solutions eligible for compaction
  lint                       Lint the knowledge store for quality issues
  health                     Machine-readable health summary (JSON)
  version                    Show version
  help                       Show this message

OPTIONS:
  --strategy beam|bfs|dfs    Search strategy (with create)

ENVIRONMENT:
  TOT_DB_PATH                Database location (default: ~/.tot-mcp/tot.db)
  TOT_DASHBOARD_PORT         Dashboard port (default: 4545)
  TOT_NO_DASHBOARD           Disable dashboard
  TOT_EMBED_PROVIDER         local|openai|voyage|ollama
  OPENAI_API_KEY             Enable OpenAI embeddings
  VOYAGE_API_KEY             Enable Voyage embeddings
  OLLAMA_BASE_URL            Enable Ollama embeddings`)
}

func cliSuggest() {
	result, err := tree.SuggestNextWork()
	if err != nil {
		fatal(err.Error())
	}
	printJSON(result)
}

func cliList() {
	trees, err := tree.ListTrees()
	if err != nil {
		fatal(err.Error())
	}
	for _, t := range trees {
		status := t.Status
		nc := tree.NodeCount(t.ID)
		fmt.Printf("%-36s  %-10s  %3d nodes  %s\n", t.ID, status, nc, truncate(t.Problem, 50))
	}
	if len(trees) == 0 {
		fmt.Println("No trees found.")
	}
}

func cliShow(treeID string) {
	summary, err := tree.Summary(treeID)
	if err != nil {
		fatal(err.Error())
	}
	printJSON(summary)
}

func cliRoute(problem string) {
	result, err := tree.RouteProblem(problem)
	if err != nil {
		fatal(err.Error())
	}
	printJSON(result)
}

func cliCreate(problem, strategy string) {
	t, root, err := tree.CreateTree(problem, strategy, 5, 3)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Printf("Created tree %s\n", t.ID)
	fmt.Printf("  Problem:  %s\n", t.Problem)
	fmt.Printf("  Strategy: %s\n", t.SearchStrategy)
	fmt.Printf("  Root:     %s\n", root.ID)
}

func cliReady() {
	trees, err := tree.ListTrees()
	if err != nil {
		fatal(err.Error())
	}
	found := false
	for _, t := range trees {
		if t.Status != "active" {
			continue
		}
		nc := tree.NodeCount(t.ID)
		summary, _ := tree.Summary(t.ID)
		var frontier int
		if stats, ok := summary["stats"].(map[string]int); ok {
			frontier = stats["frontierSize"]
		}
		if frontier > 0 {
			fmt.Printf("%-36s  %3d frontier  %3d total  %s\n", t.ID, frontier, nc, truncate(t.Problem, 40))
			found = true
		}
	}
	if !found {
		fmt.Println("No active trees with frontier nodes.")
	}
}

func cliAudit(treeID string) {
	entries, err := db.GetAuditLog(treeID, 20)
	if err != nil {
		fatal(err.Error())
	}
	for _, e := range entries {
		tid := e.TreeID
		if tid == "" {
			tid = "-"
		}
		fmt.Printf("%-20s  %-20s  %-36s  %s\n", e.CreatedAt[:19], e.Tool, tid, truncate(e.Result, 40))
	}
	if len(entries) == 0 {
		fmt.Println("No audit entries found.")
	}
}

func cliStats() {
	stats := retrieval.Stats()
	printJSON(stats)
}

func cliCompact() {
	candidates, err := retrieval.CompactAnalyze(30)
	if err != nil {
		fatal(err.Error())
	}
	if len(candidates) == 0 {
		fmt.Println("No solutions eligible for compaction (none older than 30 days).")
		return
	}
	fmt.Printf("%d solutions eligible for compaction:\n\n", len(candidates))
	for _, c := range candidates {
		fmt.Printf("  %-36s  %3d days old  %s\n", c.ID, c.AgeDays, truncate(c.Problem, 50))
	}
	fmt.Println("\nUse compact_apply via MCP to compress each with a summary.")
}

func cliLint() {
	report, err := retrieval.LintKnowledge()
	if err != nil {
		fatal(err.Error())
	}
	printJSON(report)
}

func cliHealth() {
	stats := retrieval.Stats()
	trees, _ := tree.ListTrees()

	active, paused, solved, abandoned := 0, 0, 0, 0
	var lastActivity string
	for _, t := range trees {
		switch t.Status {
		case "active":
			active++
		case "paused":
			paused++
		case "solved":
			solved++
		case "abandoned":
			abandoned++
		}
		if t.UpdatedAt > lastActivity {
			lastActivity = t.UpdatedAt
		}
	}

	printJSON(map[string]any{
		"solutions": stats,
		"trees": map[string]any{
			"total":     len(trees),
			"active":    active,
			"paused":    paused,
			"solved":    solved,
			"abandoned": abandoned,
		},
		"lastActivity": lastActivity,
	})
}

func printJSON(v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}

func fatal(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}
