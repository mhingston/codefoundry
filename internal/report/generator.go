package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/gate"
	"github.com/mhingston/codefoundry/internal/lock"
	"github.com/mhingston/codefoundry/internal/review"
)

// ReportFormat represents the output format
type ReportFormat string

const (
	FormatJSON     ReportFormat = "json"
	FormatMarkdown ReportFormat = "markdown"
	FormatHTML     ReportFormat = "html"
	FormatCI       ReportFormat = "ci"
)

// StageStatus represents a stage's status
type StageStatus struct {
	StageID     string    `json:"stage_id"`
	Status      string    `json:"status"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Duration    string    `json:"duration,omitempty"`
}

// Report contains all evidence for a run
type Report struct {
	RunID        string                 `json:"run_id"`
	Status       string                 `json:"status"`
	StartedAt    time.Time              `json:"started_at"`
	CompletedAt  time.Time              `json:"completed_at,omitempty"`
	Stages       []StageStatus          `json:"stages"`
	GateReports  []gate.GateResult      `json:"gate_reports"`
	ReviewResult *review.ReviewResult   `json:"review_result,omitempty"`
	LockDecision *lock.LockDecision     `json:"lock_decision,omitempty"`
	Artifacts    []string               `json:"artifacts"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// Generator generates reports
type Generator struct {
	artifactStore *artifact.Store
	basePath      string
}

// NewGenerator creates a new report generator
func NewGenerator(artifactStore *artifact.Store, basePath string) *Generator {
	return &Generator{
		artifactStore: artifactStore,
		basePath:      basePath,
	}
}

// GenerateReport generates a report for a run
func (g *Generator) GenerateReport(runID string, format ReportFormat) (*Report, error) {
	if g.artifactStore == nil {
		return nil, fmt.Errorf("artifact store not configured")
	}

	report := &Report{
		RunID:     runID,
		Metadata:  make(map[string]interface{}),
		StartedAt: time.Now().UTC(),
	}

	// Load all stages
	stages, err := g.discoverStages(runID)
	if err != nil {
		return nil, fmt.Errorf("failed to discover stages: %w", err)
	}

	// Load data for each stage
	for _, stageID := range stages {
		stageReport := StageStatus{StageID: stageID}

		// Load gate reports
		gateResults, err := g.loadGateReports(stageID)
		if err == nil {
			report.GateReports = append(report.GateReports, gateResults...)
		}

		// Load review result
		reviewResult, err := g.loadReviewResult(stageID)
		if err == nil && reviewResult != nil {
			report.ReviewResult = reviewResult
		}

		// Load lock decision
		lockDecision, err := g.loadLockDecision(stageID)
		if err == nil && lockDecision != nil {
			report.LockDecision = lockDecision
		}

		// Load stage status
		status, err := g.loadStageStatus(stageID)
		if err == nil {
			stageReport.Status = status.Status
			if status.StartedAt != "" {
				if t, err := time.Parse(time.RFC3339, status.StartedAt); err == nil {
					stageReport.StartedAt = t
				}
			}
			if status.CompletedAt != "" {
				if t, err := time.Parse(time.RFC3339, status.CompletedAt); err == nil {
					stageReport.CompletedAt = t
				}
			}
			if !stageReport.StartedAt.IsZero() && !stageReport.CompletedAt.IsZero() {
				stageReport.Duration = stageReport.CompletedAt.Sub(stageReport.StartedAt).String()
			}
		}

		report.Stages = append(report.Stages, stageReport)
	}

	// Determine overall status
	report.Status = g.calculateOverallStatus(report)
	report.CompletedAt = time.Now().UTC()

	return report, nil
}

// GenerateJSON generates a JSON report
func (g *Generator) GenerateJSON(runID string) ([]byte, error) {
	report, err := g.GenerateReport(runID, FormatJSON)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(report, "", "  ")
}

// GenerateMarkdown generates a Markdown report
func (g *Generator) GenerateMarkdown(runID string) (string, error) {
	report, err := g.GenerateReport(runID, FormatMarkdown)
	if err != nil {
		return "", err
	}

	return g.renderMarkdown(report), nil
}

// GenerateCI generates CI-friendly output
func (g *Generator) GenerateCI(runID string) (string, error) {
	report, err := g.GenerateReport(runID, FormatCI)
	if err != nil {
		return "", err
	}

	return g.renderCI(report), nil
}

// SaveReport saves a report to a file
func (g *Generator) SaveReport(runID string, format ReportFormat, outputPath string) error {
	var content []byte
	var err error

	switch format {
	case FormatJSON:
		content, err = g.GenerateJSON(runID)
	case FormatMarkdown:
		var md string
		md, err = g.GenerateMarkdown(runID)
		content = []byte(md)
	case FormatCI:
		var ci string
		ci, err = g.GenerateCI(runID)
		content = []byte(ci)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return err
	}

	if err := os.WriteFile(outputPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	return nil
}

// discoverStages discovers all stages for a run
func (g *Generator) discoverStages(runID string) ([]string, error) {
	// Read from run directory
	runPath := filepath.Join(g.basePath, "artifacts", runID)
	
	entries, err := os.ReadDir(runPath)
	if err != nil {
		return nil, err
	}

	var stages []string
	for _, entry := range entries {
		if entry.IsDir() {
			stages = append(stages, entry.Name())
		}
	}

	return stages, nil
}

// loadGateReports loads gate reports from a stage
func (g *Generator) loadGateReports(stageID string) ([]gate.GateResult, error) {
	// List artifacts
	artifacts, err := g.artifactStore.List(stageID)
	if err != nil {
		return nil, err
	}

	var results []gate.GateResult
	for _, artifact := range artifacts {
		// Skip non-gate artifacts
		if artifact == "status.json" || 
		   artifact == "review-result.json" || 
		   artifact == "lock-decision.json" ||
		   filepath.Ext(artifact) != ".json" {
			continue
		}

		data, err := g.artifactStore.Read(stageID, artifact)
		if err != nil {
			continue
		}

		var result gate.GateResult
		if err := json.Unmarshal(data, &result); err != nil {
			continue
		}

		if result.SchemaVersion == "codefoundry_gate_report.v1" {
			results = append(results, result)
		}
	}

	return results, nil
}

// loadReviewResult loads the review result from a stage
func (g *Generator) loadReviewResult(stageID string) (*review.ReviewResult, error) {
	data, err := g.artifactStore.Read(stageID, "review-result.json")
	if err != nil {
		return nil, err
	}

	return review.FromJSON(data)
}

// loadLockDecision loads the lock decision from a stage
func (g *Generator) loadLockDecision(stageID string) (*lock.LockDecision, error) {
	data, err := g.artifactStore.Read(stageID, "lock-decision.json")
	if err != nil {
		return nil, err
	}

	return lock.FromJSON(data)
}

// loadStageStatus loads the stage status
func (g *Generator) loadStageStatus(stageID string) (*stageStatusJSON, error) {
	data, err := g.artifactStore.Read(stageID, "status.json")
	if err != nil {
		return nil, err
	}

	var status stageStatusJSON
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, err
	}

	return &status, nil
}

// stageStatusJSON represents the stage status file structure
type stageStatusJSON struct {
	SchemaVersion string `json:"schema_version"`
	StageID       string `json:"stage_id"`
	Status        string `json:"status"`
	StartedAt     string `json:"started_at,omitempty"`
	CompletedAt   string `json:"completed_at,omitempty"`
}

// calculateOverallStatus calculates the overall run status
func (g *Generator) calculateOverallStatus(report *Report) string {
	// Check if any stage failed
	for _, stage := range report.Stages {
		if stage.Status == "fail" {
			return "fail"
		}
	}

	// Check lock decision
	if report.LockDecision != nil {
		if report.LockDecision.Decision == lock.DecisionReopen {
			return "fail"
		}
	}

	// Check if all stages complete
	allComplete := true
	for _, stage := range report.Stages {
		if stage.Status != "pass" && stage.Status != "skip" {
			allComplete = false
			break
		}
	}

	if allComplete {
		return "pass"
	}

	return "in_progress"
}

// renderMarkdown renders report as Markdown
func (g *Generator) renderMarkdown(report *Report) string {
	var md string

	md += fmt.Sprintf("# CodeFoundry Report\n\n")
	md += fmt.Sprintf("**Run ID:** %s\n\n", report.RunID)
	md += fmt.Sprintf("**Status:** %s\n\n", report.Status)
	md += fmt.Sprintf("**Started:** %s\n\n", report.StartedAt.Format(time.RFC3339))

	if !report.CompletedAt.IsZero() {
		md += fmt.Sprintf("**Completed:** %s\n\n", report.CompletedAt.Format(time.RFC3339))
	}

	// Stages
	md += "## Stages\n\n"
	md += "| Stage | Status | Duration |\n"
	md += "|-------|--------|----------|\n"
	for _, stage := range report.Stages {
		status := stage.Status
		if status == "" {
			status = "unknown"
		}
		md += fmt.Sprintf("| %s | %s | %s |\n", stage.StageID, status, stage.Duration)
	}
	md += "\n"

	// Gate Reports
	if len(report.GateReports) > 0 {
		md += "## Gates\n\n"
		for _, gate := range report.GateReports {
			md += fmt.Sprintf("### %s\n\n", gate.GateID)
			md += fmt.Sprintf("- **Status:** %s\n", gate.Status)
			md += fmt.Sprintf("- **Command:** %s\n", gate.Command)
			md += fmt.Sprintf("- **Duration:** %dms\n", gate.DurationMs)
			
			if len(gate.Failures) > 0 {
				md += "\n**Failures:**\n"
				for _, f := range gate.Failures {
					md += fmt.Sprintf("- %s:%d - %s\n", f.File, f.Line, f.Message)
				}
			}
			md += "\n"
		}
	}

	// Review Result
	if report.ReviewResult != nil {
		md += "## Review\n\n"
		md += fmt.Sprintf("**Rubric Score:** %d/100\n\n", report.ReviewResult.RubricScore)
		md += fmt.Sprintf("**Confidence:** %.2f\n\n", report.ReviewResult.ConfidenceScore)
		
		md += "### Dimensions\n\n"
		md += fmt.Sprintf("- Correctness: %d/5\n", report.ReviewResult.Dimensions.Correctness)
		md += fmt.Sprintf("- Efficiency: %d/5\n", report.ReviewResult.Dimensions.Efficiency)
		md += fmt.Sprintf("- Maintainability: %d/5\n", report.ReviewResult.Dimensions.Maintainability)
		md += fmt.Sprintf("- Safety: %d/5\n\n", report.ReviewResult.Dimensions.Safety)
		
		if report.ReviewResult.P1Count > 0 || report.ReviewResult.P2Count > 0 || report.ReviewResult.P3Count > 0 {
			md += "### Findings\n\n"
			md += fmt.Sprintf("- P1 (Must fix): %d\n", report.ReviewResult.P1Count)
			md += fmt.Sprintf("- P2 (Should fix): %d\n", report.ReviewResult.P2Count)
			md += fmt.Sprintf("- P3 (Nice to fix): %d\n\n", report.ReviewResult.P3Count)
		}
		
		if report.ReviewResult.Summary != "" {
			md += fmt.Sprintf("### Summary\n\n%s\n\n", report.ReviewResult.Summary)
		}
	}

	// Lock Decision
	if report.LockDecision != nil {
		md += "## Lock Decision\n\n"
		md += fmt.Sprintf("**Decision:** %s\n\n", report.LockDecision.Decision)
		md += fmt.Sprintf("**Reason:** %s\n\n", report.LockDecision.Reason)
		
		if report.LockDecision.EscalationRequired {
			md += fmt.Sprintf("**Escalation Required:** Yes\n\n")
			md += fmt.Sprintf("**Escalation Reason:** %s\n\n", report.LockDecision.EscalationReason)
		}
	}

	return md
}

// renderCI renders report for CI systems
func (g *Generator) renderCI(report *Report) string {
	var output string

	// GitHub Actions style annotations
	for _, gate := range report.GateReports {
		for _, failure := range gate.Failures {
			if failure.File != "" {
				output += fmt.Sprintf("::error file=%s,line=%d::%s\n", 
					failure.File, failure.Line, failure.Message)
			} else {
				output += fmt.Sprintf("::error::%s: %s\n", gate.GateID, failure.Message)
			}
		}
	}

	if report.ReviewResult != nil {
		for _, finding := range report.ReviewResult.Findings {
			if finding.Severity == review.SeverityP1 {
				if finding.File != "" {
					output += fmt.Sprintf("::error file=%s,line=%d::[%s] %s\n",
						finding.File, finding.Line, finding.Severity, finding.Message)
				} else {
					output += fmt.Sprintf("::error::[%s] %s\n", finding.Severity, finding.Message)
				}
			} else if finding.Severity == review.SeverityP2 {
				if finding.File != "" {
					output += fmt.Sprintf("::warning file=%s,line=%d::[%s] %s\n",
						finding.File, finding.Line, finding.Severity, finding.Message)
				} else {
					output += fmt.Sprintf("::warning::[%s] %s\n", finding.Severity, finding.Message)
				}
			}
		}
	}

	// Summary
	output += fmt.Sprintf("\n## CodeFoundry Summary\n\n")
	output += fmt.Sprintf("Status: %s\n", report.Status)
	
	if report.LockDecision != nil {
		output += fmt.Sprintf("Lock Decision: %s\n", report.LockDecision.Decision)
	}

	return output
}
