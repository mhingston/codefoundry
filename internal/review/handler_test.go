package review

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/gate"
	"github.com/mhingston/codefoundry/internal/protocol"
)

func setupTestStore(t *testing.T) (*artifact.Store, string, func()) {
	tmpDir, err := os.MkdirTemp("", "review-test-*")
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

func TestNewHandler(t *testing.T) {
	store, tmpDir, cleanup := setupTestStore(t)
	defer cleanup()

	opts := HandlerOptions{
		ArtifactStore:       store,
		TemplatePath:        filepath.Join(tmpDir, "template.md"),
		ConfidenceThreshold: 0.8,
	}

	handler := NewHandler(opts)

	if handler == nil {
		t.Fatal("NewHandler returned nil")
	}

	if handler.confidenceThreshold != 0.8 {
		t.Errorf("confidenceThreshold = %v, want 0.8", handler.confidenceThreshold)
	}

	if handler.artifactStore != store {
		t.Error("artifactStore not set correctly")
	}
}

func TestNewHandler_DefaultThreshold(t *testing.T) {
	store, tmpDir, cleanup := setupTestStore(t)
	defer cleanup()

	opts := HandlerOptions{
		ArtifactStore: store,
		TemplatePath:  filepath.Join(tmpDir, "template.md"),
		// ConfidenceThreshold not set
	}

	handler := NewHandler(opts)

	if handler.confidenceThreshold != 0.7 {
		t.Errorf("default confidenceThreshold = %v, want 0.7", handler.confidenceThreshold)
	}
}

func TestHandler_WithConfidenceThreshold(t *testing.T) {
	handler := NewHandler(HandlerOptions{})
	handler.WithConfidenceThreshold(0.9)

	if handler.confidenceThreshold != 0.9 {
		t.Errorf("confidenceThreshold = %v, want 0.9", handler.confidenceThreshold)
	}
}

func TestHandler_ExecuteReview(t *testing.T) {
	store, tmpDir, cleanup := setupTestStore(t)
	defer cleanup()

	// Create template file
	templatePath := filepath.Join(tmpDir, "review.md")
	templateContent := "Test template for {{.StageID}}"
	if err := os.WriteFile(templatePath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	opts := HandlerOptions{
		ArtifactStore:       store,
		TemplatePath:        templatePath,
		ConfidenceThreshold: 0.7,
	}

	handler := NewHandler(opts)

	stage := &protocol.Stage{
		ID:   "test-stage",
		Name: "Test Stage",
	}

	gateReports := []gate.GateResult{
		{
			SchemaVersion: "codefoundry_gate_report.v1",
			GateID:        "test-gate",
			Status:        "pass",
		},
	}

	ctx := context.Background()
	result, err := handler.ExecuteReview(ctx, stage, gateReports, "diff content")

	if err != nil {
		t.Errorf("ExecuteReview() error = %v", err)
	}

	if result == nil {
		t.Fatal("ExecuteReview() returned nil result")
	}

	if result.SchemaVersion != "codefoundry_review_result.v1" {
		t.Errorf("SchemaVersion = %v, want codefoundry_review_result.v1", result.SchemaVersion)
	}

	if result.StageID != "test-stage" {
		t.Errorf("StageID = %v, want test-stage", result.StageID)
	}

	// Verify prompt was stored
	if !store.Exists("test-stage", "review-prompt.md") {
		t.Error("review-prompt.md not stored")
	}
}

func TestHandler_ExecuteReview_DefaultTemplate(t *testing.T) {
	store, tmpDir, cleanup := setupTestStore(t)
	defer cleanup()

	// Don't create template file - should use default
	opts := HandlerOptions{
		ArtifactStore:       store,
		TemplatePath:        filepath.Join(tmpDir, "nonexistent.md"),
		ConfidenceThreshold: 0.7,
	}

	handler := NewHandler(opts)

	stage := &protocol.Stage{
		ID: "test-stage",
	}

	ctx := context.Background()
	result, err := handler.ExecuteReview(ctx, stage, nil, "")

	if err != nil {
		t.Errorf("ExecuteReview() with default template error = %v", err)
	}

	if result == nil {
		t.Fatal("ExecuteReview() returned nil result")
	}
}

func TestHandler_ParseReviewResult(t *testing.T) {
	handler := NewHandler(HandlerOptions{})

	// Dimensions: Correctness=4, Efficiency=4, Maintainability=4, Safety=5
	// Weighted avg = 4*0.4 + 4*0.2 + 4*0.2 + 5*0.2 = 1.6 + 0.8 + 0.8 + 1.0 = 4.2
	// Score = 4.2 * 20 = 84
	validJSON := `{
		"schema_version": "codefoundry_review_result.v1",
		"rubric_score": 84,
		"confidence_score": 0.85,
		"confidence_threshold": 0.7,
		"dimensions": {
			"correctness": 4,
			"efficiency": 4,
			"maintainability": 4,
			"safety": 5
		},
		"findings": [
			{
				"id": "finding-1",
				"severity": "P2",
				"file": "test.go",
				"line": 10,
				"message": "Test finding",
				"category": "maintainability"
			}
		],
		"p1_count": 0,
		"p2_count": 1,
		"p3_count": 0,
		"summary": "Test summary",
		"timestamp": "2024-01-15T10:00:00Z"
	}`

	result, err := handler.ParseReviewResult([]byte(validJSON), "test-stage")

	if err != nil {
		t.Errorf("ParseReviewResult() error = %v", err)
	}

	if result == nil {
		t.Fatal("ParseReviewResult() returned nil")
	}

	if result.RubricScore != 84 {
		t.Errorf("RubricScore = %v, want 84", result.RubricScore)
	}

	if result.ConfidenceScore != 0.85 {
		t.Errorf("ConfidenceScore = %v, want 0.85", result.ConfidenceScore)
	}

	if len(result.Findings) != 1 {
		t.Errorf("Findings count = %v, want 1", len(result.Findings))
	}
}

func TestHandler_ParseReviewResult_InvalidJSON(t *testing.T) {
	handler := NewHandler(HandlerOptions{})

	invalidJSON := `{"invalid json`

	_, err := handler.ParseReviewResult([]byte(invalidJSON), "test-stage")

	if err == nil {
		t.Error("ParseReviewResult() expected error for invalid JSON")
	}
}

func TestHandler_StoreAndLoadReviewResult(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	handler := NewHandler(HandlerOptions{
		ArtifactStore: store,
	})

	// Dimensions: Correctness=4, Efficiency=3, Maintainability=4, Safety=4
	// Weighted avg = 4*0.4 + 3*0.2 + 4*0.2 + 4*0.2 = 1.6 + 0.6 + 0.8 + 0.8 = 3.8
	// Score = 3.8 * 20 = 76
	result := NewReviewResult()
	result.StageID = "test-stage"
	result.RubricScore = 76
	result.ConfidenceScore = 0.8
	result.Dimensions = RubricDimensions{
		Correctness:     4,
		Efficiency:      3,
		Maintainability: 4,
		Safety:          4,
	}
	result.P1Count = 0
	result.P2Count = 1
	result.P3Count = 0
	result.Summary = "Test review"

	// Store
	if err := handler.StoreReviewResult("test-stage", result); err != nil {
		t.Errorf("StoreReviewResult() error = %v", err)
	}

	// Load
	loaded, err := handler.LoadReviewResult("test-stage")
	if err != nil {
		t.Errorf("LoadReviewResult() error = %v", err)
	}

	if loaded == nil {
		t.Fatal("LoadReviewResult() returned nil")
	}

	if loaded.RubricScore != 76 {
		t.Errorf("Loaded RubricScore = %v, want 76", loaded.RubricScore)
	}

	if loaded.ConfidenceScore != 0.8 {
		t.Errorf("Loaded ConfidenceScore = %v, want 0.8", loaded.ConfidenceScore)
	}
}

func TestDefaultTemplatePath(t *testing.T) {
	basePath := "/test/path"
	expected := filepath.Join(basePath, "templates", "review.md")
	got := DefaultTemplatePath(basePath)

	if got != expected {
		t.Errorf("DefaultTemplatePath() = %v, want %v", got, expected)
	}
}

func TestEnsureTemplateExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Ensure template doesn't exist yet
	templatePath := DefaultTemplatePath(tmpDir)
	if _, err := os.Stat(templatePath); err == nil {
		os.Remove(templatePath)
	}

	// Ensure template exists
	if err := EnsureTemplateExists(tmpDir); err != nil {
		t.Errorf("EnsureTemplateExists() error = %v", err)
	}

	// Verify template was created
	content, err := os.ReadFile(templatePath)
	if err != nil {
		t.Errorf("Failed to read created template: %v", err)
	}

	if len(content) == 0 {
		t.Error("Template file is empty")
	}

	// Call again - should not error
	if err := EnsureTemplateExists(tmpDir); err != nil {
		t.Errorf("EnsureTemplateExists() on existing template error = %v", err)
	}
}

func TestHandler_ExecuteReview_NoArtifactStore(t *testing.T) {
	store, tmpDir, cleanup := setupTestStore(t)
	defer cleanup()

	// Create template file
	templatePath := filepath.Join(tmpDir, "review.md")
	if err := os.WriteFile(templatePath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	opts := HandlerOptions{
		ArtifactStore:       store,
		TemplatePath:        templatePath,
		ConfidenceThreshold: 0.7,
	}

	handler := NewHandler(opts)
	handler.WithArtifactStore(nil) // Clear artifact store

	stage := &protocol.Stage{
		ID: "test-stage",
	}

	ctx := context.Background()
	result, err := handler.ExecuteReview(ctx, stage, nil, "diff")

	// Should still work but not store prompt
	if err != nil {
		t.Errorf("ExecuteReview() with nil artifact store error = %v", err)
	}

	if result == nil {
		t.Fatal("ExecuteReview() returned nil result")
	}
}

func TestHandler_ParseReviewResult_WithClassification(t *testing.T) {
	handler := NewHandler(HandlerOptions{})

	// JSON with findings that need classification
	jsonData := `{
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
				"severity": "",
				"file": "test.go",
				"message": "This is a security vulnerability",
				"category": "security"
			},
			{
				"id": "finding-2",
				"severity": "",
				"file": "test.go",
				"message": "Performance issue",
				"category": "performance"
			}
		],
		"summary": "Test"
	}`

	result, err := handler.ParseReviewResult([]byte(jsonData), "test-stage")

	if err != nil {
		t.Errorf("ParseReviewResult() error = %v", err)
	}

	if result == nil {
		t.Fatal("ParseReviewResult() returned nil")
	}

	// Verify classifications
	if len(result.Findings) != 2 {
		t.Fatalf("Expected 2 findings, got %d", len(result.Findings))
	}

	if result.Findings[0].Severity != SeverityP1 {
		t.Errorf("Finding 1 severity = %v, want %v", result.Findings[0].Severity, SeverityP1)
	}

	if result.Findings[1].Severity != SeverityP2 {
		t.Errorf("Finding 2 severity = %v, want %v", result.Findings[1].Severity, SeverityP2)
	}

	// Check counts were updated
	if result.P1Count != 1 || result.P2Count != 1 {
		t.Errorf("Counts: P1=%d, P2=%d, want P1=1, P2=1", result.P1Count, result.P2Count)
	}
}

func TestHandler_ParseReviewResult_InvalidData(t *testing.T) {
	handler := NewHandler(HandlerOptions{})

	tests := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{
			name:    "empty JSON",
			data:    []byte(`{}`),
			wantErr: true,
		},
		{
			name:    "missing schema version",
			data:    []byte(`{"rubric_score": 80}`),
			wantErr: true,
		},
		{
			name:    "invalid schema version",
			data:    []byte(`{"schema_version": "invalid.v1"}`),
			wantErr: true,
		},
		{
			name:    "not JSON",
			data:    []byte(`not json at all`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := handler.ParseReviewResult(tt.data, "test-stage")
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseReviewResult() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_StoreReviewResult_NoStore(t *testing.T) {
	handler := NewHandler(HandlerOptions{})

	result := NewReviewResult()
	result.RubricScore = 80

	err := handler.StoreReviewResult("test-stage", result)
	if err == nil {
		t.Error("StoreReviewResult() expected error when artifact store is nil")
	}
}

func TestHandler_LoadReviewResult_NoStore(t *testing.T) {
	handler := NewHandler(HandlerOptions{})

	_, err := handler.LoadReviewResult("test-stage")
	if err == nil {
		t.Error("LoadReviewResult() expected error when artifact store is nil")
	}
}

func TestHandler_LoadReviewResult_NotFound(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	handler := NewHandler(HandlerOptions{
		ArtifactStore: store,
	})

	_, err := handler.LoadReviewResult("nonexistent-stage")
	if err == nil {
		t.Error("LoadReviewResult() expected error when result not found")
	}
}

func TestHandler_LoadDiff_NoStore(t *testing.T) {
	handler := NewHandler(HandlerOptions{})

	_, err := handler.LoadDiff("stage-1")
	if err == nil {
		t.Error("LoadDiff() expected error when artifact store is nil")
	}
}

func TestHandler_LoadDiff_NotFound(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	handler := NewHandler(HandlerOptions{
		ArtifactStore: store,
	})

	_, err := handler.LoadDiff("nonexistent-stage")
	if err == nil {
		t.Error("LoadDiff() expected error when diff not found")
	}
}

func TestHandler_LoadGateReports_NoStore(t *testing.T) {
	handler := NewHandler(HandlerOptions{})

	_, err := handler.LoadGateReports("test-stage")
	if err == nil {
		t.Error("LoadGateReports() expected error when artifact store is nil")
	}
}

func TestHandler_LoadGateReports_WithReports(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	handler := NewHandler(HandlerOptions{
		ArtifactStore: store,
	})

	// Store a gate report
	gateReport := map[string]interface{}{
		"schema_version": "codefoundry_gate_report.v1",
		"gate_id":        "test-gate",
		"status":         "pass",
	}
	store.WriteJSON("test-stage", "gate-report.json", gateReport)

	reports, err := handler.LoadGateReports("test-stage")
	if err != nil {
		t.Errorf("LoadGateReports() error = %v", err)
	}

	if len(reports) != 1 {
		t.Errorf("LoadGateReports() returned %d reports, want 1", len(reports))
	}

	if len(reports) > 0 && reports[0].GateID != "test-gate" {
		t.Errorf("GateID = %v, want test-gate", reports[0].GateID)
	}
}

func TestHandler_ProcessHarnessOutput(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	handler := NewHandler(HandlerOptions{
		ArtifactStore: store,
	})

	validJSON := `{
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
		"findings": [],
		"summary": "Test"
	}`

	result, err := handler.ProcessHarnessOutput("test-stage", []byte(validJSON))
	if err != nil {
		t.Errorf("ProcessHarnessOutput() error = %v", err)
	}

	if result == nil {
		t.Fatal("ProcessHarnessOutput() returned nil")
	}

	// Verify it was stored
	if !store.Exists("test-stage", "review-result.json") {
		t.Error("ProcessHarnessOutput() did not store result")
	}
}

func TestHandler_ProcessHarnessOutput_InvalidData(t *testing.T) {
	handler := NewHandler(HandlerOptions{})

	_, err := handler.ProcessHarnessOutput("test-stage", []byte(`invalid`))
	if err == nil {
		t.Error("ProcessHarnessOutput() expected error for invalid data")
	}
}

func TestHandler_WithArtifactStore(t *testing.T) {
	handler := NewHandler(HandlerOptions{})
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	handler.WithArtifactStore(store)
	if handler.artifactStore != store {
		t.Error("WithArtifactStore() did not set store correctly")
	}
}

func TestEnsureTemplateExists_CreateDirError(t *testing.T) {
	// Try to create template in a location that doesn't allow directory creation
	// This test might be OS-dependent
	tmpDir, err := os.MkdirTemp("", "template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// First call should succeed
	if err := EnsureTemplateExists(tmpDir); err != nil {
		t.Errorf("EnsureTemplateExists() error = %v", err)
	}

	// Verify template exists
	templatePath := DefaultTemplatePath(tmpDir)
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		t.Error("Template was not created")
	}
}

func TestReviewResult_IsPass(t *testing.T) {
	tests := []struct {
		name      string
		result    ReviewResult
		threshold float64
		wantPass  bool
	}{
		{
			name: "pass - high confidence, no P1",
			result: ReviewResult{
				ConfidenceScore: 0.9,
				P1Count:         0,
			},
			threshold: 0.7,
			wantPass:  true,
		},
		{
			name: "fail - low confidence",
			result: ReviewResult{
				ConfidenceScore: 0.6,
				P1Count:         0,
			},
			threshold: 0.7,
			wantPass:  false,
		},
		{
			name: "fail - has P1 findings",
			result: ReviewResult{
				ConfidenceScore: 0.9,
				P1Count:         1,
			},
			threshold: 0.7,
			wantPass:  false,
		},
		{
			name: "fail - both issues",
			result: ReviewResult{
				ConfidenceScore: 0.5,
				P1Count:         2,
			},
			threshold: 0.7,
			wantPass:  false,
		},
		{
			name: "pass - exactly at threshold",
			result: ReviewResult{
				ConfidenceScore: 0.7,
				P1Count:         0,
			},
			threshold: 0.7,
			wantPass:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.IsPass(tt.threshold)
			if got != tt.wantPass {
				t.Errorf("IsPass() = %v, want %v", got, tt.wantPass)
			}
		})
	}
}

func TestReviewResult_GetFindingsBySeverity(t *testing.T) {
	result := ReviewResult{
		Findings: []Finding{
			{ID: "1", Severity: SeverityP1},
			{ID: "2", Severity: SeverityP2},
			{ID: "3", Severity: SeverityP1},
			{ID: "4", Severity: SeverityP3},
		},
	}

	p1Findings := result.GetFindingsBySeverity(SeverityP1)
	if len(p1Findings) != 2 {
		t.Errorf("P1 findings = %d, want 2", len(p1Findings))
	}

	p2Findings := result.GetFindingsBySeverity(SeverityP2)
	if len(p2Findings) != 1 {
		t.Errorf("P2 findings = %d, want 1", len(p2Findings))
	}

	p3Findings := result.GetFindingsBySeverity(SeverityP3)
	if len(p3Findings) != 1 {
		t.Errorf("P3 findings = %d, want 1", len(p3Findings))
	}

	// Test with empty findings
	emptyResult := ReviewResult{}
	filtered := emptyResult.GetFindingsBySeverity(SeverityP1)
	if len(filtered) != 0 {
		t.Errorf("Empty result findings = %d, want 0", len(filtered))
	}
}

func TestReviewResult_AddFinding(t *testing.T) {
	result := NewReviewResult()

	result.AddFinding(Finding{ID: "1", Severity: SeverityP1})
	if result.P1Count != 1 {
		t.Errorf("P1Count = %d, want 1", result.P1Count)
	}

	result.AddFinding(Finding{ID: "2", Severity: SeverityP2})
	if result.P2Count != 1 {
		t.Errorf("P2Count = %d, want 1", result.P2Count)
	}

	result.AddFinding(Finding{ID: "3", Severity: SeverityP3})
	if result.P3Count != 1 {
		t.Errorf("P3Count = %d, want 1", result.P3Count)
	}

	if len(result.Findings) != 3 {
		t.Errorf("Total findings = %d, want 3", len(result.Findings))
	}
}

func TestReviewResult_CalculateWeightedScore(t *testing.T) {
	result := ReviewResult{
		Dimensions: RubricDimensions{
			Correctness:     4,
			Efficiency:      3,
			Maintainability: 5,
			Safety:          4,
		},
	}

	weights := RubricWeights{
		Correctness:     0.4,
		Efficiency:      0.2,
		Maintainability: 0.2,
		Safety:          0.2,
	}

	// Expected: 4*0.4 + 3*0.2 + 5*0.2 + 4*0.2 = 1.6 + 0.6 + 1.0 + 0.8 = 4.0
	got := result.CalculateWeightedScore(weights)
	expected := 4.0
	if got != expected {
		t.Errorf("CalculateWeightedScore() = %v, want %v", got, expected)
	}
}

func TestDefaultWeights(t *testing.T) {
	weights := DefaultWeights()

	if weights.Correctness != 0.4 {
		t.Errorf("Correctness weight = %v, want 0.4", weights.Correctness)
	}

	if weights.Efficiency != 0.2 {
		t.Errorf("Efficiency weight = %v, want 0.2", weights.Efficiency)
	}

	if weights.Maintainability != 0.2 {
		t.Errorf("Maintainability weight = %v, want 0.2", weights.Maintainability)
	}

	if weights.Safety != 0.2 {
		t.Errorf("Safety weight = %v, want 0.2", weights.Safety)
	}
}

func TestErrors(t *testing.T) {
	t.Run("TemplateError", func(t *testing.T) {
		err := TemplateError{
			Op:      "render",
			Message: "template failed",
			Cause:   os.ErrNotExist,
		}

		if !strings.Contains(err.Error(), "render") {
			t.Error("TemplateError should contain operation")
		}

		if !strings.Contains(err.Error(), "template failed") {
			t.Error("TemplateError should contain message")
		}

		if err.Unwrap() != os.ErrNotExist {
			t.Error("TemplateError.Unwrap() should return cause")
		}
	})

	t.Run("TemplateError without cause", func(t *testing.T) {
		err := TemplateError{
			Op:      "render",
			Message: "template failed",
		}

		if err.Unwrap() != nil {
			t.Error("TemplateError.Unwrap() should return nil when no cause")
		}
	})

	t.Run("ExecutionError", func(t *testing.T) {
		err := ExecutionError{
			StageID: "test-stage",
			Message: "execution failed",
			Cause:   os.ErrNotExist,
		}

		if !strings.Contains(err.Error(), "test-stage") {
			t.Error("ExecutionError should contain stage ID")
		}

		if !strings.Contains(err.Error(), "execution failed") {
			t.Error("ExecutionError should contain message")
		}

		if err.Unwrap() != os.ErrNotExist {
			t.Error("ExecutionError.Unwrap() should return cause")
		}
	})

	t.Run("ExecutionError without cause", func(t *testing.T) {
		err := ExecutionError{
			StageID: "test-stage",
			Message: "execution failed",
		}

		if err.Unwrap() != nil {
			t.Error("ExecutionError.Unwrap() should return nil when no cause")
		}
	})
}
