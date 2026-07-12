package analyzer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/EdgarOrtegaRamirez/actionforge/internal/workflow"
)

// Severity represents the severity level of an issue
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityError:
		return "ERROR"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// MarshalJSON implements json.Marshaler for Severity
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// Category represents the category of an issue
type Category string

const (
	CategorySecurity     Category = "security"
	CategoryBestPractice Category = "best-practice"
	CategoryOptimization Category = "optimization"
	CategoryPerformance  Category = "performance"
	CategoryMaintenance  Category = "maintenance"
)

// Issue represents a detected issue
type Issue struct {
	Severity    Severity `json:"severity"`
	Category    Category `json:"category"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Location    string   `json:"location,omitempty"`
	Suggestion  string   `json:"suggestion,omitempty"`
}

// Score represents the overall score
type Score struct {
	Value       int    `json:"value"`
	Grade       string `json:"grade"`
	Description string `json:"description"`
}

// AnalysisResult contains the complete analysis result
type AnalysisResult struct {
	Workflow *workflow.Workflow
	Issues   []Issue
	Score    Score
	Summary  Summary
}

// Summary contains high-level statistics
type Summary struct {
	JobCount          int
	StepCount         int
	ActionCount       int
	TriggerCount      int
	RunnerCount       int
	HasCaching        bool
	HasMatrix         bool
	HasConcurrency    bool
	HasTimeout        bool
	HasPermissions    bool
	UnpinnedActions   int
	DeprecatedActions int
}

// Analyzer analyzes GitHub Actions workflows
type Analyzer struct {
	// Known deprecated actions
	deprecatedActions map[string]string
	// Known actions with security implications
	riskyActions map[string]string
}

// NewAnalyzer creates a new analyzer
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		deprecatedActions: map[string]string{
			"actions/checkout@v1":          "Use actions/checkout@v4",
			"actions/checkout@v2":          "Use actions/checkout@v4",
			"actions/checkout@v3":          "Use actions/checkout@v4",
			"actions/setup-node@v1":        "Use actions/setup-node@v4",
			"actions/setup-node@v2":        "Use actions/setup-node@v4",
			"actions/setup-node@v3":        "Use actions/setup-node@v4",
			"actions/setup-python@v1":      "Use actions/setup-python@v5",
			"actions/setup-python@v2":      "Use actions/setup-python@v5",
			"actions/setup-python@v3":      "Use actions/setup-python@v5",
			"actions/setup-python@v4":      "Use actions/setup-python@v5",
			"actions/setup-go@v1":          "Use actions/setup-go@v5",
			"actions/setup-go@v2":          "Use actions/setup-go@v5",
			"actions/setup-go@v3":          "Use actions/setup-go@v5",
			"actions/setup-go@v4":          "Use actions/setup-go@v5",
			"actions/upload-artifact@v1":   "Use actions/upload-artifact@v4",
			"actions/upload-artifact@v2":   "Use actions/upload-artifact@v4",
			"actions/upload-artifact@v3":   "Use actions/upload-artifact@v4",
			"actions/download-artifact@v1": "Use actions/download-artifact@v4",
			"actions/download-artifact@v2": "Use actions/download-artifact@v4",
			"actions/download-artifact@v3": "Use actions/download-artifact@v4",
		},
		riskyActions: map[string]string{
			"actions/github-script":           "Grants elevated GITHUB_TOKEN permissions",
			"peter-evans/create-pull-request": "Creates PRs automatically",
			"peter-evans/commit-comment":      "Posts commit comments",
			"codecov/codecov-action":          "Uploads coverage to external service",
			"peaceiris/actions-gh-pages":      "Deploys to GitHub Pages",
		},
	}
}

// Analyze performs a complete analysis of a workflow
func (a *Analyzer) Analyze(wf *workflow.Workflow) *AnalysisResult {
	result := &AnalysisResult{
		Workflow: wf,
		Issues:   make([]Issue, 0),
	}

	// Run all analyzers
	a.checkSecurity(wf, result)
	a.checkBestPractices(wf, result)
	a.checkOptimization(wf, result)
	a.checkPerformance(wf, result)
	a.checkMaintenance(wf, result)

	// Calculate summary
	result.Summary = a.calculateSummary(wf)

	// Calculate score
	result.Score = a.calculateScore(result)

	return result
}

// checkSecurity performs security-related checks
func (a *Analyzer) checkSecurity(wf *workflow.Workflow, result *AnalysisResult) {
	// Check for unpinned actions
	for jobName, job := range wf.Jobs {
		for i, step := range job.Steps {
			if step.Uses != "" {
				if !workflow.IsPinned(step.Uses) {
					severity := SeverityWarning
					if !workflow.IsPinnedToTag(step.Uses) {
						severity = SeverityError
					}
					result.Issues = append(result.Issues, Issue{
						Severity: severity,
						Category: CategorySecurity,
						Title:    "Action not pinned to SHA",
						Description: fmt.Sprintf("Action '%s' is not pinned to a commit SHA. "+
							"This allows the action to be modified by the owner at any time.",
							step.Uses),
						Location:   fmt.Sprintf("jobs.%s.steps[%d]", jobName, i),
						Suggestion: fmt.Sprintf("Pin to a specific SHA: %s@<commit-sha>", strings.Split(step.Uses, "@")[0]),
					})
				}
			}
		}
	}

	// Check for overly permissive GITHUB_TOKEN
	for jobName, job := range wf.Jobs {
		if job.Permissions != nil {
			if writeAll, ok := job.Permissions["contents"]; ok && writeAll == "write" {
				result.Issues = append(result.Issues, Issue{
					Severity:    SeverityWarning,
					Category:    CategorySecurity,
					Title:       "Overly permissive GITHUB_TOKEN",
					Description: "Job has 'contents: write' permission. Use the principle of least privilege.",
					Location:    fmt.Sprintf("jobs.%s.permissions", jobName),
					Suggestion:  "Set permissions to the minimum required level",
				})
			}
		}
	}

	// Check for secret exposure in env
	for jobName, job := range wf.Jobs {
		for envKey, envVal := range job.Env {
			if strings.Contains(envVal, "${{") && strings.Contains(envVal, "secrets.") {
				// This is expected - secrets should be accessed via ${{ secrets.X }}
				// But warn if they're set at the job level (might expose to all steps)
				result.Issues = append(result.Issues, Issue{
					Severity:    SeverityInfo,
					Category:    CategorySecurity,
					Title:       "Secret used in job-level environment",
					Description: fmt.Sprintf("Secret '%s' is set at the job level, making it available to all steps.", envKey),
					Location:    fmt.Sprintf("jobs.%s.env", jobName),
					Suggestion:  "Consider setting secrets only at the step level where needed",
				})
			}
		}
	}

	// Check for dangerous patterns in run steps
	dangerousPatterns := []struct {
		pattern     *regexp.Regexp
		title       string
		description string
	}{
		{
			pattern:     regexp.MustCompile(`\$\{\{.*github\.event\.issue\.body.*\}\}`),
			title:       "Potential injection via issue body",
			description: "Using issue body in a run step can lead to code injection",
		},
		{
			pattern:     regexp.MustCompile(`\$\{\{.*github\.event\.pull_request\.title.*\}\}`),
			title:       "Potential injection via PR title",
			description: "Using PR title in a run step can lead to code injection",
		},
		{
			pattern:     regexp.MustCompile(`\$\{\{.*github\.event\.comment\.body.*\}\}`),
			title:       "Potential injection via comment body",
			description: "Using comment body in a run step can lead to code injection",
		},
		{
			pattern:     regexp.MustCompile(`curl.*\|\s*sh`),
			title:       "Piping curl to shell",
			description: "Downloading and executing scripts directly can be dangerous",
		},
		{
			pattern:     regexp.MustCompile(`curl.*\|\s*bash`),
			title:       "Piping curl to bash",
			description: "Downloading and executing scripts directly can be dangerous",
		},
	}

	for jobName, job := range wf.Jobs {
		for i, step := range job.Steps {
			if step.Run != "" {
				for _, dp := range dangerousPatterns {
					if dp.pattern.MatchString(step.Run) {
						result.Issues = append(result.Issues, Issue{
							Severity:    SeverityCritical,
							Category:    CategorySecurity,
							Title:       dp.title,
							Description: dp.description,
							Location:    fmt.Sprintf("jobs.%s.steps[%d]", jobName, i),
							Suggestion:  "Download the script first, verify its integrity, then execute it",
						})
					}
				}
			}
		}
	}

	// Check for pull_request_target with checkout
	if hasPullRequestTarget(wf) {
		for jobName, job := range wf.Jobs {
			for i, step := range job.Steps {
				if step.Uses == "actions/checkout@v4" || step.Uses == "actions/checkout@v3" {
					result.Issues = append(result.Issues, Issue{
						Severity:    SeverityCritical,
						Category:    CategorySecurity,
						Title:       "Checkout in pull_request_target workflow",
						Description: "Using checkout in a pull_request_target workflow can expose secrets to forked PRs",
						Location:    fmt.Sprintf("jobs.%s.steps[%d]", jobName, i),
						Suggestion:  "Use pull_request trigger instead, or carefully control the checkout ref",
					})
				}
			}
		}
	}
}

// checkBestPractices checks for best practice violations
func (a *Analyzer) checkBestPractices(wf *workflow.Workflow, result *AnalysisResult) {
	// Check for missing timeout
	for jobName, job := range wf.Jobs {
		if job.TimeoutMinutes == 0 {
			result.Issues = append(result.Issues, Issue{
				Severity:    SeverityWarning,
				Category:    CategoryBestPractice,
				Title:       "Missing timeout-minutes",
				Description: "Job has no timeout set. Jobs can run indefinitely and consume minutes.",
				Location:    fmt.Sprintf("jobs.%s", jobName),
				Suggestion:  "Add 'timeout-minutes: 30' (or appropriate value) to the job",
			})
		}
	}

	// Check for missing concurrency
	if len(wf.Jobs) > 1 && !hasConcurrency(wf) {
		result.Issues = append(result.Issues, Issue{
			Severity:    SeverityInfo,
			Category:    CategoryBestPractice,
			Title:       "No concurrency control",
			Description: "Multi-job workflow without concurrency control. Parallel runs may cause conflicts.",
			Location:    "workflow level",
			Suggestion:  "Add 'concurrency' group to prevent redundant runs",
		})
	}

	// Check for missing permissions at workflow level
	if !hasWorkflowPermissions(wf) {
		result.Issues = append(result.Issues, Issue{
			Severity:    SeverityWarning,
			Category:    CategoryBestPractice,
			Title:       "No workflow-level permissions",
			Description: "Workflow doesn't set top-level permissions. Default permissions may be too broad.",
			Location:    "workflow level",
			Suggestion:  "Add 'permissions: { contents: read }' at the workflow level",
		})
	}

	// Check for missing error handling
	for jobName, job := range wf.Jobs {
		for i, step := range job.Steps {
			if step.Run != "" {
				// Check for set -e or error handling
				if !strings.Contains(step.Run, "set -e") &&
					!strings.Contains(step.Run, "set -o errexit") &&
					!strings.Contains(step.Run, "set -euo pipefail") {
					// This is a warning, not error - many steps don't need explicit error handling
					if i > 0 && len(step.Run) > 100 {
						result.Issues = append(result.Issues, Issue{
							Severity:    SeverityInfo,
							Category:    CategoryBestPractice,
							Title:       "Consider adding error handling",
							Description: "Long run steps should handle errors explicitly",
							Location:    fmt.Sprintf("jobs.%s.steps[%d]", jobName, i),
							Suggestion:  "Add 'set -euo pipefail' or handle errors with 'if' conditions",
						})
					}
				}
			}
		}
	}

	// Check for hardcoded versions
	for jobName, job := range wf.Jobs {
		if runners, ok := job.RunsOn.([]interface{}); ok {
			for _, r := range runners {
				if runnerStr, ok := r.(string); ok {
					if strings.Contains(runnerStr, "ubuntu-") && runnerStr != "ubuntu-latest" {
						result.Issues = append(result.Issues, Issue{
							Severity:    SeverityInfo,
							Category:    CategoryMaintenance,
							Title:       "Hardcoded runner version",
							Description: fmt.Sprintf("Using '%s' instead of 'ubuntu-latest'. May miss security updates.", runnerStr),
							Location:    fmt.Sprintf("jobs.%s.runs-on", jobName),
							Suggestion:  "Consider using 'ubuntu-latest' for automatic updates",
						})
					}
				}
			}
		}
	}
}

// checkOptimization checks for optimization opportunities
func (a *Analyzer) checkOptimization(wf *workflow.Workflow, result *AnalysisResult) {
	// Check for missing caching
	hasCaching := false
	for _, job := range wf.Jobs {
		for _, step := range job.Steps {
			if step.Uses != "" && strings.Contains(step.Uses, "cache") {
				hasCaching = true
			}
		}
	}
	if !hasCaching {
		result.Issues = append(result.Issues, Issue{
			Severity:    SeverityInfo,
			Category:    CategoryOptimization,
			Title:       "No caching detected",
			Description: "Workflow doesn't use caching. Adding cache can significantly speed up builds.",
			Location:    "workflow level",
			Suggestion:  "Add actions/cache or built-in cache in setup-node/setup-python",
		})
	}

	// Check for matrix opportunities
	if len(wf.Jobs) > 1 {
		similarJobs := findSimilarJobs(wf)
		if len(similarJobs) > 0 {
			for _, jobPair := range similarJobs {
				result.Issues = append(result.Issues, Issue{
					Severity:    SeverityInfo,
					Category:    CategoryOptimization,
					Title:       "Potential matrix strategy",
					Description: fmt.Sprintf("Jobs '%s' and '%s' have similar steps. Consider using matrix strategy.", jobPair[0], jobPair[1]),
					Location:    "jobs",
					Suggestion:  "Combine similar jobs using matrix strategy to reduce duplication",
				})
			}
		}
	}

	// Check for unnecessary checkouts
	for jobName, job := range wf.Jobs {
		checkoutCount := 0
		for _, step := range job.Steps {
			if step.Uses != "" && strings.Contains(step.Uses, "actions/checkout") {
				checkoutCount++
			}
		}
		if checkoutCount > 1 {
			result.Issues = append(result.Issues, Issue{
				Severity:    SeverityWarning,
				Category:    CategoryOptimization,
				Title:       "Multiple checkouts in job",
				Description: fmt.Sprintf("Job '%s' checks out code %d times. This may be unnecessary.", jobName, checkoutCount),
				Location:    fmt.Sprintf("jobs.%s", jobName),
				Suggestion:  "Remove redundant checkout steps",
			})
		}
	}
}

// checkPerformance checks for performance issues
func (a *Analyzer) checkPerformance(wf *workflow.Workflow, result *AnalysisResult) {
	// Check for slow runners
	for jobName, job := range wf.Jobs {
		if runner, ok := job.RunsOn.(string); ok {
			if runner == "self-hosted" {
				result.Issues = append(result.Issues, Issue{
					Severity:    SeverityInfo,
					Category:    CategoryPerformance,
					Title:       "Using self-hosted runner",
					Description: "Self-hosted runners may have different performance characteristics.",
					Location:    fmt.Sprintf("jobs.%s.runs-on", jobName),
					Suggestion:  "Ensure self-hosted runners are properly configured and maintained",
				})
			}
		}
	}

	// Check for sequential jobs that could be parallel
	deps := wf.GetJobDependencies()
	for jobName, jobDeps := range deps {
		if len(jobDeps) > 1 {
			// This job depends on multiple jobs - check if they could run in parallel
			result.Issues = append(result.Issues, Issue{
				Severity:    SeverityInfo,
				Category:    CategoryPerformance,
				Title:       "Complex dependency chain",
				Description: fmt.Sprintf("Job '%s' depends on %d jobs. Ensure dependencies are necessary.", jobName, len(jobDeps)),
				Location:    fmt.Sprintf("jobs.%s.needs", jobName),
				Suggestion:  "Review if all dependencies are required",
			})
		}
	}
}

// checkMaintenance checks for maintenance issues
func (a *Analyzer) checkMaintenance(wf *workflow.Workflow, result *AnalysisResult) {
	// Check for deprecated actions
	for jobName, job := range wf.Jobs {
		for i, step := range job.Steps {
			if step.Uses != "" {
				if suggestion, ok := a.deprecatedActions[step.Uses]; ok {
					result.Issues = append(result.Issues, Issue{
						Severity:    SeverityWarning,
						Category:    CategoryMaintenance,
						Title:       "Deprecated action version",
						Description: fmt.Sprintf("Action '%s' is deprecated.", step.Uses),
						Location:    fmt.Sprintf("jobs.%s.steps[%d]", jobName, i),
						Suggestion:  suggestion,
					})
				}
			}
		}
	}

	// Check for potential action updates
	for jobName, job := range wf.Jobs {
		for i, step := range job.Steps {
			if step.Uses != "" && workflow.IsPinnedToTag(step.Uses) {
				_, _, ref := workflow.GetActionRef(step.Uses)
				// Check for old major versions
				if ref == "v1" || ref == "v2" {
					result.Issues = append(result.Issues, Issue{
						Severity:    SeverityInfo,
						Category:    CategoryMaintenance,
						Title:       "Outdated action version",
						Description: fmt.Sprintf("Action '%s' is using an old version.", step.Uses),
						Location:    fmt.Sprintf("jobs.%s.steps[%d]", jobName, i),
						Suggestion:  "Check for newer versions of this action",
					})
				}
			}
		}
	}
}

// calculateSummary calculates summary statistics
func (a *Analyzer) calculateSummary(wf *workflow.Workflow) Summary {
	summary := Summary{
		JobCount:     wf.GetJobCount(),
		StepCount:    wf.GetStepCount(),
		ActionCount:  len(wf.GetAllActions()),
		TriggerCount: len(wf.GetTriggers()),
		RunnerCount:  len(wf.GetUniqueRunners()),
		HasMatrix:    wf.HasMatrixStrategy(),
	}

	// Check for caching
	for _, job := range wf.Jobs {
		for _, step := range job.Steps {
			if step.Uses != "" && strings.Contains(step.Uses, "cache") {
				summary.HasCaching = true
			}
		}
	}

	// Check for concurrency
	summary.HasConcurrency = hasConcurrency(wf)

	// Check for timeouts
	for _, job := range wf.Jobs {
		if job.TimeoutMinutes > 0 {
			summary.HasTimeout = true
			break
		}
	}

	// Check for permissions
	summary.HasPermissions = hasWorkflowPermissions(wf)

	// Count unpinned and deprecated actions
	for _, job := range wf.Jobs {
		for _, step := range job.Steps {
			if step.Uses != "" {
				if !workflow.IsPinned(step.Uses) {
					summary.UnpinnedActions++
				}
				if _, ok := a.deprecatedActions[step.Uses]; ok {
					summary.DeprecatedActions++
				}
			}
		}
	}

	return summary
}

// calculateScore calculates the overall score (0-100)
func (a *Analyzer) calculateScore(result *AnalysisResult) Score {
	score := 100

	for _, issue := range result.Issues {
		switch issue.Severity {
		case SeverityCritical:
			score -= 15
		case SeverityError:
			score -= 10
		case SeverityWarning:
			score -= 5
		case SeverityInfo:
			score -= 1
		}
	}

	if score < 0 {
		score = 0
	}

	grade := "F"
	switch {
	case score >= 90:
		grade = "A"
	case score >= 80:
		grade = "B"
	case score >= 70:
		grade = "C"
	case score >= 60:
		grade = "D"
	}

	description := "Excellent"
	switch grade {
	case "B":
		description = "Good - minor improvements possible"
	case "C":
		description = "Fair - several improvements recommended"
	case "D":
		description = "Poor - significant improvements needed"
	case "F":
		description = "Critical - immediate attention required"
	}

	return Score{
		Value:       score,
		Grade:       grade,
		Description: description,
	}
}

// Helper functions

func hasConcurrency(wf *workflow.Workflow) bool {
	// Check if any job has concurrency settings
	// This would need to be parsed from raw YAML
	return false
}

func hasWorkflowPermissions(wf *workflow.Workflow) bool {
	// Check if workflow has top-level permissions
	// This would need to be parsed from raw YAML
	return false
}

func hasPullRequestTarget(wf *workflow.Workflow) bool {
	// Check if the workflow has pull_request_target trigger
	// This would need to be parsed from raw YAML's "on" key
	// For now, check if there's no pull_request target but there's
	// a pull_request_target in Other triggers
	for key := range wf.On.Other {
		if key == "pull_request_target" {
			return true
		}
	}
	return false
}

func findSimilarJobs(wf *workflow.Workflow) [][]string {
	var pairs [][]string
	jobNames := make([]string, 0, len(wf.Jobs))
	jobStepCounts := make(map[string]int)

	for name, job := range wf.Jobs {
		jobNames = append(jobNames, name)
		jobStepCounts[name] = len(job.Steps)
	}

	for i := 0; i < len(jobNames); i++ {
		for j := i + 1; j < len(jobNames); j++ {
			// Simple similarity check based on step count
			if jobStepCounts[jobNames[i]] == jobStepCounts[jobNames[j]] &&
				jobStepCounts[jobNames[i]] > 3 {
				pairs = append(pairs, []string{jobNames[i], jobNames[j]})
			}
		}
	}

	return pairs
}
