package workflow

import (
	"fmt"
	"strings"
)

// Workflow represents a parsed GitHub Actions workflow file
type Workflow struct {
	Name     string            `yaml:"name"`
	On       Trigger           `yaml:"on"`
	Env      map[string]string `yaml:"env"`
	Jobs     map[string]Job    `yaml:"jobs"`
	FilePath string            `yaml:"-"`
	RawYAML  string            `yaml:"-"`
}

// Trigger represents the workflow trigger configuration
type Trigger struct {
	Push             *EventTrigger           `yaml:"push,omitempty"`
	PullRequest      *EventTrigger           `yaml:"pull_request,omitempty"`
	WorkflowDispatch *map[string]interface{} `yaml:"workflow_dispatch,omitempty"`
	Schedule         []ScheduleTrigger       `yaml:"schedule,omitempty"`
	Release          *EventTrigger           `yaml:"release,omitempty"`
	Other            map[string]interface{}  `yaml:"-"`
}

// EventTrigger represents a push/pull_request/release trigger
type EventTrigger struct {
	Branches []string `yaml:"branches,omitempty"`
	Tags     []string `yaml:"tags,omitempty"`
	Paths    []string `yaml:"paths,omitempty"`
	Types    []string `yaml:"types,omitempty"`
}

// ScheduleTrigger represents a cron schedule
type ScheduleTrigger struct {
	Cron string `yaml:"cron"`
}

// Job represents a workflow job
type Job struct {
	Name           string                 `yaml:"name"`
	RunsOn         interface{}            `yaml:"runs-on"`
	Needs          interface{}            `yaml:"needs"`
	Steps          []Step                 `yaml:"steps"`
	Env            map[string]string      `yaml:"env"`
	TimeoutMinutes int                    `yaml:"timeout-minutes"`
	If             string                 `yaml:"if,omitempty"`
	Permissions    map[string]string      `yaml:"permissions,omitempty"`
	Strategy       *Strategy              `yaml:"strategy,omitempty"`
	Container      interface{}            `yaml:"container,omitempty"`
	Services       map[string]interface{} `yaml:"services,omitempty"`
	Defaults       *Defaults              `yaml:"defaults,omitempty"`
	Output         map[string]string      `yaml:"-"`
}

// Strategy represents the matrix strategy
type Strategy struct {
	Matrix      map[string]interface{} `yaml:"matrix"`
	FailFast    *bool                  `yaml:"fail-fast,omitempty"`
	MaxParallel int                    `yaml:"max-parallel,omitempty"`
}

// Step represents a job step
type Step struct {
	Name             string                 `yaml:"name"`
	ID               string                 `yaml:"id,omitempty"`
	If               string                 `yaml:"if,omitempty"`
	Uses             string                 `yaml:"uses,omitempty"`
	Run              string                 `yaml:"run,omitempty"`
	Shell            string                 `yaml:"shell,omitempty"`
	WorkingDirectory string                 `yaml:"working-directory,omitempty"`
	With             map[string]interface{} `yaml:"with,omitempty"`
	Env              map[string]string      `yaml:"env,omitempty"`
	ContinueOnError  interface{}            `yaml:"continue-on-error,omitempty"`
	TimeoutMinutes   int                    `yaml:"timeout-minutes,omitempty"`
}

// Defaults represents default settings for jobs
type Defaults struct {
	Run *RunDefaults `yaml:"run,omitempty"`
}

// RunDefaults represents defaults for run steps
type RunDefaults struct {
	Shell string `yaml:"shell,omitempty"`
}

// GetActionRef extracts the action reference from a "uses" string
// e.g., "actions/checkout@v4" -> ("actions/checkout", "v4")
func GetActionRef(uses string) (owner, name, ref string) {
	// Handle local actions (./path)
	if strings.HasPrefix(uses, "./") {
		return "", uses, ""
	}

	// Handle docker actions
	if strings.HasPrefix(uses, "docker://") {
		return "", uses, ""
	}

	parts := strings.SplitN(uses, "@", 2)
	if len(parts) == 1 {
		return "", uses, ""
	}

	ref = parts[1]
	pathParts := strings.Split(parts[0], "/")
	if len(pathParts) == 2 {
		return pathParts[0], pathParts[1], ref
	}
	if len(pathParts) == 3 {
		return pathParts[0] + "/" + pathParts[1], pathParts[2], ref
	}
	return parts[0], "", ref
}

// IsPinned checks if an action is pinned to a SHA
func IsPinned(uses string) bool {
	_, _, ref := GetActionRef(uses)
	// SHA is 40 hex characters
	return len(ref) == 40 && isHexString(ref)
}

// IsPinnedToTag checks if an action is pinned to a specific version tag
func IsPinnedToTag(uses string) bool {
	_, _, ref := GetActionRef(uses)
	return strings.HasPrefix(ref, "v") || strings.HasPrefix(ref, "V")
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// GetTriggers returns a list of trigger names
func (w *Workflow) GetTriggers() []string {
	var triggers []string
	if w.On.Push != nil {
		triggers = append(triggers, "push")
	}
	if w.On.PullRequest != nil {
		triggers = append(triggers, "pull_request")
	}
	if w.On.WorkflowDispatch != nil {
		triggers = append(triggers, "workflow_dispatch")
	}
	if len(w.On.Schedule) > 0 {
		triggers = append(triggers, "schedule")
	}
	if w.On.Release != nil {
		triggers = append(triggers, "release")
	}
	for k := range w.On.Other {
		triggers = append(triggers, k)
	}
	return triggers
}

// GetAllActions returns all unique actions used in the workflow
func (w *Workflow) GetAllActions() []string {
	actionSet := make(map[string]bool)
	for _, job := range w.Jobs {
		for _, step := range job.Steps {
			if step.Uses != "" {
				actionSet[step.Uses] = true
			}
		}
	}
	var actions []string
	for a := range actionSet {
		actions = append(actions, a)
	}
	return actions
}

// GetJobCount returns the number of jobs
func (w *Workflow) GetJobCount() int {
	return len(w.Jobs)
}

// GetStepCount returns the total number of steps
func (w *Workflow) GetStepCount() int {
	count := 0
	for _, job := range w.Jobs {
		count += len(job.Steps)
	}
	return count
}

// GetRunSteps returns all steps that use "run" (shell commands)
func (w *Workflow) GetRunSteps() []Step {
	var steps []Step
	for _, job := range w.Jobs {
		for _, step := range job.Steps {
			if step.Run != "" {
				steps = append(steps, step)
			}
		}
	}
	return steps
}

// GetActionsSteps returns all steps that use "uses" (actions)
func (w *Workflow) GetActionsSteps() []Step {
	var steps []Step
	for _, job := range w.Jobs {
		for _, step := range job.Steps {
			if step.Uses != "" {
				steps = append(steps, step)
			}
		}
	}
	return steps
}

// GetJobDependencies returns the dependency graph
func (w *Workflow) GetJobDependencies() map[string][]string {
	deps := make(map[string][]string)
	for jobName, job := range w.Jobs {
		needs := job.Needs
		switch n := needs.(type) {
		case string:
			deps[jobName] = []string{n}
		case []interface{}:
			var depList []string
			for _, d := range n {
				if ds, ok := d.(string); ok {
					depList = append(depList, ds)
				}
			}
			deps[jobName] = depList
		default:
			deps[jobName] = nil
		}
	}
	return deps
}

// GetConcurrencyGroup returns the concurrency group for the workflow
func (w *Workflow) GetConcurrencyGroup() string {
	// This would need to be extracted from the raw YAML
	// For now, return empty
	return ""
}

// HasMatrixStrategy checks if any job uses a matrix strategy
func (w *Workflow) HasMatrixStrategy() bool {
	for _, job := range w.Jobs {
		if job.Strategy != nil && job.Strategy.Matrix != nil {
			return true
		}
	}
	return false
}

// GetUniqueRunners returns unique runner labels used
func (w *Workflow) GetUniqueRunners() []string {
	runnerSet := make(map[string]bool)
	for _, job := range w.Jobs {
		switch r := job.RunsOn.(type) {
		case string:
			runnerSet[r] = true
		case []interface{}:
			for _, v := range r {
				if s, ok := v.(string); ok {
					runnerSet[s] = true
				}
			}
		}
	}
	var runners []string
	for r := range runnerSet {
		runners = append(runners, r)
	}
	return runners
}

// String returns a summary string
func (w *Workflow) String() string {
	return fmt.Sprintf("Workflow: %s (jobs: %d, steps: %d)",
		w.Name, w.GetJobCount(), w.GetStepCount())
}
