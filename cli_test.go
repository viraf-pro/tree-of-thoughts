package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

func TestMain(m *testing.M) {
	tmp := filepath.Join(os.TempDir(), "tot-mcp-cli-test.db")
	os.Remove(tmp)
	if _, err := db.Init(tmp); err != nil {
		panic(err)
	}
	code := m.Run()
	os.Remove(tmp)
	os.Exit(code)
}

func TestCliLintRuns(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cliLint()

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "totalSolutions") {
		t.Fatalf("expected totalSolutions in output, got: %s", output)
	}
}

func TestCliDriftRuns(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cliDrift()

	w.Close()
	os.Stdout = old

	buf := make([]byte, 8192)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "duplicateTreePairs") {
		t.Fatalf("expected duplicateTreePairs in output, got: %s", output)
	}
}

func TestCliHealthRuns(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cliHealth()

	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "trees") {
		t.Fatalf("expected trees in output, got: %s", output)
	}
	if !strings.Contains(output, "solutions") {
		t.Fatalf("expected solutions in output, got: %s", output)
	}
}

func TestCliAddEvalSolve(t *testing.T) {
	// Create a tree to operate on
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	cliCreate("cli add eval solve test", "beam")
	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	// Extract tree ID and root ID from create output
	var treeID, rootID string
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Created tree ") {
			treeID = strings.TrimPrefix(line, "Created tree ")
		}
		if strings.Contains(line, "Root:") {
			rootID = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "Root:"))
		}
	}
	if treeID == "" || rootID == "" {
		t.Fatalf("failed to extract IDs from create output: %s", output)
	}

	// Test add
	r, w, _ = os.Pipe()
	os.Stdout = w
	cliAdd(treeID, rootID, "test thought from CLI")
	w.Close()
	os.Stdout = old

	buf = make([]byte, 4096)
	n, _ = r.Read(buf)
	addOutput := string(buf[:n])
	if !strings.Contains(addOutput, "Added node") {
		t.Fatalf("cliAdd output missing 'Added node': %s", addOutput)
	}
	if !strings.Contains(addOutput, "Depth:   1") {
		t.Fatalf("cliAdd should create depth 1 node: %s", addOutput)
	}

	// Extract node ID
	var nodeID string
	for _, line := range strings.Split(addOutput, "\n") {
		if strings.HasPrefix(line, "Added node ") {
			nodeID = strings.TrimPrefix(line, "Added node ")
		}
	}
	if nodeID == "" {
		t.Fatalf("failed to extract node ID from add output: %s", addOutput)
	}

	// Test eval
	r, w, _ = os.Pipe()
	os.Stdout = w
	cliEval(treeID, nodeID, "sure", "0.85")
	w.Close()
	os.Stdout = old

	buf = make([]byte, 4096)
	n, _ = r.Read(buf)
	evalOutput := string(buf[:n])
	if !strings.Contains(evalOutput, "Evaluated node") {
		t.Fatalf("cliEval output missing 'Evaluated node': %s", evalOutput)
	}
	if !strings.Contains(evalOutput, "Score:      0.85") {
		t.Fatalf("cliEval should show custom score 0.85: %s", evalOutput)
	}

	// Test solve
	r, w, _ = os.Pipe()
	os.Stdout = w
	cliSolve(treeID, nodeID)
	w.Close()
	os.Stdout = old

	buf = make([]byte, 4096)
	n, _ = r.Read(buf)
	solveOutput := string(buf[:n])
	if !strings.Contains(solveOutput, "Solution marked") {
		t.Fatalf("cliSolve output missing 'Solution marked': %s", solveOutput)
	}
	if !strings.Contains(solveOutput, "Path:") {
		t.Fatalf("cliSolve should show path: %s", solveOutput)
	}
}

func TestCliEvalWithoutScore(t *testing.T) {
	// Create a tree and node
	tr, root, _ := tree.CreateTree("cli eval no score test", "beam", 5, 3)
	node, _ := tree.AddThought(tr.ID, root.ID, "eval without score", nil)

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	cliEval(tr.ID, node.ID, "maybe", "")
	w.Close()
	os.Stdout = old

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	output := string(buf[:n])

	if !strings.Contains(output, "Score:      0.50") {
		t.Fatalf("cliEval 'maybe' without custom score should default to 0.50: %s", output)
	}
}

// --- Structural tests (self-enforcement) ---

func TestStructuralToolCountMatchesDocs(t *testing.T) {
	// Count s.AddTool( calls in main.go
	mainFile, err := os.Open("main.go")
	if err != nil {
		t.Fatalf("open main.go: %v", err)
	}
	defer mainFile.Close()

	toolCount := 0
	scanner := bufio.NewScanner(mainFile)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "s.AddTool(mcp.NewTool(") {
			toolCount++
		}
	}

	if toolCount < 30 {
		t.Fatalf("expected at least 30 registered tools, found %d", toolCount)
	}

	// Verify TOT_INSTRUCTIONS.md has a tool reference table
	instrFile, err := os.Open("TOT_INSTRUCTIONS.md")
	if err != nil {
		t.Fatalf("open TOT_INSTRUCTIONS.md: %v", err)
	}
	defer instrFile.Close()

	docToolCount := 0
	scanner = bufio.NewScanner(instrFile)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "| `") && strings.Contains(line, "` |") {
			docToolCount++
		}
	}

	if docToolCount < 15 {
		t.Fatalf("expected at least 15 tools documented in TOT_INSTRUCTIONS.md, found %d", docToolCount)
	}
}

func TestStructuralAllToolsHaveDescriptions(t *testing.T) {
	mainFile, err := os.Open("main.go")
	if err != nil {
		t.Fatalf("open main.go: %v", err)
	}
	defer mainFile.Close()

	scanner := bufio.NewScanner(mainFile)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(line, "s.AddTool(mcp.NewTool(") {
			// Next few lines should contain WithDescription
			foundDesc := false
			for i := 0; i < 5 && scanner.Scan(); i++ {
				lineNum++
				if strings.Contains(scanner.Text(), "WithDescription") {
					foundDesc = true
					break
				}
			}
			if !foundDesc {
				t.Fatalf("tool at line %d missing WithDescription", lineNum)
			}
		}
	}
}

func TestStructuralCLIHelpCoversAllCommands(t *testing.T) {
	// Every case in the switch should appear in help text
	mainFile, err := os.ReadFile("cli.go")
	if err != nil {
		t.Fatalf("read cli.go: %v", err)
	}
	content := string(mainFile)

	commands := []string{"suggest", "list", "show", "route", "create", "add", "eval", "solve", "ready", "audit", "stats", "compact", "lint", "health", "drift", "export", "report", "ingest"}
	for _, cmd := range commands {
		if !strings.Contains(content, fmt.Sprintf("case %q:", cmd)) {
			t.Fatalf("CLI switch missing case for %q", cmd)
		}
		if !strings.Contains(content, "  "+cmd) {
			t.Fatalf("CLI help text missing entry for %q", cmd)
		}
	}
}
