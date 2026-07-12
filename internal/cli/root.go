package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/EdgarOrtegaRamirez/actionforge/internal/analyzer"
	"github.com/EdgarOrtegaRamirez/actionforge/internal/workflow"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	format      string
	dirPath     string
	strictMode  bool
	showVersion bool
)

const version = "1.0.0"

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "actionforge",
	Short: "GitHub Actions Workflow Analyzer & Linter",
	Long: `ActionForge analyzes GitHub Actions workflow files for security issues,
best practices, optimization opportunities, performance problems, and
maintenance concerns. It produces a 0-100 score with letter grade.`,
	Version: version,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Validate format flag
		validFormats := map[string]bool{"text": true, "json": true, "markdown": true}
		if !validFormats[format] {
			fmt.Fprintf(os.Stderr, "Error: invalid format '%s'. Must be one of: text, json, markdown\n", format)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&format, "format", "f", "text", "Output format (text, json, markdown)")

	// Root command defaults to analyzing files or directories
	rootCmd.RunE = runAnalyze

	// analyze subcommand
	analyzeCmd := &cobra.Command{
		Use:   "analyze [files...]",
		Short: "Analyze one or more workflow files",
		Long:  `Analyze GitHub Actions workflow files and produce a detailed report.`,
		Args:  cobra.ArbitraryArgs,
		RunE:  runAnalyze,
	}
	analyzeCmd.Flags().StringVarP(&dirPath, "dir", "d", "", "Directory containing .github/workflows/")
	rootCmd.AddCommand(analyzeCmd)

	// lint subcommand (CI-friendly, exits non-zero on errors/criticals)
	lintCmd := &cobra.Command{
		Use:   "lint [files...]",
		Short: "CI-friendly lint mode, exits non-zero on errors or criticals",
		Long:  `Lint workflow files and exit with code 1 if any ERROR or CRITICAL issues are found.`,
		Args:  cobra.ArbitraryArgs,
		RunE:  runLint,
	}
	lintCmd.Flags().StringVarP(&dirPath, "dir", "d", "", "Directory containing .github/workflows/")
	rootCmd.AddCommand(lintCmd)

	// score subcommand (just output the score and grade)
	scoreCmd := &cobra.Command{
		Use:   "score [files...]",
		Short: "Output just the score and grade",
		Long:  `Quick scoring of workflow files, outputting only the score and letter grade.`,
		Args:  cobra.ArbitraryArgs,
		RunE:  runScore,
	}
	scoreCmd.Flags().StringVarP(&dirPath, "dir", "d", "", "Directory containing .github/workflows/")
	rootCmd.AddCommand(scoreCmd)

	// version command
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("actionforge v%s\n", version)
		},
	}
	rootCmd.AddCommand(versionCmd)
}

func getWorkflows(args []string) ([]*workflow.Workflow, error) {
	parser := workflow.NewParser()

	// If --dir is specified, parse that directory
	if dirPath != "" {
		return parser.ParseDirectory(dirPath)
	}

	// If arguments are provided, parse each file
	if len(args) > 0 {
		var workflows []*workflow.Workflow
		for _, file := range args {
			wf, err := parser.ParseFile(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", file, err)
				continue
			}
			workflows = append(workflows, wf)
		}
		if len(workflows) == 0 {
			return nil, fmt.Errorf("no valid workflow files found")
		}
		return workflows, nil
	}

	// Default: look for .github/workflows in current directory
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	return parser.ParseDirectory(cwd)
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	workflows, err := getWorkflows(args)
	if err != nil {
		return err
	}

	an := analyzer.NewAnalyzer()
	for _, wf := range workflows {
		result := an.Analyze(wf)
		outputResult(result)
	}
	return nil
}

func runLint(cmd *cobra.Command, args []string) error {
	workflows, err := getWorkflows(args)
	if err != nil {
		return err
	}

	an := analyzer.NewAnalyzer()
	hasErrors := false

	for _, wf := range workflows {
		result := an.Analyze(wf)
		hasCriticalOrError := false
		for _, issue := range result.Issues {
			if issue.Severity == analyzer.SeverityCritical || issue.Severity == analyzer.SeverityError {
				hasCriticalOrError = true
				break
			}
		}

		if format == "json" {
			outputJSON(result)
		} else {
			fmt.Printf("📋 %s\n", color.CyanString(result.Workflow.Name))
			fmt.Printf("   Score: %d/100 (%s)\n", result.Score.Value, colorGrade(result.Score.Grade))
			if hasCriticalOrError {
				color.Red("   ❌ FAILED: ERRORS or CRITICALS found")
				hasErrors = true
			} else {
				color.Green("   ✅ PASSED")
			}
			fmt.Println()
		}
	}

	if hasErrors {
		os.Exit(1)
	}
	return nil
}

func runScore(cmd *cobra.Command, args []string) error {
	workflows, err := getWorkflows(args)
	if err != nil {
		return err
	}

	an := analyzer.NewAnalyzer()
	for _, wf := range workflows {
		result := an.Analyze(wf)
		if format == "json" {
			outputJSON(result)
		} else {
			fmt.Printf("%s: %d/100 (%s)\n", wf.Name, result.Score.Value, result.Score.Grade)
		}
	}
	return nil
}

func outputResult(result *analyzer.AnalysisResult) {
	switch format {
	case "json":
		outputJSON(result)
	case "markdown":
		outputMarkdown(result)
	default:
		outputText(result)
	}
}

func outputText(result *analyzer.AnalysisResult) {
	wf := result.Workflow

	color.Cyan("\n📋 %s", wf.Name)
	if wf.FilePath != "" {
		fmt.Printf("   File: %s\n", wf.FilePath)
	}
	fmt.Printf("   Triggers: %s\n", strings.Join(wf.GetTriggers(), ", "))
	fmt.Printf("   Jobs: %d | Steps: %d | Actions: %d | Runners: %d\n",
		result.Summary.JobCount, result.Summary.StepCount,
		result.Summary.ActionCount, result.Summary.RunnerCount)

	// Summary badges
	var badges []string
	if result.Summary.HasCaching {
		badges = append(badges, "📦 caching")
	}
	if result.Summary.HasMatrix {
		badges = append(badges, "🔀 matrix")
	}
	if result.Summary.HasTimeout {
		badges = append(badges, "⏱️  timeout")
	}
	if result.Summary.HasPermissions {
		badges = append(badges, "🔒 permissions")
	}
	if len(badges) > 0 {
		fmt.Printf("   Features: %s\n", strings.Join(badges, ", "))
	}

	// Score
	fmt.Printf("\n   %s\n", colorGrade(result.Score.Grade))
	fmt.Printf("   Score: %d/100 — %s\n", result.Score.Value, result.Score.Description)

	// Issues by severity
	if len(result.Issues) > 0 {
		fmt.Printf("\n   Issues (%d total):\n", len(result.Issues))
		for _, issue := range result.Issues {
			severityColor := severityColor(issue.Severity)
			severityStr := fmt.Sprintf("[%s]", severityColor(issue.Severity.String()))
			fmt.Printf("   %s %s\n", severityStr, issue.Title)
			fmt.Printf("       %s\n", issue.Description)
			if issue.Location != "" {
				fmt.Printf("       Location: %s\n", issue.Location)
			}
			if issue.Suggestion != "" {
				fmt.Printf("       💡 %s\n", issue.Suggestion)
			}
			fmt.Println()
		}
	}
}

func outputJSON(result *analyzer.AnalysisResult) {
	data := struct {
		Name   string           `json:"name"`
		File   string           `json:"file,omitempty"`
		Score  analyzer.Score   `json:"score"`
		Issues []analyzer.Issue `json:"issues"`
		Stats  analyzer.Summary `json:"stats"`
		Jobs   int              `json:"job_count"`
		Steps  int              `json:"step_count"`
	}{
		Name:   result.Workflow.Name,
		File:   result.Workflow.FilePath,
		Score:  result.Score,
		Issues: result.Issues,
		Stats:  result.Summary,
		Jobs:   result.Summary.JobCount,
		Steps:  result.Summary.StepCount,
	}

	fmt.Println(toJSON(data))
}

func outputMarkdown(result *analyzer.AnalysisResult) {
	wf := result.Workflow

	fmt.Printf("## Workflow: %s\n\n", wf.Name)
	if wf.FilePath != "" {
		fmt.Printf("**File:** `%s`\n\n", wf.FilePath)
	}
	fmt.Printf("**Score:** %d/100 (**%s**) — %s\n\n", result.Score.Value, result.Score.Grade, result.Score.Description)

	// Summary table
	fmt.Println("| Metric | Value |")
	fmt.Println("|--------|-------|")
	fmt.Printf("| Jobs | %d |\n", result.Summary.JobCount)
	fmt.Printf("| Steps | %d |\n", result.Summary.StepCount)
	fmt.Printf("| Actions | %d |\n", result.Summary.ActionCount)
	fmt.Printf("| Runners | %d |\n", result.Summary.RunnerCount)
	fmt.Printf("| Triggers | %d |\n", result.Summary.TriggerCount)
	fmt.Printf("| Caching | %v |\n", result.Summary.HasCaching)
	fmt.Printf("| Matrix | %v |\n", result.Summary.HasMatrix)
	fmt.Printf("| Timeouts | %v |\n", result.Summary.HasTimeout)
	fmt.Printf("| Permissions | %v |\n", result.Summary.HasPermissions)
	fmt.Println()

	// Issues
	if len(result.Issues) > 0 {
		fmt.Println("### Issues")
		fmt.Println("| Severity | Category | Title | Location | Suggestion |")
		fmt.Println("|----------|----------|-------|----------|------------|")
		for _, issue := range result.Issues {
			fmt.Printf("| %s | %s | %s | %s | %s |\n",
				issue.Severity.String(),
				issue.Category,
				issue.Title,
				issue.Location,
				issue.Suggestion,
			)
		}
		fmt.Println()
	}
}

// Helper functions

func colorGrade(grade string) string {
	switch grade {
	case "A":
		return color.GreenString("🅰️  Excellent")
	case "B":
		return color.GreenString("🅱️  Good")
	case "C":
		return color.YellowString("🅲  Fair")
	case "D":
		return color.RedString("🅳  Poor")
	default:
		return color.RedString("🅵  Critical")
	}
}

func severityColor(s analyzer.Severity) func(format string, a ...interface{}) string {
	switch s {
	case analyzer.SeverityCritical:
		return color.RedString
	case analyzer.SeverityError:
		return color.RedString
	case analyzer.SeverityWarning:
		return color.YellowString
	default:
		return color.CyanString
	}
}

func toJSON(v interface{}) string {
	data, err := marshalJSON(v)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(data)
}

// marshalJSON is a helper to serialize JSON
func marshalJSON(v interface{}) ([]byte, error) {
	// Use encoding/json via a simple wrapper
	return jsonMarshal(v)
}
