package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/gate"
)

func setupTestArtifacts(t *testing.T) (*artifact.Store, string, func()) {
	tmpDir, err := os.MkdirTemp("", "report-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	ns := artifact.NewNamespace(tmpDir, "test-run")
	store := artifact.NewStore(ns)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return store, tmpDir, cleanup
}

func createStageArtifacts(store *artifact.Store, stageID string) error {
	// Create status
	status := map[string]interface{}{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       stageID,
		"status":         "pass",
		"started_at":     time.Now().UTC().Format(time.RFC3339),
		"completed_at":   time.Now().UTC().Format(time.RFC3339),
	}
	if err := store.WriteJSON(stageID, "status.json", status); err != nil {
		return err
	}

	// Create a gate report
	gateResult := gate.GateResult{
		SchemaVersion: "codefoundry_gate_report.v1",
		GateID:        "test-gate",
		Status:        "pass",
		Command:       "go test ./...",
		DurationMs:    5000,
	}
	if err := store.WriteJSON(stageID, "test-gate.json", gateResult); err != nil {
		return err
	}

	return nil
}

func TestNewGenerator(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	generator := NewGenerator(store, tmpDir)

	if generator == nil {
		t.Fatal("NewGenerator returned nil")
	}

	if generator.artifactStore != store {
		t.Error("artifactStore not set correctly")
	}

	if generator.basePath != tmpDir {
		t.Error("basePath not set correctly")
	}
}

func TestGenerator_GenerateReport(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create stage artifacts
	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	generator := NewGenerator(store, tmpDir)

	report, err := generator.GenerateReport("test-run", FormatJSON)
	if err != nil {
		t.Errorf("GenerateReport() error = %v", err)
	}

	if report == nil {
		t.Fatal("GenerateReport() returned nil")
	}

	if report.RunID != "test-run" {
		t.Errorf("RunID = %v, want test-run", report.RunID)
	}

	if len(report.Stages) != 1 {
		t.Errorf("Stages count = %v, want 1", len(report.Stages))
	}

	if len(report.GateReports) != 1 {
		t.Errorf("GateReports count = %v, want 1", len(report.GateReports))
	}
}

func TestGenerator_GenerateJSON(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	generator := NewGenerator(store, tmpDir)

	jsonData, err := generator.GenerateJSON("test-run")
	if err != nil {
		t.Errorf("GenerateJSON() error = %v", err)
	}

	if len(jsonData) == 0 {
		t.Error("GenerateJSON() returned empty data")
	}

	// Verify it's valid JSON
	var report Report
	if err := json.Unmarshal(jsonData, &report); err != nil {
		t.Errorf("GenerateJSON() returned invalid JSON: %v", err)
	}
}

func TestGenerator_GenerateMarkdown(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	generator := NewGenerator(store, tmpDir)

	md, err := generator.GenerateMarkdown("test-run")
	if err != nil {
		t.Errorf("GenerateMarkdown() error = %v", err)
	}

	if len(md) == 0 {
		t.Error("GenerateMarkdown() returned empty markdown")
	}

	// Check for expected content
	if !contains(md, "# CodeFoundry Report") {
		t.Error("Markdown missing title")
	}

	if !contains(md, "test-stage") {
		t.Error("Markdown missing stage name")
	}
}

func TestGenerator_GenerateCI(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create artifacts with failures
	status := map[string]interface{}{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       "test-stage",
		"status":         "fail",
		"started_at":     time.Now().UTC().Format(time.RFC3339),
	}
	store.WriteJSON("test-stage", "status.json", status)

	gateResult := gate.GateResult{
		SchemaVersion: "codefoundry_gate_report.v1",
		GateID:        "test-gate",
		Status:        "fail",
		Command:       "go test ./...",
		Failures: []gate.GateFailure{
			{Message: "Test failed", File: "test.go", Line: 10},
		},
	}
	store.WriteJSON("test-stage", "test-gate.json", gateResult)

	generator := NewGenerator(store, tmpDir)

	ci, err := generator.GenerateCI("test-run")
	if err != nil {
		t.Errorf("GenerateCI() error = %v", err)
	}

	if len(ci) == 0 {
		t.Error("GenerateCI() returned empty output")
	}

	// Check for GitHub Actions style annotation
	if !contains(ci, "::error") {
		t.Error("CI output missing error annotation")
	}
}

func TestGenerator_SaveReport(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	generator := NewGenerator(store, tmpDir)
	outputPath := filepath.Join(tmpDir, "report.json")

	if err := generator.SaveReport("test-run", FormatJSON, outputPath); err != nil {
		t.Errorf("SaveReport() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("SaveReport() did not create output file")
	}

	// Verify content
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Errorf("Failed to read saved report: %v", err)
	}

	if len(data) == 0 {
		t.Error("Saved report is empty")
	}
}

func TestGenerator_UnsupportedFormat(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	generator := NewGenerator(store, tmpDir)

	err := generator.SaveReport("test-run", ReportFormat("invalid"), "/tmp/out.txt")
	if err == nil {
		t.Error("SaveReport() expected error for unsupported format")
	}
}

func TestGenerator_NoArtifactStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "report-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	generator := NewGenerator(nil, tmpDir)

	_, err = generator.GenerateReport("test-run", FormatJSON)
	if err == nil {
		t.Error("GenerateReport() expected error when artifact store is nil")
	}
}

func TestReport_calculateOverallStatus(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	generator := NewGenerator(store, tmpDir)

	tests := []struct {
		name     string
		stages   []StageStatus
		decision string
		expected string
	}{
		{
			name:     "all pass",
			stages:   []StageStatus{{Status: "pass"}},
			expected: "pass",
		},
		{
			name:     "one fail",
			stages:   []StageStatus{{Status: "pass"}, {Status: "fail"}},
			expected: "fail",
		},
		{
			name:     "in progress",
			stages:   []StageStatus{{Status: "pass"}, {Status: "running"}},
			expected: "in_progress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &Report{
				Stages: tt.stages,
			}
			if tt.decision != "" {
				// We can't easily set the lock decision here
				// So we skip that case
			}

			got := generator.calculateOverallStatus(report)
			if got != tt.expected {
				t.Errorf("calculateOverallStatus() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsSubstring(s, substr)))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestGenerator_GenerateReport_WithReviewResult(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create stage with review result
	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Create review result
	reviewResult := `{
		"schema_version": "codefoundry_review_result.v1",
		"rubric_score": 80,
		"confidence_score": 0.85,
		"confidence_threshold": 0.7,
		"dimensions": {
			"correctness": 4,
			"efficiency": 4,
			"maintainability": 4,
			"safety": 4
		},
		"findings": [
			{
				"id": "finding-1",
				"severity": "P1",
				"file": "test.go",
				"message": "Test finding",
				"category": "security"
			}
		],
		"p1_count": 1,
		"p2_count": 0,
		"p3_count": 0,
		"summary": "Test summary",
		"timestamp": "2024-01-15T10:00:00Z"
	}`
	store.Write("test-stage", "review-result.json", []byte(reviewResult))

	generator := NewGenerator(store, tmpDir)
	report, err := generator.GenerateReport("test-run", FormatJSON)

	if err != nil {
		t.Errorf("GenerateReport() error = %v", err)
	}

	if report == nil {
		t.Fatal("GenerateReport() returned nil")
	}

	if report.ReviewResult == nil {
		t.Error("ReviewResult should not be nil")
	}

	if report.ReviewResult != nil && report.ReviewResult.RubricScore != 80 {
		t.Errorf("RubricScore = %v, want 80", report.ReviewResult.RubricScore)
	}
}

func TestGenerator_GenerateReport_WithLockDecision(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create stage with lock decision
	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Create lock decision
	lockDecision := `{
		"schema_version": "codefoundry_lock_decision.v1",
		"decision": "resolved",
		"reason": "All checks passed",
		"required_gate_ids": ["test-gate"],
		"passed_gate_ids": ["test-gate"],
		"failed_gate_ids": [],
		"confidence_score": 0.85,
		"confidence_threshold": 0.7,
		"p1_findings": 0,
		"p2_findings": 1,
		"p3_findings": 2,
		"rubric_score": 80,
		"escalation_required": false,
		"timestamp": "2024-01-15T10:00:00Z"
	}`
	store.Write("test-stage", "lock-decision.json", []byte(lockDecision))

	generator := NewGenerator(store, tmpDir)
	report, err := generator.GenerateReport("test-run", FormatJSON)

	if err != nil {
		t.Errorf("GenerateReport() error = %v", err)
	}

	if report == nil {
		t.Fatal("GenerateReport() returned nil")
	}

	if report.LockDecision == nil {
		t.Error("LockDecision should not be nil")
	}

	if report.LockDecision != nil && report.LockDecision.Decision != "resolved" {
		t.Errorf("Decision = %v, want resolved", report.LockDecision.Decision)
	}
}

func TestGenerator_GenerateReport_WithFailedLockDecision(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create stage with failed lock decision
	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Update status to fail
	status := map[string]interface{}{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       "test-stage",
		"status":         "pass",
	}
	store.WriteJSON("test-stage", "status.json", status)

	// Create lock decision with reopen
	lockDecision := `{
		"schema_version": "codefoundry_lock_decision.v1",
		"decision": "reopen",
		"reason": "P1 findings found",
		"required_gate_ids": ["test-gate"],
		"passed_gate_ids": [],
		"failed_gate_ids": ["test-gate"],
		"confidence_score": 0.85,
		"confidence_threshold": 0.7,
		"p1_findings": 2,
		"escalation_required": true,
		"escalation_reason": "P1 findings must be fixed",
		"timestamp": "2024-01-15T10:00:00Z"
	}`
	store.Write("test-stage", "lock-decision.json", []byte(lockDecision))

	generator := NewGenerator(store, tmpDir)
	report, err := generator.GenerateReport("test-run", FormatJSON)

	if err != nil {
		t.Errorf("GenerateReport() error = %v", err)
	}

	if report == nil {
		t.Fatal("GenerateReport() returned nil")
	}

	// Overall status should be fail due to reopen decision
	if report.Status != "fail" {
		t.Errorf("Status = %v, want fail", report.Status)
	}
}

func TestGenerator_GenerateReport_MultipleStages(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create multiple stages
	if err := createStageArtifacts(store, "stage-1"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}
	if err := createStageArtifacts(store, "stage-2"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	generator := NewGenerator(store, tmpDir)
	report, err := generator.GenerateReport("test-run", FormatJSON)

	if err != nil {
		t.Errorf("GenerateReport() error = %v", err)
	}

	if report == nil {
		t.Fatal("GenerateReport() returned nil")
	}

	if len(report.Stages) != 2 {
		t.Errorf("Stages count = %v, want 2", len(report.Stages))
	}
}

func TestGenerator_GenerateMarkdown_WithReviewAndLock(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Add review result
	reviewResult := `{
		"schema_version": "codefoundry_review_result.v1",
		"rubric_score": 85,
		"confidence_score": 0.9,
		"confidence_threshold": 0.7,
		"dimensions": {
			"correctness": 5,
			"efficiency": 4,
			"maintainability": 4,
			"safety": 5
		},
		"findings": [
			{
				"id": "finding-1",
				"severity": "P1",
				"file": "test.go",
				"line": 42,
				"message": "Critical issue",
				"category": "security"
			}
		],
		"p1_count": 1,
		"p2_count": 0,
		"p3_count": 0,
		"summary": "Found critical issues",
		"timestamp": "2024-01-15T10:00:00Z"
	}`
	store.Write("test-stage", "review-result.json", []byte(reviewResult))

	// Add lock decision with escalation
	lockDecision := `{
		"schema_version": "codefoundry_lock_decision.v1",
		"decision": "reopen",
		"reason": "P1 findings found",
		"required_gate_ids": ["test-gate"],
		"passed_gate_ids": ["test-gate"],
		"failed_gate_ids": [],
		"confidence_score": 0.9,
		"confidence_threshold": 0.7,
		"p1_findings": 1,
		"escalation_required": true,
		"escalation_reason": "1 P1 finding(s) must be fixed before proceeding",
		"timestamp": "2024-01-15T10:00:00Z"
	}`
	store.Write("test-stage", "lock-decision.json", []byte(lockDecision))

	generator := NewGenerator(store, tmpDir)
	md, err := generator.GenerateMarkdown("test-run")

	if err != nil {
		t.Errorf("GenerateMarkdown() error = %v", err)
	}

	if len(md) == 0 {
		t.Error("GenerateMarkdown() returned empty markdown")
	}

	// Check for various sections
	expectedSections := []string{
		"# CodeFoundry Report",
		"## Stages",
		"## Gates",
		"## Review",
		"Rubric Score:",
		"Confidence:",
		"## Lock Decision",
		"**Decision:**",
		"**Reason:**",
		"**Escalation Required:**",
	}

	for _, section := range expectedSections {
		if !contains(md, section) {
			t.Errorf("Markdown missing section: %s", section)
		}
	}
}

func TestGenerator_GenerateCI_WithFindings(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Add review result with P1 and P2 findings
	reviewResult := `{
		"schema_version": "codefoundry_review_result.v1",
		"rubric_score": 70,
		"confidence_score": 0.8,
		"confidence_threshold": 0.7,
		"dimensions": {
			"correctness": 3,
			"efficiency": 4,
			"maintainability": 3,
			"safety": 4
		},
		"findings": [
			{
				"id": "finding-1",
				"severity": "P1",
				"file": "test.go",
				"line": 10,
				"message": "Critical security issue"
			},
			{
				"id": "finding-2",
				"severity": "P2",
				"file": "test.go",
				"line": 20,
				"message": "Performance issue"
			}
		],
		"p1_count": 1,
		"p2_count": 1,
		"p3_count": 0,
		"summary": "Test summary",
		"timestamp": "2024-01-15T10:00:00Z"
	}`
	store.Write("test-stage", "review-result.json", []byte(reviewResult))

	generator := NewGenerator(store, tmpDir)
	ci, err := generator.GenerateCI("test-run")

	if err != nil {
		t.Errorf("GenerateCI() error = %v", err)
	}

	if len(ci) == 0 {
		t.Error("GenerateCI() returned empty output")
	}

	// Check for GitHub Actions annotations
	if !contains(ci, "::error") {
		t.Error("CI output missing error annotations")
	}

	if !contains(ci, "::warning") {
		t.Error("CI output missing warning annotations")
	}

	if !contains(ci, "[P1]") {
		t.Error("CI output missing P1 severity marker")
	}

	if !contains(ci, "[P2]") {
		t.Error("CI output missing P2 severity marker")
	}
}

func TestGenerator_GenerateCI_WithGateFailures(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create stage with failed gate
	status := map[string]interface{}{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       "test-stage",
		"status":         "fail",
	}
	store.WriteJSON("test-stage", "status.json", status)

	gateResult := gate.GateResult{
		SchemaVersion: "codefoundry_gate_report.v1",
		GateID:        "test-gate",
		Status:        "fail",
		Command:       "go test",
		Failures: []gate.GateFailure{
			{Message: "Test failed", File: "test.go", Line: 42},
			{Message: "Another failure"},
		},
	}
	store.WriteJSON("test-stage", "test-gate.json", gateResult)

	generator := NewGenerator(store, tmpDir)
	ci, err := generator.GenerateCI("test-run")

	if err != nil {
		t.Errorf("GenerateCI() error = %v", err)
	}

	if len(ci) == 0 {
		t.Error("GenerateCI() returned empty output")
	}

	// Check for gate error annotations
	if !contains(ci, "::error") {
		t.Error("CI output missing error annotations")
	}

	if !contains(ci, "test.go") {
		t.Error("CI output missing file reference")
	}

	if !contains(ci, "line=42") {
		t.Error("CI output missing line reference")
	}
}

func TestGenerator_SaveReport_Markdown(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	generator := NewGenerator(store, tmpDir)
	outputPath := filepath.Join(tmpDir, "report.md")

	err := generator.SaveReport("test-run", FormatMarkdown, outputPath)
	if err != nil {
		t.Errorf("SaveReport() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("SaveReport() did not create output file")
	}

	// Verify content is markdown
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Errorf("Failed to read saved report: %v", err)
	}

	if !contains(string(data), "# CodeFoundry Report") {
		t.Error("Saved report is not valid markdown")
	}
}

func TestGenerator_SaveReport_CI(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	generator := NewGenerator(store, tmpDir)
	outputPath := filepath.Join(tmpDir, "report.txt")

	err := generator.SaveReport("test-run", FormatCI, outputPath)
	if err != nil {
		t.Errorf("SaveReport() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("SaveReport() did not create output file")
	}

	// Verify content
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Errorf("Failed to read saved report: %v", err)
	}

	if !contains(string(data), "CodeFoundry Summary") {
		t.Error("Saved report missing summary")
	}
}

func TestGenerator_calculateOverallStatus_AllComplete(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	generator := NewGenerator(store, tmpDir)

	report := &Report{
		Stages: []StageStatus{
			{Status: "pass"},
			{Status: "pass"},
		},
	}

	got := generator.calculateOverallStatus(report)
	if got != "pass" {
		t.Errorf("calculateOverallStatus() = %v, want pass", got)
	}
}

func TestGenerator_calculateOverallStatus_SkipAllowed(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	generator := NewGenerator(store, tmpDir)

	report := &Report{
		Stages: []StageStatus{
			{Status: "pass"},
			{Status: "skip"},
			{Status: "pass"},
		},
	}

	got := generator.calculateOverallStatus(report)
	if got != "pass" {
		t.Errorf("calculateOverallStatus() = %v, want pass", got)
	}
}

func TestGenerator_calculateOverallStatus_InProgress(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	generator := NewGenerator(store, tmpDir)

	report := &Report{
		Stages: []StageStatus{
			{Status: "pass"},
			{Status: "running"},
		},
	}

	got := generator.calculateOverallStatus(report)
	if got != "in_progress" {
		t.Errorf("calculateOverallStatus() = %v, want in_progress", got)
	}
}

func TestGenerator_GenerateReport_InvalidStageStatus(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create status with invalid timestamps
	status := map[string]interface{}{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       "test-stage",
		"status":         "pass",
		"started_at":     "invalid-timestamp",
		"completed_at":   "also-invalid",
	}
	store.WriteJSON("test-stage", "status.json", status)

	// Create minimal gate report
	gateResult := gate.GateResult{
		SchemaVersion: "codefoundry_gate_report.v1",
		GateID:        "test-gate",
		Status:        "pass",
	}
	store.WriteJSON("test-stage", "test-gate.json", gateResult)

	generator := NewGenerator(store, tmpDir)
	report, err := generator.GenerateReport("test-run", FormatJSON)

	if err != nil {
		t.Errorf("GenerateReport() error = %v", err)
	}

	if report == nil {
		t.Fatal("GenerateReport() returned nil")
	}

	// Should still work with invalid timestamps
	if len(report.Stages) != 1 {
		t.Errorf("Stages count = %v, want 1", len(report.Stages))
	}
}

func TestGenerator_GenerateReport_CorruptedGateReport(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create stage with corrupted gate report
	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Overwrite with invalid JSON
	store.Write("test-stage", "test-gate.json", []byte(`invalid json`))

	generator := NewGenerator(store, tmpDir)
	report, err := generator.GenerateReport("test-run", FormatJSON)

	if err != nil {
		t.Errorf("GenerateReport() error = %v", err)
	}

	if report == nil {
		t.Fatal("GenerateReport() returned nil")
	}

	// Should have 0 gate reports since corrupted one is skipped
	if len(report.GateReports) != 0 {
		t.Errorf("GateReports count = %v, want 0 (corrupted skipped)", len(report.GateReports))
	}
}

func TestGenerator_GenerateReport_WrongSchemaVersion(t *testing.T) {
	store, tmpDir, cleanup := setupTestArtifacts(t)
	defer cleanup()

	// Create stage with wrong schema version
	if err := createStageArtifacts(store, "test-stage"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Create a JSON file with wrong schema version
	wrongReport := `{
		"schema_version": "wrong.schema.v1",
		"gate_id": "test-gate",
		"status": "pass"
	}`
	store.Write("test-stage", "wrong-gate.json", []byte(wrongReport))

	generator := NewGenerator(store, tmpDir)
	report, err := generator.GenerateReport("test-run", FormatJSON)

	if err != nil {
		t.Errorf("GenerateReport() error = %v", err)
	}

	if report == nil {
		t.Fatal("GenerateReport() returned nil")
	}

	// Should only have the valid gate report
	if len(report.GateReports) != 1 {
		t.Errorf("GateReports count = %v, want 1", len(report.GateReports))
	}
}

func TestGenerateReport_IncludesStageMetadata(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "report-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ns := artifact.NewNamespace(tmpDir, "test-run")
	store := artifact.NewStore(ns)

	status := map[string]interface{}{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       "test-stage",
		"status":         "pass",
		"metadata": map[string]interface{}{
			"variant_selection": map[string]interface{}{"selected_variant": "beta"},
		},
	}
	if err := store.WriteJSON("test-stage", "status.json", status); err != nil {
		t.Fatalf("failed to write status: %v", err)
	}

	generator := NewGenerator(store, tmpDir)
	report, err := generator.GenerateReport("test-run", FormatJSON)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}
	if len(report.Stages) != 1 {
		t.Fatalf("expected 1 stage, got %d", len(report.Stages))
	}
	if report.Stages[0].Metadata == nil {
		t.Fatalf("expected stage metadata to be present")
	}
	if _, ok := report.Stages[0].Metadata["variant_selection"]; !ok {
		t.Fatalf("expected variant_selection in metadata")
	}
}
