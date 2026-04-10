package main

import (
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
