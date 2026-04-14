package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All MCP tool names registered in main.go.
var registeredTools = map[string]bool{
	"open_dashboard": true, "create_tree": true, "generate_thoughts": true,
	"evaluate_thought": true, "search_step": true, "backtrack": true,
	"mark_solution": true, "get_best_path": true, "get_tree_summary": true,
	"inspect_node": true, "get_frontier": true, "get_all_paths": true,
	"list_trees": true, "route_problem": true, "abandon_tree": true,
	"resume_tree": true, "retrieve_context": true, "store_solution": true,
	"retrieval_stats": true, "compact_analyze": true, "compact_apply": true,
	"compact_restore": true, "configure_experiment": true, "prepare_experiment": true,
	"execute_experiment": true, "experiment_history": true, "link_trees": true,
	"get_tree_links": true, "suggest_next": true, "audit_log": true,
	"link_solutions": true, "get_solution_links": true, "lint_knowledge": true,
	"knowledge_log": true, "get_tree_context": true, "drift_scan": true,
	"knowledge_report": true, "knowledge_graph": true, "ingest_url": true,
}

// All agent names shipped in agents/ directory.
var registeredAgents = map[string]bool{
	"conductor": true, "critic": true, "experimenter": true,
	"librarian": true, "researcher": true, "scout": true, "synthesizer": true,
}

func TestAllSkillsHaveValidFrontmatter(t *testing.T) {
	skills, err := filepath.Glob("skills/*/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(skills) < 21 {
		t.Fatalf("expected at least 21 skills, found %d", len(skills))
	}

	for _, path := range skills {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(content)
		name := filepath.Base(filepath.Dir(path))

		// Must have frontmatter delimiters
		if !strings.HasPrefix(text, "---\n") {
			t.Errorf("skill %s: missing opening frontmatter ---", name)
			continue
		}
		endIdx := strings.Index(text[4:], "\n---\n")
		if endIdx == -1 {
			t.Errorf("skill %s: missing closing frontmatter ---", name)
			continue
		}
		frontmatter := text[4 : 4+endIdx]

		// Must have name field
		if !strings.Contains(frontmatter, "name:") {
			t.Errorf("skill %s: missing name in frontmatter", name)
		}

		// Must have description field
		if !strings.Contains(frontmatter, "description:") {
			t.Errorf("skill %s: missing description in frontmatter", name)
		}

		// Body must not be empty
		body := strings.TrimSpace(text[4+endIdx+5:])
		if len(body) < 50 {
			t.Errorf("skill %s: body too short (%d chars)", name, len(body))
		}
	}
}

func TestAllAgentsHaveValidFrontmatter(t *testing.T) {
	agents, err := filepath.Glob("agents/*.md")
	if err != nil {
		t.Fatal(err)
	}
	if len(agents) < 7 {
		t.Fatalf("expected at least 7 agents, found %d", len(agents))
	}

	requiredFields := []string{"name:", "description:", "model:", "maxTurns:"}

	for _, path := range agents {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(content)
		name := strings.TrimSuffix(filepath.Base(path), ".md")

		if !strings.HasPrefix(text, "---\n") {
			t.Errorf("agent %s: missing opening frontmatter", name)
			continue
		}
		endIdx := strings.Index(text[4:], "\n---\n")
		if endIdx == -1 {
			t.Errorf("agent %s: missing closing frontmatter", name)
			continue
		}
		frontmatter := text[4 : 4+endIdx]

		for _, field := range requiredFields {
			if !strings.Contains(frontmatter, field) {
				t.Errorf("agent %s: missing %s in frontmatter", name, field)
			}
		}

		// Model must be valid
		validModels := []string{"sonnet", "opus", "haiku"}
		hasValidModel := false
		for _, m := range validModels {
			if strings.Contains(frontmatter, "model: "+m) {
				hasValidModel = true
				break
			}
		}
		if !hasValidModel {
			t.Errorf("agent %s: invalid or missing model (expected sonnet/opus/haiku)", name)
		}

		// Body must not be empty
		body := strings.TrimSpace(text[4+endIdx+5:])
		if len(body) < 100 {
			t.Errorf("agent %s: system prompt too short (%d chars)", name, len(body))
		}
	}
}

func TestSkillsReferenceValidTools(t *testing.T) {
	skills, _ := filepath.Glob("skills/*/SKILL.md")

	// MCP tool names that might appear in skill text as backtick-quoted references
	for _, path := range skills {
		content, _ := os.ReadFile(path)
		text := string(content)
		name := filepath.Base(filepath.Dir(path))

		// Find backtick-quoted tool references
		for i := 0; i < len(text)-1; i++ {
			if text[i] == '`' {
				end := strings.IndexByte(text[i+1:], '`')
				if end == -1 {
					break
				}
				ref := text[i+1 : i+1+end]
				// Check if it looks like an MCP tool name (snake_case, no spaces)
				if strings.Contains(ref, "_") && !strings.Contains(ref, " ") && !strings.Contains(ref, "/") && !strings.Contains(ref, ".") && len(ref) < 30 {
					if !registeredTools[ref] {
						// Only warn for things that look like they SHOULD be tool names
						// Skip known non-tool patterns
						skip := false
						for _, prefix := range []string{"tree_id", "node_id", "parent_id", "solution_id", "source_id", "target_id", "link_type", "search_strategy", "max_depth", "branching_factor", "top_k", "max_tokens", "min_age_days", "target_file", "run_command", "metric_regex", "metric_direction", "timeout_seconds", "work_dir", "git_branch_prefix", "log_file", "memory_regex", "baseline_metric", "patch_content", "commit_message", "previous_hash", "source_tree", "target_tree"} {
							if ref == prefix {
								skip = true
								break
							}
						}
						if !skip {
							t.Logf("skill %s references `%s` — not a registered tool (may be intentional)", name, ref)
						}
					}
				}
			}
		}
	}
}

func TestAgentsReferenceValidAgentNames(t *testing.T) {
	// The conductor agent should only reference agents that exist
	content, err := os.ReadFile("agents/conductor.md")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)

	expectedRefs := []string{"scout", "researcher", "critic", "experimenter", "librarian", "synthesizer"}
	for _, ref := range expectedRefs {
		if !strings.Contains(text, ref) {
			t.Errorf("conductor doesn't reference agent %q", ref)
		}
	}
}

func TestWorkflowSkillsReferenceValidAgents(t *testing.T) {
	workflowSkills := []string{
		"skills/research-and-validate/SKILL.md",
		"skills/experiment-loop/SKILL.md",
		"skills/knowledge-maintenance/SKILL.md",
	}

	for _, path := range workflowSkills {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(content)
		name := filepath.Base(filepath.Dir(path))

		// Workflow skills must reference tree-of-thoughts: agents
		if !strings.Contains(text, "tree-of-thoughts:") {
			t.Errorf("workflow %s: should reference tree-of-thoughts: agents", name)
		}

		// Must have Context Baton sections
		if !strings.Contains(text, "Context Baton") {
			t.Errorf("workflow %s: missing Context Baton handoff sections", name)
		}

		// Must have Stage numbering
		if !strings.Contains(text, "Stage 1") {
			t.Errorf("workflow %s: missing Stage numbering", name)
		}
	}
}

func TestVerificationSensorsHaveChecks(t *testing.T) {
	sensors := []struct {
		path      string
		minChecks int
	}{
		{"skills/verify-research/SKILL.md", 6},
		{"skills/verify-experiment/SKILL.md", 6},
		{"skills/verify-knowledge/SKILL.md", 6},
	}

	for _, s := range sensors {
		content, err := os.ReadFile(s.path)
		if err != nil {
			t.Fatalf("read %s: %v", s.path, err)
		}
		text := string(content)
		name := filepath.Base(filepath.Dir(s.path))

		// Count PASS + FAIL + WARN criteria (sensors use all three verdicts)
		passCount := strings.Count(text, "**PASS:**")
		failCount := strings.Count(text, "**FAIL:**")
		warnCount := strings.Count(text, "**WARN:**")
		totalCriteria := passCount + failCount + warnCount

		if passCount < s.minChecks {
			t.Errorf("sensor %s: has %d PASS criteria, expected at least %d", name, passCount, s.minChecks)
		}
		if totalCriteria < s.minChecks*2 {
			t.Errorf("sensor %s: has %d total criteria (PASS+FAIL+WARN), expected at least %d", name, totalCriteria, s.minChecks*2)
		}

		// Must have Output section
		if !strings.Contains(text, "## Output") || !strings.Contains(text, "Overall:") {
			t.Errorf("sensor %s: missing Output section with Overall verdict", name)
		}
	}
}

func TestPluginManifestValid(t *testing.T) {
	content, err := os.ReadFile(".claude-plugin/plugin.json")
	if err != nil {
		t.Fatalf("read plugin.json: %v", err)
	}
	text := string(content)

	required := []string{`"name"`, `"version"`, `"description"`, `"author"`, `"repository"`}
	for _, field := range required {
		if !strings.Contains(text, field) {
			t.Errorf("plugin.json missing field %s", field)
		}
	}
}

func TestMCPConfigValid(t *testing.T) {
	content, err := os.ReadFile(".mcp.json")
	if err != nil {
		t.Fatalf("read .mcp.json: %v", err)
	}
	text := string(content)

	if !strings.Contains(text, "CLAUDE_PLUGIN_DATA") {
		t.Error(".mcp.json should reference CLAUDE_PLUGIN_DATA for binary path")
	}
	if !strings.Contains(text, "tree-of-thoughts") {
		t.Error(".mcp.json should define tree-of-thoughts server")
	}
}

func TestHooksConfigValid(t *testing.T) {
	content, err := os.ReadFile("hooks/hooks.json")
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}
	text := string(content)

	// Check all hook events are present
	requiredEvents := []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse", "Stop"}
	for _, event := range requiredEvents {
		if !strings.Contains(text, event) {
			t.Errorf("hooks.json missing event %s", event)
		}
	}

	// Check all scripts are referenced
	requiredScripts := []string{
		"install.sh",
		"session-briefing.sh",
		"prior-knowledge-check.sh",
		"check-duplicate-tree.sh",
		"check-experiment-safety.sh",
		"verify-after-solution.sh",
		"lint-after-store.sh",
		"session-quality-gate.sh",
	}
	for _, script := range requiredScripts {
		if !strings.Contains(text, script) {
			t.Errorf("hooks.json missing reference to %s", script)
		}
	}

	// Check tool matchers
	requiredMatchers := []string{"create_tree", "execute_experiment", "mark_solution", "store_solution|ingest_url"}
	for _, matcher := range requiredMatchers {
		if !strings.Contains(text, matcher) {
			t.Errorf("hooks.json missing matcher %q", matcher)
		}
	}

	// Check all paths use CLAUDE_PLUGIN_ROOT
	if strings.Count(text, "CLAUDE_PLUGIN_ROOT") < len(requiredScripts) {
		t.Error("some hook commands may not use ${CLAUDE_PLUGIN_ROOT} prefix")
	}
}

func TestAllHookScriptsExecutable(t *testing.T) {
	scripts := []string{
		"scripts/install.sh",
		"scripts/session-briefing.sh",
		"scripts/prior-knowledge-check.sh",
		"scripts/check-duplicate-tree.sh",
		"scripts/check-experiment-safety.sh",
		"scripts/verify-after-solution.sh",
		"scripts/lint-after-store.sh",
		"scripts/session-quality-gate.sh",
	}
	for _, path := range scripts {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s: not found", path)
			continue
		}
		if info.Mode()&0111 == 0 {
			t.Errorf("%s: not executable", path)
		}
	}
}

func TestAllHookScriptsHaveShebang(t *testing.T) {
	scripts, _ := filepath.Glob("scripts/*.sh")
	for _, path := range scripts {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)
			continue
		}
		if !strings.HasPrefix(string(content), "#!/") {
			t.Errorf("%s: missing shebang line", path)
		}
	}
}

func TestBinWrapperExecutable(t *testing.T) {
	info, err := os.Stat("bin/tot-mcp")
	if err != nil {
		t.Fatalf("stat bin/tot-mcp: %v", err)
	}
	if info.Mode()&0111 == 0 {
		t.Error("bin/tot-mcp is not executable")
	}
}
