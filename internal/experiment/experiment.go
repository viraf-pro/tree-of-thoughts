package experiment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tot-mcp/tot-mcp-go/internal/db"
	"github.com/tot-mcp/tot-mcp-go/internal/tree"
)

// Config defines how to run experiments for a tree.
type Config struct {
	TargetFile      string  `json:"target_file"`
	RunCommand      string  `json:"run_command"`
	MetricRegex     string  `json:"metric_regex"`
	MetricDirection string  `json:"metric_direction"` // "lower" or "higher"
	TimeoutSeconds  int     `json:"timeout_seconds"`
	WorkDir         string  `json:"work_dir"`
	GitBranchPrefix string  `json:"git_branch_prefix"`
	LogFile         string  `json:"log_file"`
	MemoryRegex     string  `json:"memory_regex,omitempty"`
	BaselineMetric  *float64 `json:"baseline_metric,omitempty"`
}

// Result of a single experiment run.
type Result struct {
	Status       string   `json:"status"` // improved, regressed, crashed, timeout
	Metric       *float64 `json:"metric"`
	MemoryMB     *float64 `json:"memoryMb"`
	DurationSecs float64  `json:"durationSeconds"`
	CommitHash   string   `json:"commitHash"`
	LogTail      string   `json:"logTail"`
	Kept         bool     `json:"kept"`
}

// PrepareResult is returned after patching + commit.
type PrepareResult struct {
	CommitHash   string `json:"commitHash"`
	PreviousHash string `json:"previousHash"`
	Branch       string `json:"branch"`
}

// SetConfig stores a config for a tree.
func SetConfig(treeID string, cfg Config) error {
	// Validate WorkDir is absolute to prevent path traversal
	if !filepath.IsAbs(cfg.WorkDir) {
		return fmt.Errorf("work_dir must be an absolute path, got %q", cfg.WorkDir)
	}
	if _, err := os.Stat(cfg.WorkDir); err != nil {
		return fmt.Errorf("work_dir not found: %w", err)
	}
	// Validate TargetFile doesn't escape WorkDir
	target := filepath.Join(cfg.WorkDir, cfg.TargetFile)
	absTarget, _ := filepath.Abs(target)
	if !strings.HasPrefix(absTarget, filepath.Clean(cfg.WorkDir)+string(filepath.Separator)) {
		return fmt.Errorf("target_file %q escapes work_dir", cfg.TargetFile)
	}
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("target_file not found: %w", err)
	}
	if _, err := regexp.Compile(cfg.MetricRegex); err != nil {
		return fmt.Errorf("invalid metric_regex: %w", err)
	}

	data, _ := json.Marshal(cfg)
	d := db.Get()
	_, err := d.Exec(`INSERT OR REPLACE INTO experiment_configs (tree_id,config) VALUES (?,?)`, treeID, string(data))
	return err
}

// GetConfig loads a config for a tree.
func GetConfig(treeID string) (*Config, error) {
	d := db.Get()
	var raw string
	err := d.QueryRow(`SELECT config FROM experiment_configs WHERE tree_id=?`, treeID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var cfg Config
	json.Unmarshal([]byte(raw), &cfg)
	return &cfg, nil
}

// Prepare applies a code patch and git commits.
func Prepare(treeID, content, commitMsg string) (*PrepareResult, error) {
	cfg, err := GetConfig(treeID)
	if err != nil {
		return nil, fmt.Errorf("no config: %w", err)
	}

	branch := ensureBranch(cfg, treeID)
	prevHash := gitShort(cfg.WorkDir)

	// Overwrite target file
	target := filepath.Join(cfg.WorkDir, cfg.TargetFile)
	if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
		return nil, err
	}

	hash, err := gitCommit(cfg.WorkDir, commitMsg)
	if err != nil {
		return nil, fmt.Errorf("git commit failed: %w", err)
	}

	return &PrepareResult{CommitHash: hash, PreviousHash: prevHash, Branch: branch}, nil
}

// Execute runs the experiment, parses the metric, auto-evaluates the node.
func Execute(treeID, nodeID, previousHash string) (*Result, error) {
	cfg, err := GetConfig(treeID)
	if err != nil {
		return nil, fmt.Errorf("no config: %w", err)
	}

	start := time.Now()
	commitHash := gitShort(cfg.WorkDir)
	logPath := filepath.Join(cfg.WorkDir, cfg.LogFile)

	// Run the command
	output, runErr := runCmd(cfg.RunCommand, cfg.WorkDir, cfg.TimeoutSeconds, logPath)
	duration := time.Since(start).Seconds()

	var result Result
	result.DurationSecs = duration
	result.CommitHash = commitHash

	if runErr != nil {
		isTimeout := strings.Contains(runErr.Error(), "killed") || strings.Contains(runErr.Error(), "signal")
		if isTimeout {
			result.Status = "timeout"
		} else {
			result.Status = "crashed"
		}
		result.LogTail = readTail(logPath, 50)
		result.Kept = false
	} else {
		metric := parseMetric(output, cfg.MetricRegex)
		result.Metric = metric

		if cfg.MemoryRegex != "" {
			result.MemoryMB = parseMetric(output, cfg.MemoryRegex)
		}

		if metric == nil {
			result.Status = "crashed"
			result.LogTail = readTail(logPath, 30)
			result.Kept = false
		} else {
			if isImproved(*metric, cfg) {
				result.Status = "improved"
				result.Kept = true
				cfg.BaselineMetric = metric
				SetConfig(treeID, *cfg)
			} else {
				result.Status = "regressed"
				result.Kept = false
			}
			result.LogTail = readTail(logPath, 10)
		}
	}

	// Git keep/discard
	if !result.Kept && previousHash != "" {
		if err := gitReset(cfg.WorkDir, previousHash); err != nil {
			result.LogTail += fmt.Sprintf("\nWARN: git reset --hard %s failed: %v", previousHash, err)
		}
	}

	// Auto-evaluate the thought node
	if err := autoEvaluate(treeID, nodeID, &result, cfg); err != nil {
		// Log the result before returning the error
		logResult(treeID, nodeID, &result)
		return &result, fmt.Errorf("experiment ran but node evaluation failed: %w", err)
	}

	// Log result
	logResult(treeID, nodeID, &result)

	return &result, nil
}

func autoEvaluate(treeID, nodeID string, r *Result, cfg *Config) error {
	var eval string
	var score float64

	switch r.Status {
	case "improved":
		eval = "sure"
		score = improvementScore(r.Metric, cfg)
	case "regressed":
		eval = "maybe"
		score = 0.2
	default:
		eval = "impossible"
		score = 0.0
	}
	_, err := tree.EvaluateThought(treeID, nodeID, eval, &score)
	return err
}

func improvementScore(metric *float64, cfg *Config) float64 {
	if metric == nil || cfg.BaselineMetric == nil {
		return 0.7
	}
	baseline := *cfg.BaselineMetric
	var delta float64
	if cfg.MetricDirection == "lower" {
		delta = baseline - *metric
	} else {
		delta = *metric - baseline
	}
	if delta <= 0 {
		return 0.2
	}
	pct := delta / math.Abs(baseline)
	return math.Min(0.99, 0.7+pct*6)
}

func isImproved(metric float64, cfg *Config) bool {
	if cfg.BaselineMetric == nil {
		return true
	}
	if cfg.MetricDirection == "lower" {
		return metric < *cfg.BaselineMetric
	}
	return metric > *cfg.BaselineMetric
}

func parseMetric(output, regex string) *float64 {
	re, err := regexp.Compile("(?m)" + regex)
	if err != nil {
		return nil
	}
	match := re.FindStringSubmatch(output)
	if len(match) < 2 {
		return nil
	}
	var v float64
	_, err = fmt.Sscanf(match[1], "%f", &v)
	if err != nil {
		return nil
	}
	return &v
}

// --- Command execution ---

func runCmd(command, cwd string, timeout int, logPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	// Use shell execution to properly handle quoted args and pipes
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	cmd.Dir = cwd

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	err := cmd.Run()

	// Write log
	os.WriteFile(logPath, buf.Bytes(), 0o644)

	return buf.String(), err
}

func readTail(path string, lines int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return "(no log)"
	}
	all := strings.Split(string(data), "\n")
	if len(all) <= lines {
		return string(data)
	}
	return strings.Join(all[len(all)-lines:], "\n")
}

// --- Git helpers ---

func git(args []string, cwd string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, _ := cmd.Output()
	return strings.TrimSpace(string(out))
}

func gitShort(cwd string) string {
	return git([]string{"rev-parse", "--short=7", "HEAD"}, cwd)
}

func ensureBranch(cfg *Config, treeID string) string {
	name := cfg.GitBranchPrefix + "/" + treeID[:8]
	current := git([]string{"rev-parse", "--abbrev-ref", "HEAD"}, cfg.WorkDir)
	if current == name {
		return name
	}
	// Try checkout, create if needed
	cmd := exec.Command("git", "checkout", name)
	cmd.Dir = cfg.WorkDir
	if err := cmd.Run(); err != nil {
		exec.Command("git", "checkout", "-b", name).Run()
	}
	return name
}

func gitCommit(cwd, msg string) (string, error) {
	if err := exec.Command("git", "-C", cwd, "add", "-A").Run(); err != nil {
		return "", fmt.Errorf("git add: %w", err)
	}
	if err := exec.Command("git", "-C", cwd, "commit", "-m", msg).Run(); err != nil {
		return "", fmt.Errorf("git commit: %w", err)
	}
	return gitShort(cwd), nil
}

func gitReset(cwd, hash string) error {
	return exec.Command("git", "-C", cwd, "reset", "--hard", hash).Run()
}

// --- History ---

// History returns experiment stats.
func History(treeID string) map[string]any {
	d := db.Get()
	var total, improved, crashed int
	d.QueryRow(`SELECT COUNT(*) FROM experiment_results WHERE tree_id=?`, treeID).Scan(&total)
	d.QueryRow(`SELECT COUNT(*) FROM experiment_results WHERE tree_id=? AND status='improved'`, treeID).Scan(&improved)
	d.QueryRow(`SELECT COUNT(*) FROM experiment_results WHERE tree_id=? AND status='crashed'`, treeID).Scan(&crashed)

	cfg, _ := GetConfig(treeID)
	var baseline *float64
	if cfg != nil {
		baseline = cfg.BaselineMetric
	}

	rate := 0
	if total > 0 {
		rate = improved * 100 / total
	}

	return map[string]any{
		"totalExperiments": total,
		"improved":         improved,
		"crashed":          crashed,
		"discarded":        total - improved - crashed,
		"successRate":      rate,
		"currentBaseline":  baseline,
	}
}

func logResult(treeID, nodeID string, r *Result) {
	d := db.Get()
	d.Exec(`INSERT INTO experiment_results (tree_id,node_id,status,metric,memory_mb,duration_seconds,commit_hash,kept,log_tail,created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?)`,
		treeID, nodeID, r.Status, r.Metric, r.MemoryMB, r.DurationSecs, r.CommitHash,
		boolToInt(r.Kept), r.LogTail, time.Now().UTC().Format(time.RFC3339))
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
