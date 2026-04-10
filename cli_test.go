package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
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

	commands := []string{"suggest", "list", "show", "route", "create", "ready", "audit", "stats", "compact", "lint", "health", "drift"}
	for _, cmd := range commands {
		if !strings.Contains(content, fmt.Sprintf("case %q:", cmd)) {
			t.Fatalf("CLI switch missing case for %q", cmd)
		}
		if !strings.Contains(content, "  "+cmd) {
			t.Fatalf("CLI help text missing entry for %q", cmd)
		}
	}
}
