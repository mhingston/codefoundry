package ci

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"
)

// GitHubConfig represents configuration for GitHub Actions
type GitHubConfig struct {
	WorkflowName string
	OnEvents     []string // push, pull_request, workflow_dispatch
	Branches     []string // main, master
	Stages       []string // plan, spec, implement, verify, review, lock
	GoVersion    string
	WorkingDir   string
}

// CIStatus represents a GitHub status check
type CIStatus struct {
	State       string `json:"state"` // success, failure, pending, error
	Description string `json:"description"`
	TargetURL   string `json:"target_url"`
	Context     string `json:"context"`
}

// DefaultConfig returns the default GitHub Actions configuration
func DefaultConfig() GitHubConfig {
	return GitHubConfig{
		WorkflowName: "CodeFoundry",
		OnEvents:     []string{"push", "pull_request"},
		Branches:     []string{"main", "master"},
		Stages:       []string{"plan", "spec", "implement", "verify", "review", "lock"},
		GoVersion:    "1.21",
		WorkingDir:   ".",
	}
}

// GenerateWorkflow generates a GitHub Actions workflow YAML
func GenerateWorkflow(config GitHubConfig) (string, error) {
	if config.WorkflowName == "" {
		config.WorkflowName = "CodeFoundry"
	}
	if len(config.OnEvents) == 0 {
		config.OnEvents = []string{"push", "pull_request"}
	}
	if len(config.Branches) == 0 {
		config.Branches = []string{"main"}
	}
	if len(config.Stages) == 0 {
		config.Stages = []string{"plan", "spec", "implement", "verify", "review", "lock"}
	}
	if config.GoVersion == "" {
		config.GoVersion = "1.21"
	}

	funcMap := template.FuncMap{
		"formatBranches": formatBranches,
	}
	tmpl, err := template.New("workflow").Funcs(funcMap).Parse(workflowTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse workflow template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		return "", fmt.Errorf("failed to execute workflow template: %w", err)
	}

	return buf.String(), nil
}

// GenerateStatusPayload generates a GitHub status check payload
func GenerateStatusPayload(status CIStatus) ([]byte, error) {
	// Validate state
	validStates := map[string]bool{
		"success": true,
		"failure": true,
		"pending": true,
		"error":   true,
	}

	if !validStates[status.State] {
		return nil, fmt.Errorf("invalid state: %s", status.State)
	}

	// Set default context
	if status.Context == "" {
		status.Context = "continuous-integration/codefoundry"
	}

	return json.MarshalIndent(status, "", "  ")
}

// GeneratePRComment generates a comment for a PR based on results
func GeneratePRComment(runID string, success bool, findings map[string]int) string {
	var sb strings.Builder

	sb.WriteString("## CodeFoundry Results\n\n")

	if success {
		sb.WriteString("✅ **All checks passed**\n\n")
	} else {
		sb.WriteString("❌ **Some checks failed**\n\n")
	}

	sb.WriteString(fmt.Sprintf("**Run ID:** %s\n\n", runID))

	if len(findings) > 0 {
		sb.WriteString("### Findings\n\n")
		for severity, count := range findings {
			sb.WriteString(fmt.Sprintf("- **%s:** %d\n", severity, count))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("<details>\n")
	sb.WriteString("<summary>View Details</summary>\n\n")
	sb.WriteString("Run `codefoundry report` locally for full details.\n")
	sb.WriteString("</details>\n")

	return sb.String()
}

// GenerateAnnotations generates GitHub Actions annotations from findings
func GenerateAnnotations(findings []Finding) []string {
	annotations := make([]string, 0)

	for _, finding := range findings {
		level := "notice"
		if finding.Severity == "error" || finding.Severity == "P1" {
			level = "error"
		} else if finding.Severity == "warning" || finding.Severity == "P2" {
			level = "warning"
		}

		annotation := fmt.Sprintf("::%s file=%s,line=%d::%s",
			level, finding.File, finding.Line, finding.Message)

		annotations = append(annotations, annotation)
	}

	return annotations
}

// Finding represents a CI finding/annotation
type Finding struct {
	File     string
	Line     int
	Message  string
	Severity string
}

// ParseWorkflow parses a workflow file to extract config
func ParseWorkflow(content string) (*GitHubConfig, error) {
	// Simple parsing - in production, use a YAML parser
	config := &GitHubConfig{}

	// Extract workflow name
	if strings.Contains(content, "name:") {
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "name:") {
				config.WorkflowName = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
				config.WorkflowName = strings.Trim(config.WorkflowName, `"'`)
				break
			}
		}
	}

	// This is a simplified parser - full implementation would use YAML
	return config, nil
}

// ValidateWorkflow validates a workflow configuration
func ValidateWorkflow(config GitHubConfig) error {
	if config.WorkflowName == "" {
		return fmt.Errorf("workflow name is required")
	}

	if len(config.OnEvents) == 0 {
		return fmt.Errorf("at least one trigger event is required")
	}

	validEvents := map[string]bool{
		"push":              true,
		"pull_request":      true,
		"workflow_dispatch": true,
		"schedule":          true,
		"release":           true,
	}

	for _, event := range config.OnEvents {
		if !validEvents[event] {
			return fmt.Errorf("invalid trigger event: %s", event)
		}
	}

	if len(config.Branches) == 0 {
		return fmt.Errorf("at least one branch is required")
	}

	return nil
}

// WorkflowTemplate is the GitHub Actions workflow template
var workflowTemplate = `name: {{ .WorkflowName }}

on:
{{- range .OnEvents }}
  {{ . }}:
    branches: {{ formatBranches $.Branches }}
{{- end }}

env:
  GO_VERSION: '{{ .GoVersion }}'

jobs:
  codefoundry:
    runs-on: ubuntu-latest
    defaults:
      run:
        working-directory: {{ .WorkingDir }}

    steps:
    - name: Checkout
      uses: actions/checkout@v4

    - name: Setup Go
      uses: actions/setup-go@v5
      with:
        go-version: {{ .GoVersion }}

    - name: Build CodeFoundry
      run: go build -o codefoundry ./cmd/codefoundry

    - name: Initialize CodeFoundry
      run: ./codefoundry init

{{- range .Stages }}
    - name: Stage - {{ . }}
      run: ./codefoundry run {{ . }}
      continue-on-error: true
{{- end }}

    - name: Generate Report
      run: ./codefoundry report -f ci -o report.txt

    - name: Upload Evidence
      uses: actions/upload-artifact@v4
      with:
        name: codefoundry-evidence
        path: |
          .codefoundry/artifacts/
          report.txt

    - name: Check Results
      run: |
        if grep -q "fail" report.txt; then
          echo "CodeFoundry checks failed"
          exit 1
        fi
`

// formatBranches formats branches for YAML list format
func formatBranches(branches []string) string {
	if len(branches) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, branch := range branches {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(fmt.Sprintf("      - %s", branch))
	}
	return sb.String()
}

// GenerateWorkflowFile generates a complete workflow file with proper formatting
func GenerateWorkflowFile(config GitHubConfig) (string, error) {
	// Build workflow manually for better control
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("name: %s\n\n", config.WorkflowName))

	// On section
	sb.WriteString("on:\n")
	for _, event := range config.OnEvents {
		sb.WriteString(fmt.Sprintf("  %s:\n", event))
		if len(config.Branches) > 0 {
			sb.WriteString("    branches:")
			for _, branch := range config.Branches {
				sb.WriteString(fmt.Sprintf("\n      - %s", branch))
			}
			sb.WriteString("\n")
		}
	}
	sb.WriteString("\n")

	// Environment
	sb.WriteString(fmt.Sprintf("env:\n  GO_VERSION: '%s'\n\n", config.GoVersion))

	// Jobs
	sb.WriteString("jobs:\n")
	sb.WriteString("  codefoundry:\n")
	sb.WriteString("    runs-on: ubuntu-latest\n")
	sb.WriteString(fmt.Sprintf("    defaults:\n      run:\n        working-directory: %s\n\n", config.WorkingDir))

	// Steps
	sb.WriteString("    steps:\n")
	sb.WriteString("    - name: Checkout\n")
	sb.WriteString("      uses: actions/checkout@v4\n\n")

	sb.WriteString("    - name: Setup Go\n")
	sb.WriteString("      uses: actions/setup-go@v5\n")
	sb.WriteString(fmt.Sprintf("      with:\n        go-version: '%s'\n\n", config.GoVersion))

	sb.WriteString("    - name: Build CodeFoundry\n")
	sb.WriteString("      run: go build -o codefoundry ./cmd/codefoundry\n\n")

	sb.WriteString("    - name: Initialize CodeFoundry\n")
	sb.WriteString("      run: ./codefoundry init\n")

	// Stages
	for _, stage := range config.Stages {
		sb.WriteString(fmt.Sprintf("\n    - name: Stage - %s\n", stage))
		sb.WriteString(fmt.Sprintf("      run: ./codefoundry run %s\n", stage))
		sb.WriteString("      continue-on-error: true\n")
	}

	// Report
	sb.WriteString("\n    - name: Generate Report\n")
	sb.WriteString("      run: ./codefoundry report -f ci -o report.txt\n\n")

	// Upload
	sb.WriteString("    - name: Upload Evidence\n")
	sb.WriteString("      uses: actions/upload-artifact@v4\n")
	sb.WriteString("      with:\n")
	sb.WriteString("        name: codefoundry-evidence\n")
	sb.WriteString("        path: |\n")
	sb.WriteString("          .codefoundry/artifacts/\n")
	sb.WriteString("          report.txt\n\n")

	// Check results
	sb.WriteString("    - name: Check Results\n")
	sb.WriteString("      run: |\n")
	sb.WriteString("        if grep -q \"fail\" report.txt; then\n")
	sb.WriteString("          echo \"CodeFoundry checks failed\"\n")
	sb.WriteString("          exit 1\n")
	sb.WriteString("        fi\n")

	return sb.String(), nil
}
