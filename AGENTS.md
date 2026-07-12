# ActionForge

## AI Agent Guide

This file provides context for AI coding agents working on this project.

### Project Overview

ActionForge is a Go CLI tool that analyzes GitHub Actions workflow files. It parses YAML workflow files and checks for security issues, best practices, optimization opportunities, performance problems, and maintenance concerns.

### Key Design Decisions

1. **Go with Cobra CLI** — Standard Go CLI pattern with cobra for command parsing
2. **YAML-based parsing** — Uses `gopkg.in/yaml.v3` for parsing workflow files
3. **Modular analysis** — Each analysis category (security, best-practice, etc.) is a separate method on the Analyzer
4. **Severity-based scoring** — Issues have severity levels (CRITICAL/ERROR/WARNING/INFO) that determine score deduction
5. **fatih/color** — Colored terminal output for readability

### Package Structure

- `cmd/actionforge/main.go` — CLI entrypoint
- `internal/cli/` — Cobra command definitions and output formatting
- `internal/workflow/` — Data models (models.go) and YAML parser (parser.go)
- `internal/analyzer/` — Analysis engine with all check categories

### Adding New Checks

To add a new analysis check:

1. Add a new method to `Analyzer` in `internal/analyzer/analyzer.go`
2. Call it from the `Analyze()` method
3. Add test fixtures in `tests/fixtures/`
4. Add tests in `tests/` directory

### Building

```bash
go build ./cmd/actionforge/
```

### Testing

```bash
go test ./... -v
```

### Data Flow

1. CLI receives file paths or directory
2. Parser reads YAML files into `Workflow` struct
3. Analyzer checks each workflow against rules
4. CLI formats and outputs the `AnalysisResult`

### Issue Severity Deductions

- CRITICAL: -15 points
- ERROR: -10 points
- WARNING: -5 points
- INFO: -1 point