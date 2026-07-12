# ActionForge

**GitHub Actions Workflow Analyzer & Linter**

ActionForge analyzes GitHub Actions workflow files for security issues, best practices, optimization opportunities, performance problems, and maintenance concerns. It produces a comprehensive report with a 0-100 score and letter grade.

## Features

- 🔒 **Security Checks** — Unpinned actions, overly permissive tokens, secret exposure, injection risks (issue body, PR title, comment body), curl-to-shell patterns, pull_request_target vulnerabilities
- ✅ **Best Practices** — Missing timeouts, concurrency control, workflow-level permissions, error handling in run steps, hardcoded runner versions
- ⚡ **Optimization** — Missing caching, matrix strategy opportunities, redundant checkouts
- 🏎️ **Performance** — Self-hosted runners, complex dependency chains
- 🔧 **Maintenance** — Deprecated actions, outdated action versions
- 📊 **Scoring** — 0-100 score with letter grade (A-F)
- 📋 **Multiple Output Formats** — Text (terminal), JSON, Markdown
- 🔄 **CI/CD Mode** — `lint` subcommand exits non-zero on errors/criticals
- 📁 **Directory Scanning** — Analyze all workflows in `.github/workflows/`

## Installation

### From source

```bash
go install github.com/EdgarOrtegaRamirez/actionforge/cmd/actionforge@latest
```

### From release

```bash
# Download the latest binary for your platform from GitHub Releases
```

## Quick Start

```bash
# Analyze a single workflow file
actionforge analyze .github/workflows/ci.yml

# Analyze all workflows in a directory
actionforge analyze --dir /path/to/repo

# Quick score only
actionforge score .github/workflows/deploy.yml

# CI-friendly lint mode (exits non-zero on errors/criticals)
actionforge lint .github/workflows/ci.yml

# JSON output
actionforge analyze -f json .github/workflows/ci.yml

# Markdown output
actionforge analyze -f markdown .github/workflows/ci.yml
```

## Usage

### Subcommands

| Command | Description |
|---------|-------------|
| `analyze` | Analyze workflow files with full report (default) |
| `lint` | CI-friendly mode, exits non-zero on ERROR/CRITICAL |
| `score` | Quick output of score and grade only |
| `version` | Print version information |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--format` | `-f` | Output format: `text`, `json`, `markdown` (default: `text`) |
| `--dir` | `-d` | Directory containing `.github/workflows/` |

### Examples

**Analyze with colored terminal output:**
```bash
actionforge analyze .github/workflows/ci.yml
```

**JSON output for CI integration:**
```bash
actionforge analyze -f json .github/workflows/*.yml
```

**CI pipeline check:**
```bash
actionforge lint --dir . && echo "All checks passed" || echo "Issues found"
```

**Markdown report:**
```bash
actionforge analyze -f markdown .github/workflows/deploy.yml > report.md
```

## Scoring

ActionForge computes a 0-100 score based on detected issues:

| Grade | Score | Meaning |
|-------|-------|---------|
| 🅰️  A | 90-100 | Excellent |
| 🅱️  B | 80-89 | Good — minor improvements |
| 🅲  C | 70-79 | Fair — several improvements |
| 🅳  D | 60-69 | Poor — significant improvements |
| 🅵  F | 0-59 | Critical — immediate attention |

Deductions:
- **Critical** issues: -15 points
- **Error** issues: -10 points
- **Warning** issues: -5 points
- **Info** issues: -1 point

## Architecture

```
actionforge/
├── cmd/actionforge/main.go      # CLI entrypoint
├── internal/
│   ├── cli/                     # CLI implementation (cobra commands)
│   │   ├── root.go              # Root command and subcommands
│   │   └── json.go              # JSON helpers
│   ├── workflow/                # Workflow model and parsing
│   │   ├── models.go            # Data models
│   │   └── parser.go            # YAML workflow parser
│   └── analyzer/                # Analysis engine
│       └── analyzer.go          # Security, best practices, optimization checks
├── tests/                       # Test fixtures and integration tests
├── .github/workflows/ci.yml     # CI configuration
└── README.md
```

## Example Output

```
📋 CI Pipeline
   File: /repo/.github/workflows/ci.yml
   Triggers: push, pull_request
   Jobs: 3 | Steps: 15 | Actions: 8 | Runners: 2
   Features: 📦 caching, 🔀 matrix, ⏱️  timeout

   🅱️  Good
   Score: 82/100 — Good - minor improvements possible

   Issues (5 total):
   [WARNING] Action not pinned to SHA
       Action 'actions/checkout@v4' is not pinned to a commit SHA.
       💡 Pin to a specific SHA: actions/checkout@<commit-sha>
   [INFO] No caching detected
       Workflow doesn't use caching. Adding cache can significantly speed up builds.
       💡 Add actions/cache or built-in cache in setup-node/setup-python
```

## License

MIT License — see [LICENSE](LICENSE)

## Security

See [SECURITY.md](SECURITY.md) for security policy.