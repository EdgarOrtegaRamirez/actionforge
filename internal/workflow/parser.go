package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parser handles parsing GitHub Actions workflow files
type Parser struct{}

// NewParser creates a new workflow parser
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile parses a single workflow file
func (p *Parser) ParseFile(path string) (*Workflow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}

	return p.ParseBytes(data, path)
}

// ParseBytes parses workflow content from bytes
func (p *Parser) ParseBytes(data []byte, filePath string) (*Workflow, error) {
	var workflow Workflow

	// Parse YAML
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Set file path
	workflow.FilePath = filePath
	workflow.RawYAML = string(data)

	// Parse the "on" trigger which has complex YAML structure
	if err := p.parseTrigger(data, &workflow); err != nil {
		// Non-fatal: some workflows may have unusual trigger formats
		fmt.Fprintf(os.Stderr, "Warning: failed to parse triggers: %v\n", err)
	}

	// Normalize job data
	for jobName, job := range workflow.Jobs {
		job.Output = make(map[string]string)
		workflow.Jobs[jobName] = job
	}

	return &workflow, nil
}

// parseTrigger handles the complex "on" trigger parsing
func (p *Parser) parseTrigger(data []byte, workflow *Workflow) error {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return err
	}

	onRaw, ok := raw["on"]
	if !ok {
		return nil
	}

	onMap, ok := onRaw.(map[string]interface{})
	if !ok {
		return nil
	}

	// Parse push trigger
	if pushRaw, ok := onMap["push"]; ok {
		if pushMap, ok := pushRaw.(map[string]interface{}); ok {
			workflow.On.Push = parseEventTrigger(pushMap)
		}
	}

	// Parse pull_request trigger
	if prRaw, ok := onMap["pull_request"]; ok {
		if prMap, ok := prRaw.(map[string]interface{}); ok {
			workflow.On.PullRequest = parseEventTrigger(prMap)
		}
	}

	// Parse release trigger
	if relRaw, ok := onMap["release"]; ok {
		if relMap, ok := relRaw.(map[string]interface{}); ok {
			workflow.On.Release = parseEventTrigger(relMap)
		}
	}

	// Parse schedule trigger
	if schedRaw, ok := onMap["schedule"]; ok {
		if schedList, ok := schedRaw.([]interface{}); ok {
			for _, s := range schedList {
				if schedMap, ok := s.(map[string]interface{}); ok {
					if cron, ok := schedMap["cron"].(string); ok {
						workflow.On.Schedule = append(workflow.On.Schedule, ScheduleTrigger{Cron: cron})
					}
				}
			}
		}
	}

	// Parse workflow_dispatch
	if wdRaw, ok := onMap["workflow_dispatch"]; ok {
		if wdMap, ok := wdRaw.(map[string]interface{}); ok {
			workflow.On.WorkflowDispatch = &wdMap
		} else {
			// workflow_dispatch can be empty (boolean true)
			empty := make(map[string]interface{})
			workflow.On.WorkflowDispatch = &empty
		}
	}

	// Store other triggers
	knownTriggers := map[string]bool{
		"push": true, "pull_request": true, "release": true,
		"schedule": true, "workflow_dispatch": true,
	}
	workflow.On.Other = make(map[string]interface{})
	for k, v := range onMap {
		if !knownTriggers[k] {
			workflow.On.Other[k] = v
		}
	}

	return nil
}

// parseEventTrigger parses an event trigger (push/pull_request)
func parseEventTrigger(m map[string]interface{}) *EventTrigger {
	trigger := &EventTrigger{}

	if branches, ok := m["branches"]; ok {
		trigger.Branches = toStringSlice(branches)
	}
	if tags, ok := m["tags"]; ok {
		trigger.Tags = toStringSlice(tags)
	}
	if paths, ok := m["paths"]; ok {
		trigger.Paths = toStringSlice(paths)
	}
	if types, ok := m["types"]; ok {
		trigger.Types = toStringSlice(types)
	}

	return trigger
}

// toStringSlice converts an interface{} to a string slice
func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case []interface{}:
		var result []string
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case []string:
		return val
	default:
		return nil
	}
}

// ParseDirectory parses all workflow files in a directory
func (p *Parser) ParseDirectory(dir string) ([]*Workflow, error) {
	workflowDir := filepath.Join(dir, ".github", "workflows")
	if _, err := os.Stat(workflowDir); os.IsNotExist(err) {
		// Try standard path
		workflowDir = filepath.Join(dir, ".github", "workflows")
	}

	return p.ParseWorkflowDir(workflowDir)
}

// ParseWorkflowDir parses all workflow files in a specific directory
func (p *Parser) ParseWorkflowDir(dir string) ([]*Workflow, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var workflows []*Workflow
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			path := filepath.Join(dir, name)
			wf, err := p.ParseFile(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", path, err)
				continue
			}
			workflows = append(workflows, wf)
		}
	}

	return workflows, nil
}
