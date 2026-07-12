package workflow

import (
	"testing"
)

func TestNewParser(t *testing.T) {
	p := NewParser()
	if p == nil {
		t.Fatal("expected non-nil parser")
	}
}

func TestParseFileNotFound(t *testing.T) {
	p := NewParser()
	_, err := p.ParseFile("/nonexistent/file.yml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestGetActionRef(t *testing.T) {
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
		{"org/repo/path@v1", "org/repo", "path", "v1"},
		{"simple", "", "simple", ""},
	}

	for _, tt := range tests {
		owner, name, ref := GetActionRef(tt.uses)
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

func TestIsPinned(t *testing.T) {
	if !IsPinned("actions/checkout@e2b06a2d56b61f0b1a9e9d2a2b0f8c9d4e5f6a7b") {
		t.Error("SHA-pinned action should be detected as pinned")
	}
	if IsPinned("actions/checkout@v4") {
		t.Error("tag-pinned action should not be detected as SHA-pinned")
	}
	if IsPinned("actions/checkout") {
		t.Error("unpinned action should not be detected as pinned")
	}
}

func TestIsPinnedToTag(t *testing.T) {
	if !IsPinnedToTag("actions/checkout@v4") {
		t.Error("tag-pinned action should be detected")
	}
	if !IsPinnedToTag("actions/setup-go@V5") {
		t.Error("capital V tag should be detected")
	}
	if IsPinnedToTag("actions/checkout@e2b06a2d56b61f0b1a9e9d2a2b0f8c9d4e5f6a7b") {
		t.Error("SHA-pinned action should not be detected as tag-pinned")
	}
}

func TestGetTriggers(t *testing.T) {
	wf := &Workflow{
		On: Trigger{
			Push:        &EventTrigger{},
			PullRequest: &EventTrigger{},
			Other: map[string]interface{}{
				"workflow_run": map[string]interface{}{},
			},
		},
	}

	triggers := wf.GetTriggers()
	if len(triggers) != 3 {
		t.Errorf("expected 3 triggers, got %d: %v", len(triggers), triggers)
	}
}

func TestGetJobCount(t *testing.T) {
	wf := &Workflow{
		Jobs: map[string]Job{
			"job1": {},
			"job2": {},
		},
	}
	if wf.GetJobCount() != 2 {
		t.Errorf("expected 2 jobs, got %d", wf.GetJobCount())
	}
}

func TestGetStepCount(t *testing.T) {
	wf := &Workflow{
		Jobs: map[string]Job{
			"job1": {Steps: []Step{{}, {}}},
			"job2": {Steps: []Step{{}}},
		},
	}
	if wf.GetStepCount() != 3 {
		t.Errorf("expected 3 steps, got %d", wf.GetStepCount())
	}
}

func TestGetAllActions(t *testing.T) {
	wf := &Workflow{
		Jobs: map[string]Job{
			"job1": {
				Steps: []Step{
					{Uses: "actions/checkout@v4"},
					{Uses: "actions/setup-go@v5"},
				},
			},
			"job2": {
				Steps: []Step{
					{Uses: "actions/checkout@v4"},
				},
			},
		},
	}

	actions := wf.GetAllActions()
	if len(actions) != 2 {
		t.Errorf("expected 2 unique actions, got %d: %v", len(actions), actions)
	}
}

func TestGetRunSteps(t *testing.T) {
	wf := &Workflow{
		Jobs: map[string]Job{
			"job1": {
				Steps: []Step{
					{Run: "echo hello"},
					{Uses: "actions/checkout@v4"},
					{Run: "echo world"},
				},
			},
		},
	}

	steps := wf.GetRunSteps()
	if len(steps) != 2 {
		t.Errorf("expected 2 run steps, got %d", len(steps))
	}
}

func TestGetJobDependencies(t *testing.T) {
	wf := &Workflow{
		Jobs: map[string]Job{
			"build":  {Steps: []Step{{Run: "echo build"}}},
			"test":   {Needs: "build", Steps: []Step{{Run: "echo test"}}},
			"deploy": {Needs: []interface{}{"build", "test"}, Steps: []Step{{Run: "echo deploy"}}},
		},
	}

	deps := wf.GetJobDependencies()
	if len(deps) != 3 {
		t.Errorf("expected 3 entries, got %d", len(deps))
	}

	// test depends on build
	testDeps, ok := deps["test"]
	if !ok || len(testDeps) != 1 || testDeps[0] != "build" {
		t.Errorf("test should depend on build, got %v", testDeps)
	}

	// deploy depends on build and test
	deployDeps, ok := deps["deploy"]
	if !ok || len(deployDeps) != 2 {
		t.Errorf("deploy should depend on 2 jobs, got %v", deployDeps)
	}
}

func TestHasMatrixStrategy(t *testing.T) {
	wf := &Workflow{
		Jobs: map[string]Job{
			"test": {
				Strategy: &Strategy{
					Matrix: map[string]interface{}{
						"go": []interface{}{"1.24", "1.25"},
					},
				},
			},
		},
	}
	if !wf.HasMatrixStrategy() {
		t.Error("expected HasMatrixStrategy to be true")
	}
}

func TestGetUniqueRunners(t *testing.T) {
	wf := &Workflow{
		Jobs: map[string]Job{
			"job1": {RunsOn: "ubuntu-latest"},
			"job2": {RunsOn: []interface{}{"ubuntu-latest", "windows-latest"}},
		},
	}

	runners := wf.GetUniqueRunners()
	if len(runners) != 2 {
		t.Errorf("expected 2 unique runners, got %d: %v", len(runners), runners)
	}
}

func TestWorkflowString(t *testing.T) {
	wf := &Workflow{
		Name: "Test",
		Jobs: map[string]Job{
			"job1": {Steps: []Step{{Run: "echo hello"}}},
		},
	}

	s := wf.String()
	if s != "Workflow: Test (jobs: 1, steps: 1)" {
		t.Errorf("unexpected string: %s", s)
	}
}
