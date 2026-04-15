package experiment

import (
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

func TestMain(m *testing.M) {
	tmp := filepath.Join(os.TempDir(), "tot-mcp-experiment-test.db")
	os.Remove(tmp)
	if _, err := db.Init(tmp); err != nil {
		panic(err)
	}
	code := m.Run()
	os.Remove(tmp)
	os.Exit(code)
}

// --- parseMetric ---

func TestParseMetricMatchesFirstGroup(t *testing.T) {
	got := parseMetric("val_loss: 0.456\nval_acc: 0.89", `val_loss:\s+([\d.]+)`)
	if got == nil {
		t.Fatal("expected metric, got nil")
	}
	if *got != 0.456 {
		t.Fatalf("got %f, want 0.456", *got)
	}
}

func TestParseMetricNoMatch(t *testing.T) {
	got := parseMetric("no metrics here", `val_loss:\s+([\d.]+)`)
	if got != nil {
		t.Fatalf("expected nil, got %f", *got)
	}
}

func TestParseMetricInvalidRegex(t *testing.T) {
	got := parseMetric("anything", `[invalid`)
	if got != nil {
		t.Fatalf("expected nil for invalid regex, got %f", *got)
	}
}

func TestParseMetricMultiline(t *testing.T) {
	output := "epoch 1\nloss: 1.23\nepoch 2\nloss: 0.89"
	got := parseMetric(output, `^loss:\s+([\d.]+)`)
	if got == nil {
		t.Fatal("expected metric from multiline output")
	}
	if *got != 1.23 {
		t.Fatalf("got %f, want 1.23 (first match)", *got)
	}
}

func TestParseMetricNonNumericGroup(t *testing.T) {
	got := parseMetric("status: running", `status:\s+(\w+)`)
	if got != nil {
		t.Fatalf("expected nil for non-numeric capture, got %f", *got)
	}
}

// --- isImproved ---

func TestIsImprovedLowerBetter(t *testing.T) {
	baseline := 1.0
	cfg := &Config{MetricDirection: "lower", BaselineMetric: &baseline}

	if !isImproved(0.5, cfg) {
		t.Error("0.5 < 1.0 should be improved when lower is better")
	}
	if isImproved(1.5, cfg) {
		t.Error("1.5 > 1.0 should not be improved when lower is better")
	}
	if isImproved(1.0, cfg) {
		t.Error("equal should not be improved")
	}
}

func TestIsImprovedHigherBetter(t *testing.T) {
	baseline := 0.8
	cfg := &Config{MetricDirection: "higher", BaselineMetric: &baseline}

	if !isImproved(0.9, cfg) {
		t.Error("0.9 > 0.8 should be improved when higher is better")
	}
	if isImproved(0.7, cfg) {
		t.Error("0.7 < 0.8 should not be improved when higher is better")
	}
}

func TestIsImprovedNoBaseline(t *testing.T) {
	cfg := &Config{MetricDirection: "lower", BaselineMetric: nil}
	if !isImproved(99.0, cfg) {
		t.Error("first run (no baseline) should always be improved")
	}
}

// --- improvementScore ---

func TestImprovementScoreNoBaseline(t *testing.T) {
	metric := 0.5
	cfg := &Config{MetricDirection: "lower", BaselineMetric: nil}
	score := improvementScore(&metric, cfg)
	if score != 0.7 {
		t.Fatalf("no baseline should return 0.7, got %f", score)
	}
}

func TestImprovementScoreNilMetric(t *testing.T) {
	baseline := 1.0
	cfg := &Config{MetricDirection: "lower", BaselineMetric: &baseline}
	score := improvementScore(nil, cfg)
	if score != 0.7 {
		t.Fatalf("nil metric should return 0.7, got %f", score)
	}
}

func TestImprovementScoreLowerImproved(t *testing.T) {
	baseline := 1.0
	metric := 0.5
	cfg := &Config{MetricDirection: "lower", BaselineMetric: &baseline}
	score := improvementScore(&metric, cfg)
	// delta=0.5, pct=0.5, score = 0.7 + 0.5*6 = 3.7, capped at 0.99
	if score != 0.99 {
		t.Fatalf("large improvement should cap at 0.99, got %f", score)
	}
}

func TestImprovementScoreRegressed(t *testing.T) {
	baseline := 0.5
	metric := 1.0
	cfg := &Config{MetricDirection: "lower", BaselineMetric: &baseline}
	score := improvementScore(&metric, cfg)
	if score != 0.2 {
		t.Fatalf("regression should return 0.2, got %f", score)
	}
}

func TestImprovementScoreSmallImprovement(t *testing.T) {
	baseline := 1.0
	metric := 0.95
	cfg := &Config{MetricDirection: "lower", BaselineMetric: &baseline}
	score := improvementScore(&metric, cfg)
	// delta=0.05, pct=0.05, score = 0.7 + 0.05*6 = 1.0, capped at 0.99
	expected := math.Min(0.99, 0.7+0.05*6)
	if math.Abs(score-expected) > 0.001 {
		t.Fatalf("got %f, want %f", score, expected)
	}
}

// --- boolToInt ---

func TestBoolToInt(t *testing.T) {
	if boolToInt(true) != 1 {
		t.Error("true should be 1")
	}
	if boolToInt(false) != 0 {
		t.Error("false should be 0")
	}
}

// --- readTail ---

func TestReadTailFewerLines(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "log.txt")
	os.WriteFile(tmp, []byte("line1\nline2\nline3"), 0o644)

	got := readTail(tmp, 10)
	if got != "line1\nline2\nline3" {
		t.Fatalf("fewer lines than requested should return all: got %q", got)
	}
}

func TestReadTailTruncates(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "log.txt")
	os.WriteFile(tmp, []byte("a\nb\nc\nd\ne"), 0o644)

	got := readTail(tmp, 3)
	if got != "c\nd\ne" {
		t.Fatalf("should return last 3 lines: got %q", got)
	}
}

func TestReadTailMissing(t *testing.T) {
	got := readTail("/nonexistent/file", 10)
	if got != "(no log)" {
		t.Fatalf("missing file should return '(no log)', got %q", got)
	}
}

// --- SetConfig / GetConfig (DB) ---

func TestSetConfigAndGet(t *testing.T) {
	tr, _, err := tree.CreateTree("experiment config test", "beam", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "train.py")
	os.WriteFile(targetFile, []byte("print('hello')"), 0o644)

	cfg := Config{
		TargetFile:      "train.py",
		RunCommand:      "python train.py",
		MetricRegex:     `loss:\s+([\d.]+)`,
		MetricDirection: "lower",
		TimeoutSeconds:  30,
		WorkDir:         tmpDir,
		GitBranchPrefix: "test",
		LogFile:         "out.log",
	}

	if err := SetConfig(tr.ID, cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	got, err := GetConfig(tr.ID)
	if err != nil {
		t.Fatalf("GetConfig: %v", err)
	}
	if got.TargetFile != "train.py" {
		t.Errorf("TargetFile: got %q", got.TargetFile)
	}
	if got.MetricDirection != "lower" {
		t.Errorf("MetricDirection: got %q", got.MetricDirection)
	}
	if got.WorkDir != tmpDir {
		t.Errorf("WorkDir: got %q", got.WorkDir)
	}
}

func TestSetConfigRejectsRelativePath(t *testing.T) {
	err := SetConfig("fake-tree", Config{WorkDir: "relative/path"})
	if err == nil {
		t.Error("expected error for relative work_dir")
	}
}

func TestSetConfigRejectsPathTraversal(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a target file outside the work dir
	targetFile := filepath.Join(tmpDir, "legit.py")
	os.WriteFile(targetFile, []byte("x"), 0o644)

	err := SetConfig("fake-tree", Config{
		WorkDir:     tmpDir,
		TargetFile:  "../../etc/passwd",
		MetricRegex: `.*`,
	})
	if err == nil {
		t.Error("expected error for path traversal in target_file")
	}
}

func TestSetConfigRejectsInvalidRegex(t *testing.T) {
	tmpDir := t.TempDir()
	targetFile := filepath.Join(tmpDir, "train.py")
	os.WriteFile(targetFile, []byte("x"), 0o644)

	err := SetConfig("fake-tree", Config{
		WorkDir:     tmpDir,
		TargetFile:  "train.py",
		MetricRegex: `[invalid`,
	})
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestGetConfigMissing(t *testing.T) {
	_, err := GetConfig("nonexistent-tree-id")
	if err == nil {
		t.Error("expected error for missing config")
	}
}

// --- History (DB) ---

func TestHistoryEmpty(t *testing.T) {
	tr, _, err := tree.CreateTree("history empty test", "bfs", 3, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	h := History(tr.ID)
	if h["totalExperiments"].(int) != 0 {
		t.Errorf("expected 0 total experiments, got %v", h["totalExperiments"])
	}
	if h["successRate"].(int) != 0 {
		t.Errorf("expected 0 success rate, got %v", h["successRate"])
	}
}

func TestHistoryWithResults(t *testing.T) {
	tr, root, err := tree.CreateTree("history results test", "bfs", 3, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	metric := 0.5
	logResult(tr.ID, root.ID, &Result{Status: "improved", Metric: &metric, Kept: true})
	logResult(tr.ID, root.ID, &Result{Status: "regressed", Kept: false})
	logResult(tr.ID, root.ID, &Result{Status: "crashed", Kept: false})

	h := History(tr.ID)
	if h["totalExperiments"].(int) != 3 {
		t.Errorf("expected 3 total, got %v", h["totalExperiments"])
	}
	if h["improved"].(int) != 1 {
		t.Errorf("expected 1 improved, got %v", h["improved"])
	}
	if h["crashed"].(int) != 1 {
		t.Errorf("expected 1 crashed, got %v", h["crashed"])
	}
	if h["successRate"].(int) != 33 {
		t.Errorf("expected 33%% success rate, got %v", h["successRate"])
	}
}

// --- Prepare + Execute (integration, needs git) ---

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}
	// Create initial file and commit
	os.WriteFile(filepath.Join(dir, "train.py"), []byte("print('v1')"), 0o644)
	exec.Command("git", "-C", dir, "add", "-A").Run()
	exec.Command("git", "-C", dir, "commit", "-m", "initial").Run()
	return dir
}

func TestPrepareCreatesCommit(t *testing.T) {
	dir := initGitRepo(t)
	tr, _, err := tree.CreateTree("prepare test", "beam", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	cfg := Config{
		TargetFile:      "train.py",
		RunCommand:      "echo ok",
		MetricRegex:     `ok`,
		MetricDirection: "lower",
		TimeoutSeconds:  5,
		WorkDir:         dir,
		GitBranchPrefix: "exp",
		LogFile:         "out.log",
	}
	if err := SetConfig(tr.ID, cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	result, err := Prepare(tr.ID, "print('v2')", "test patch")
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if result.CommitHash == "" {
		t.Error("expected commit hash")
	}
	if result.PreviousHash == "" {
		t.Error("expected previous hash")
	}
	if result.Branch == "" {
		t.Error("expected branch name")
	}

	// Verify file was patched
	content, _ := os.ReadFile(filepath.Join(dir, "train.py"))
	if string(content) != "print('v2')" {
		t.Errorf("file not patched: got %q", string(content))
	}
}

func TestExecuteImproved(t *testing.T) {
	dir := initGitRepo(t)
	tr, root, err := tree.CreateTree("execute improved test", "beam", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	baseline := 1.0
	cfg := Config{
		TargetFile:      "train.py",
		RunCommand:      "echo 'loss: 0.5'",
		MetricRegex:     `loss:\s+([\d.]+)`,
		MetricDirection: "lower",
		TimeoutSeconds:  5,
		WorkDir:         dir,
		GitBranchPrefix: "exp",
		LogFile:         "out.log",
		BaselineMetric:  &baseline,
	}
	if err := SetConfig(tr.ID, cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	// Need a thought node to evaluate
	node, err := tree.AddThought(tr.ID, root.ID, "try something", nil)
	if err != nil {
		t.Fatalf("AddThought: %v", err)
	}

	prevHash := gitShort(dir)
	result, err := Execute(tr.ID, node.ID, prevHash)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "improved" {
		t.Errorf("expected improved, got %q", result.Status)
	}
	if result.Metric == nil || *result.Metric != 0.5 {
		t.Errorf("expected metric 0.5, got %v", result.Metric)
	}
	if !result.Kept {
		t.Error("improved result should be kept")
	}
}

func TestExecuteRegressed(t *testing.T) {
	dir := initGitRepo(t)
	tr, root, err := tree.CreateTree("execute regressed test", "beam", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	baseline := 0.1
	cfg := Config{
		TargetFile:      "train.py",
		RunCommand:      "echo 'loss: 0.5'",
		MetricRegex:     `loss:\s+([\d.]+)`,
		MetricDirection: "lower",
		TimeoutSeconds:  5,
		WorkDir:         dir,
		GitBranchPrefix: "exp",
		LogFile:         "out.log",
		BaselineMetric:  &baseline,
	}
	if err := SetConfig(tr.ID, cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	node, err := tree.AddThought(tr.ID, root.ID, "try worse", nil)
	if err != nil {
		t.Fatalf("AddThought: %v", err)
	}

	prevHash := gitShort(dir)
	result, err := Execute(tr.ID, node.ID, prevHash)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "regressed" {
		t.Errorf("expected regressed, got %q", result.Status)
	}
	if result.Kept {
		t.Error("regressed result should not be kept")
	}
}

func TestExecuteCrashed(t *testing.T) {
	dir := initGitRepo(t)
	tr, root, err := tree.CreateTree("execute crashed test", "beam", 5, 3)
	if err != nil {
		t.Fatalf("CreateTree: %v", err)
	}

	cfg := Config{
		TargetFile:      "train.py",
		RunCommand:      "exit 1",
		MetricRegex:     `loss:\s+([\d.]+)`,
		MetricDirection: "lower",
		TimeoutSeconds:  5,
		WorkDir:         dir,
		GitBranchPrefix: "exp",
		LogFile:         "out.log",
	}
	if err := SetConfig(tr.ID, cfg); err != nil {
		t.Fatalf("SetConfig: %v", err)
	}

	node, err := tree.AddThought(tr.ID, root.ID, "crash test", nil)
	if err != nil {
		t.Fatalf("AddThought: %v", err)
	}

	prevHash := gitShort(dir)
	result, err := Execute(tr.ID, node.ID, prevHash)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Status != "crashed" {
		t.Errorf("expected crashed, got %q", result.Status)
	}
	if result.Kept {
		t.Error("crashed result should not be kept")
	}
}
