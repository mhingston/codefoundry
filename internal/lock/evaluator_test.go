package lock

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/review"
)

func setupTestStore(t *testing.T) (*artifact.Store, string, func()) {
	tmpDir, err := os.MkdirTemp("", "lock-test-*")
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

func TestDefaultLockConfig(t *testing.T) {
	config := DefaultLockConfig()

	if config.ConfidenceThreshold != 0.7 {
		t.Errorf("ConfidenceThreshold = %v, want 0.7", config.ConfidenceThreshold)
	}

	if !config.AutoResolve {
		t.Error("AutoResolve should be true by default")
	}
}

func TestNewEvaluator(t *testing.T) {
	config := LockConfig{
		ConfidenceThreshold: 0.8,
		AutoResolve:         false,
	}

	opts := EvaluatorOptions{
		Config: config,
	}

	evaluator := NewEvaluator(opts)

	if evaluator == nil {
		t.Fatal("NewEvaluator returned nil")
	}

	if evaluator.config.ConfidenceThreshold != 0.8 {
		t.Errorf("config.ConfidenceThreshold = %v, want 0.8", evaluator.config.ConfidenceThreshold)
	}

	if evaluator.config.AutoResolve {
		t.Error("config.AutoResolve should be false")
	}
}

func TestEvaluator_Evaluate_RequiredGateFailed(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{
		Config: DefaultLockConfig(),
	})

	gateResults := []GateResult{
		{GateID: "test-gate", Status: "fail", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore: 0.9,
		P1Count:         0,
	}

	decision, err := evaluator.Evaluate(gateResults, reviewResult)

	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}

	if decision.Decision != DecisionReopen {
		t.Errorf("Decision = %v, want %v", decision.Decision, DecisionReopen)
	}

	if decision.EscalationRequired {
		t.Error("Escalation should not be required for gate failure")
	}

	if !contains(decision.FailedGateIDs, "test-gate") {
		t.Error("FailedGateIDs should contain 'test-gate'")
	}
}

func TestEvaluator_Evaluate_LowConfidence(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{
		Config: LockConfig{
			ConfidenceThreshold: 0.8,
			AutoResolve:         true,
		},
	})

	gateResults := []GateResult{
		{GateID: "test-gate", Status: "pass", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore: 0.5, // Below threshold
		P1Count:         0,
	}

	decision, err := evaluator.Evaluate(gateResults, reviewResult)

	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}

	if decision.Decision != DecisionReopen {
		t.Errorf("Decision = %v, want %v", decision.Decision, DecisionReopen)
	}

	if !decision.EscalationRequired {
		t.Error("Escalation should be required for low confidence")
	}

	if decision.EscalationReason == "" {
		t.Error("EscalationReason should not be empty")
	}
}

func TestEvaluator_Evaluate_P1Findings(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{
		Config: DefaultLockConfig(),
	})

	gateResults := []GateResult{
		{GateID: "test-gate", Status: "pass", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore: 0.9,
		P1Count:         2, // Has P1 findings
		P2Count:         3,
	}

	decision, err := evaluator.Evaluate(gateResults, reviewResult)

	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}

	if decision.Decision != DecisionReopen {
		t.Errorf("Decision = %v, want %v", decision.Decision, DecisionReopen)
	}

	if !decision.EscalationRequired {
		t.Error("Escalation should be required for P1 findings")
	}

	if decision.P1Findings != 2 {
		t.Errorf("P1Findings = %v, want 2", decision.P1Findings)
	}
}

func TestEvaluator_Evaluate_Resolved(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{
		Config: LockConfig{
			ConfidenceThreshold: 0.7,
			AutoResolve:         true,
		},
	})

	gateResults := []GateResult{
		{GateID: "test-gate", Status: "pass", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore: 0.9,
		P1Count:         0,
		P2Count:         1,
		P3Count:         2,
		RubricScore:     80,
	}

	decision, err := evaluator.Evaluate(gateResults, reviewResult)

	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}

	if decision.Decision != DecisionResolved {
		t.Errorf("Decision = %v, want %v", decision.Decision, DecisionResolved)
	}

	if decision.EscalationRequired {
		t.Error("Escalation should not be required when resolved")
	}

	if decision.RubricScore != 80 {
		t.Errorf("RubricScore = %v, want 80", decision.RubricScore)
	}
}

func TestEvaluator_StoreAndLoadDecision(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	evaluator := NewEvaluator(EvaluatorOptions{
		Config:        DefaultLockConfig(),
		ArtifactStore: store,
	})

	decision := NewLockDecision()
	decision.Decision = DecisionResolved
	decision.Reason = "All checks passed"

	// Store
	if err := evaluator.StoreDecision("test-stage", decision); err != nil {
		t.Errorf("StoreDecision() error = %v", err)
	}

	// Load
	loaded, err := evaluator.LoadDecision("test-stage")
	if err != nil {
		t.Errorf("LoadDecision() error = %v", err)
	}

	if loaded == nil {
		t.Fatal("LoadDecision() returned nil")
	}

	if loaded.Decision != DecisionResolved {
		t.Errorf("Loaded Decision = %v, want %v", loaded.Decision, DecisionResolved)
	}

	if loaded.Reason != "All checks passed" {
		t.Errorf("Loaded Reason = %v, want 'All checks passed'", loaded.Reason)
	}
}

func TestEvaluator_DecisionExists(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	evaluator := NewEvaluator(EvaluatorOptions{
		Config:        DefaultLockConfig(),
		ArtifactStore: store,
	})

	// Should not exist initially
	if evaluator.DecisionExists("test-stage") {
		t.Error("DecisionExists should be false initially")
	}

	// Store a decision
	decision := NewLockDecision()
	decision.Decision = DecisionResolved
	decision.Reason = "Test"
	evaluator.StoreDecision("test-stage", decision)

	// Should exist now
	if !evaluator.DecisionExists("test-stage") {
		t.Error("DecisionExists should be true after storing")
	}
}

func TestBuildFromInputs(t *testing.T) {
	gateResults := []GateResult{
		{GateID: "gate1", Status: "pass", Required: true},
		{GateID: "gate2", Status: "fail", Required: false},
		{GateID: "gate3", Status: "pass", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore:     0.85,
		ConfidenceThreshold: 0.7,
		P1Count:             1,
		P2Count:             2,
		P3Count:             3,
		RubricScore:         75,
	}

	config := LockConfig{
		ConfidenceThreshold: 0.8,
	}

	decision := BuildFromInputs(gateResults, reviewResult, config)

	// Check gate IDs
	if len(decision.RequiredGateIDs) != 2 {
		t.Errorf("RequiredGateIDs count = %v, want 2", len(decision.RequiredGateIDs))
	}

	if len(decision.PassedGateIDs) != 2 {
		t.Errorf("PassedGateIDs count = %v, want 2", len(decision.PassedGateIDs))
	}

	if len(decision.FailedGateIDs) != 1 {
		t.Errorf("FailedGateIDs count = %v, want 1", len(decision.FailedGateIDs))
	}

	// Check review data
	if decision.ConfidenceScore != 0.85 {
		t.Errorf("ConfidenceScore = %v, want 0.85", decision.ConfidenceScore)
	}

	if decision.P1Findings != 1 {
		t.Errorf("P1Findings = %v, want 1", decision.P1Findings)
	}

	if decision.RubricScore != 75 {
		t.Errorf("RubricScore = %v, want 75", decision.RubricScore)
	}
}

func TestGateResult_IsPass(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"pass", true},
		{"fail", false},
		{"running", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			g := GateResult{Status: tt.status}
			if got := g.IsPass(); got != tt.expected {
				t.Errorf("IsPass() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGateResult_IsFail(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{"pass", false},
		{"fail", true},
		{"running", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			g := GateResult{Status: tt.status}
			if got := g.IsFail(); got != tt.expected {
				t.Errorf("IsFail() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestLockDecision_IsResolved(t *testing.T) {
	d := &LockDecision{Decision: DecisionResolved}
	if !d.IsResolved() {
		t.Error("IsResolved() should be true")
	}

	d.Decision = DecisionReopen
	if d.IsResolved() {
		t.Error("IsResolved() should be false for reopen")
	}
}

func TestLockDecision_IsReopen(t *testing.T) {
	d := &LockDecision{Decision: DecisionReopen}
	if !d.IsReopen() {
		t.Error("IsReopen() should be true")
	}

	d.Decision = DecisionResolved
	if d.IsReopen() {
		t.Error("IsReopen() should be false for resolved")
	}
}

func TestLockDecision_RequiresEscalation(t *testing.T) {
	d := &LockDecision{EscalationRequired: true}
	if !d.RequiresEscalation() {
		t.Error("RequiresEscalation() should be true")
	}

	d.EscalationRequired = false
	if d.RequiresEscalation() {
		t.Error("RequiresEscalation() should be false")
	}
}

// Helper function
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestEvaluator_Evaluate_ConfidenceThresholdEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		score     float64
		threshold float64
		wantReopen bool
	}{
		{
			name:       "exactly at threshold",
			score:      0.80,
			threshold:  0.80,
			wantReopen: false, // >= threshold should pass
		},
		{
			name:       "just above threshold",
			score:      0.81,
			threshold:  0.80,
			wantReopen: false,
		},
		{
			name:       "just below threshold",
			score:      0.79,
			threshold:  0.80,
			wantReopen: true,
		},
		{
			name:       "zero confidence",
			score:      0.0,
			threshold:  0.7,
			wantReopen: true,
		},
		{
			name:       "maximum confidence",
			score:      1.0,
			threshold:  0.7,
			wantReopen: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewEvaluator(EvaluatorOptions{
				Config: LockConfig{
					ConfidenceThreshold: tt.threshold,
					AutoResolve:         true,
				},
			})

			gateResults := []GateResult{
				{GateID: "test-gate", Status: "pass", Required: true},
			}

			reviewResult := &review.ReviewResult{
				ConfidenceScore: tt.score,
				P1Count:         0,
			}

			decision, err := evaluator.Evaluate(gateResults, reviewResult)

			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
			}

			if tt.wantReopen && decision.Decision != DecisionReopen {
				t.Errorf("Decision = %v, want %v", decision.Decision, DecisionReopen)
			}
			if !tt.wantReopen && decision.Decision != DecisionResolved {
				t.Errorf("Decision = %v, want %v", decision.Decision, DecisionResolved)
			}
		})
	}
}

func TestEvaluator_Evaluate_AllDecisionPaths(t *testing.T) {
	tests := []struct {
		name            string
		gateResults     []GateResult
		reviewResult    *review.ReviewResult
		expectedDecision string
		expectEscalation bool
	}{
		{
			name: "Required gate fails - reopen",
			gateResults: []GateResult{
				{GateID: "gate1", Status: "pass", Required: true},
				{GateID: "gate2", Status: "fail", Required: true},
			},
			reviewResult: &review.ReviewResult{
				ConfidenceScore: 0.9,
				P1Count:         0,
			},
			expectedDecision: DecisionReopen,
			expectEscalation: false,
		},
		{
			name: "All gates pass, low confidence - reopen with escalation",
			gateResults: []GateResult{
				{GateID: "gate1", Status: "pass", Required: true},
			},
			reviewResult: &review.ReviewResult{
				ConfidenceScore: 0.6, // Below threshold of 0.7
				P1Count:         0,
			},
			expectedDecision: DecisionReopen,
			expectEscalation: true,
		},
		{
			name: "All gates pass, good confidence, P1 found - reopen with escalation",
			gateResults: []GateResult{
				{GateID: "gate1", Status: "pass", Required: true},
			},
			reviewResult: &review.ReviewResult{
				ConfidenceScore: 0.9,
				P1Count:         1,
			},
			expectedDecision: DecisionReopen,
			expectEscalation: true,
		},
		{
			name: "All gates pass, good confidence, no P1 - resolved",
			gateResults: []GateResult{
				{GateID: "gate1", Status: "pass", Required: true},
			},
			reviewResult: &review.ReviewResult{
				ConfidenceScore: 0.9,
				P1Count:         0,
				P2Count:         1,
				P3Count:         2,
			},
			expectedDecision: DecisionResolved,
			expectEscalation: false,
		},
		{
			name: "No required gates, good confidence, no P1 - resolved",
			gateResults: []GateResult{
				{GateID: "gate1", Status: "pass", Required: false},
				{GateID: "gate2", Status: "pass", Required: false},
			},
			reviewResult: &review.ReviewResult{
				ConfidenceScore: 0.9,
				P1Count:         0,
			},
			expectedDecision: DecisionResolved,
			expectEscalation: false,
		},
		{
			name: "Multiple P1 findings - reopen",
			gateResults: []GateResult{
				{GateID: "gate1", Status: "pass", Required: true},
			},
			reviewResult: &review.ReviewResult{
				ConfidenceScore: 0.9,
				P1Count:         5,
				P2Count:         3,
				P3Count:         2,
			},
			expectedDecision: DecisionReopen,
			expectEscalation: true,
		},
		{
			name: "P2 and P3 only, good confidence - resolved",
			gateResults: []GateResult{
				{GateID: "gate1", Status: "pass", Required: true},
			},
			reviewResult: &review.ReviewResult{
				ConfidenceScore: 0.85,
				P1Count:         0,
				P2Count:         2,
				P3Count:         5,
			},
			expectedDecision: DecisionResolved,
			expectEscalation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluator := NewEvaluator(EvaluatorOptions{
				Config: LockConfig{
					ConfidenceThreshold: 0.7,
					AutoResolve:         true,
				},
			})

			decision, err := evaluator.Evaluate(tt.gateResults, tt.reviewResult)

			if err != nil {
				t.Errorf("Evaluate() error = %v", err)
			}

			if decision.Decision != tt.expectedDecision {
				t.Errorf("Decision = %v, want %v", decision.Decision, tt.expectedDecision)
			}

			if decision.EscalationRequired != tt.expectEscalation {
				t.Errorf("EscalationRequired = %v, want %v", decision.EscalationRequired, tt.expectEscalation)
			}
		})
	}
}

func TestEvaluator_Evaluate_EmptyGateResults(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{
		Config: LockConfig{
			ConfidenceThreshold: 0.7,
			AutoResolve:         true,
		},
	})

	reviewResult := &review.ReviewResult{
		ConfidenceScore: 0.9,
		P1Count:         0,
	}

	decision, err := evaluator.Evaluate([]GateResult{}, reviewResult)

	if err != nil {
		t.Errorf("Evaluate() error = %v", err)
	}

	if decision.Decision != DecisionResolved {
		t.Errorf("Decision = %v, want %v", decision.Decision, DecisionResolved)
	}
}

func TestEvaluator_WithArtifactStore(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{})
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	evaluator.WithArtifactStore(store)
	if evaluator.artifactStore != store {
		t.Error("WithArtifactStore() did not set store correctly")
	}
}

func TestEvaluator_EvaluateAndStore(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	evaluator := NewEvaluator(EvaluatorOptions{
		Config:        DefaultLockConfig(),
		ArtifactStore: store,
	})

	gateResults := []GateResult{
		{GateID: "test-gate", Status: "pass", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore: 0.9,
		P1Count:         0,
	}

	decision, err := evaluator.EvaluateAndStore("test-stage", gateResults, reviewResult)

	if err != nil {
		t.Errorf("EvaluateAndStore() error = %v", err)
	}

	if decision == nil {
		t.Fatal("EvaluateAndStore() returned nil decision")
	}

	// Verify it was stored
	if !store.Exists("test-stage", "lock-decision.json") {
		t.Error("Decision was not stored")
	}
}

func TestEvaluator_EvaluateAndStore_NoStore(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{
		Config: DefaultLockConfig(),
		// No artifact store
	})

	gateResults := []GateResult{
		{GateID: "test-gate", Status: "pass", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore: 0.9,
		P1Count:         0,
	}

	// Should still work without storing
	decision, err := evaluator.EvaluateAndStore("test-stage", gateResults, reviewResult)

	if err != nil {
		t.Errorf("EvaluateAndStore() error = %v", err)
	}

	if decision == nil {
		t.Fatal("EvaluateAndStore() returned nil decision")
	}
}

func TestEvaluator_StoreDecision_NoStore(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{})

	decision := NewLockDecision()
	decision.Decision = DecisionResolved

	err := evaluator.StoreDecision("test-stage", decision)
	if err == nil {
		t.Error("StoreDecision() expected error when artifact store is nil")
	}
}

func TestEvaluator_LoadDecision_NoStore(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{})

	_, err := evaluator.LoadDecision("test-stage")
	if err == nil {
		t.Error("LoadDecision() expected error when artifact store is nil")
	}
}

func TestEvaluator_LoadDecision_NotFound(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	evaluator := NewEvaluator(EvaluatorOptions{
		Config:        DefaultLockConfig(),
		ArtifactStore: store,
	})

	_, err := evaluator.LoadDecision("nonexistent-stage")
	if err == nil {
		t.Error("LoadDecision() expected error when decision not found")
	}
}

func TestEvaluator_DecisionExists_NoStore(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{})

	// Should return false without error
	exists := evaluator.DecisionExists("test-stage")
	if exists {
		t.Error("DecisionExists() should return false when no store")
	}
}

func TestEvaluator_LoadGateResultsFromStage_NoStore(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{})

	_, err := evaluator.LoadGateResultsFromStage("test-stage")
	if err == nil {
		t.Error("LoadGateResultsFromStage() expected error when artifact store is nil")
	}
}

func TestEvaluator_LoadGateResultsFromStage_WithResults(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	evaluator := NewEvaluator(EvaluatorOptions{
		Config:        DefaultLockConfig(),
		ArtifactStore: store,
	})

	// Store a gate report
	gateReport := map[string]interface{}{
		"schema_version": "codefoundry_gate_report.v1",
		"gate_id":        "test-gate",
		"status":         "pass",
	}
	store.WriteJSON("test-stage", "test-gate.json", gateReport)

	results, err := evaluator.LoadGateResultsFromStage("test-stage")
	if err != nil {
		t.Errorf("LoadGateResultsFromStage() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result, got %d", len(results))
	}

	if len(results) > 0 && results[0].GateID != "test-gate" {
		t.Errorf("GateID = %v, want test-gate", results[0].GateID)
	}
}

func TestEvaluator_LoadReviewResultFromStage_NoStore(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{})

	_, err := evaluator.LoadReviewResultFromStage("test-stage")
	if err == nil {
		t.Error("LoadReviewResultFromStage() expected error when artifact store is nil")
	}
}

func TestEvaluator_LoadReviewResultFromStage_NotFound(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	evaluator := NewEvaluator(EvaluatorOptions{
		Config:        DefaultLockConfig(),
		ArtifactStore: store,
	})

	_, err := evaluator.LoadReviewResultFromStage("nonexistent-stage")
	if err == nil {
		t.Error("LoadReviewResultFromStage() expected error when result not found")
	}
}

func TestEvaluator_EvaluateStage(t *testing.T) {
	store, _, cleanup := setupTestStore(t)
	defer cleanup()

	evaluator := NewEvaluator(EvaluatorOptions{
		Config:        DefaultLockConfig(),
		ArtifactStore: store,
	})

	// Store a gate report
	gateReport := map[string]interface{}{
		"schema_version": "codefoundry_gate_report.v1",
		"gate_id":        "test-gate",
		"status":         "pass",
	}
	store.WriteJSON("test-stage", "test-gate.json", gateReport)

	// Store a review result
	reviewResult := review.ReviewResult{
		SchemaVersion:       "codefoundry_review_result.v1",
		ConfidenceScore:     0.9,
		ConfidenceThreshold: 0.7,
		P1Count:             0,
		RubricScore:         80,
	}
	data, _ := reviewResult.ToJSON()
	store.Write("test-stage", "review-result.json", data)

	ctx := context.Background()
	decision, err := evaluator.EvaluateStage(ctx, "test-stage")

	if err != nil {
		t.Errorf("EvaluateStage() error = %v", err)
	}

	if decision == nil {
		t.Fatal("EvaluateStage() returned nil decision")
	}

	if decision.Decision != DecisionResolved {
		t.Errorf("Decision = %v, want %v", decision.Decision, DecisionResolved)
	}
}

func TestEvaluator_Reevaluate(t *testing.T) {
	evaluator := NewEvaluator(EvaluatorOptions{
		Config: DefaultLockConfig(),
	})

	originalDecision := NewLockDecision()
	originalDecision.Decision = DecisionReopen

	gateResults := []GateResult{
		{GateID: "test-gate", Status: "pass", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore: 0.9,
		P1Count:         0,
	}

	newDecision, err := evaluator.Reevaluate(originalDecision, gateResults, reviewResult)

	if err != nil {
		t.Errorf("Reevaluate() error = %v", err)
	}

	if newDecision == nil {
		t.Fatal("Reevaluate() returned nil decision")
	}

	if newDecision.Decision != DecisionResolved {
		t.Errorf("New decision = %v, want %v", newDecision.Decision, DecisionResolved)
	}
}

func TestBuildFromInputs_NilReviewResult(t *testing.T) {
	gateResults := []GateResult{
		{GateID: "gate1", Status: "pass", Required: true},
	}

	config := LockConfig{
		ConfidenceThreshold: 0.8,
	}

	decision := BuildFromInputs(gateResults, nil, config)

	if decision.ConfidenceThreshold != 0.8 {
		t.Errorf("ConfidenceThreshold = %v, want 0.8", decision.ConfidenceThreshold)
	}

	if decision.ConfidenceScore != 0 {
		t.Errorf("ConfidenceScore = %v, want 0", decision.ConfidenceScore)
	}

	if decision.P1Findings != 0 {
		t.Errorf("P1Findings = %v, want 0", decision.P1Findings)
	}
}

func TestBuildFromInputs_DefaultThreshold(t *testing.T) {
	gateResults := []GateResult{
		{GateID: "gate1", Status: "pass", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore:     0.9,
		ConfidenceThreshold: 0.6,
	}

	// Config with zero threshold should use review result threshold
	config := LockConfig{
		ConfidenceThreshold: 0, // Zero should use review result's threshold
	}

	decision := BuildFromInputs(gateResults, reviewResult, config)

	if decision.ConfidenceThreshold != 0.6 {
		t.Errorf("ConfidenceThreshold = %v, want 0.6", decision.ConfidenceThreshold)
	}
}

func TestBuildFromInputs_ReviewResultZeroThreshold(t *testing.T) {
	gateResults := []GateResult{
		{GateID: "gate1", Status: "pass", Required: true},
	}

	reviewResult := &review.ReviewResult{
		ConfidenceScore:     0.9,
		ConfidenceThreshold: 0,
	}

	config := LockConfig{
		ConfidenceThreshold: 0, // Both zero - behavior: uses reviewResult threshold (0)
	}

	decision := BuildFromInputs(gateResults, reviewResult, config)

	// When both are 0, it falls through to 0 (no default applied when reviewResult exists)
	if decision.ConfidenceThreshold != 0 {
		t.Errorf("ConfidenceThreshold = %v, want 0", decision.ConfidenceThreshold)
	}
}

func TestLockDecision_Validate(t *testing.T) {
	tests := []struct {
		name    string
		decision LockDecision
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid decision",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionResolved,
				Reason:              "All checks passed",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				P1Findings:          0,
				P2Findings:          1,
				P3Findings:          2,
				RubricScore:         80,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: false,
		},
		{
			name: "invalid schema version",
			decision: LockDecision{
				SchemaVersion:       "wrong.version",
				Decision:            DecisionResolved,
				Reason:              "All checks passed",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: true,
			errMsg:  "invalid schema version",
		},
		{
			name: "invalid decision",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            "unknown",
				Reason:              "All checks passed",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: true,
			errMsg:  "invalid decision",
		},
		{
			name: "empty reason",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionResolved,
				Reason:              "",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: true,
			errMsg:  "reason cannot be empty",
		},
		{
			name: "negative confidence score",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionResolved,
				Reason:              "All checks passed",
				ConfidenceScore:     -0.1,
				ConfidenceThreshold: 0.7,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: true,
			errMsg:  "invalid confidence score",
		},
		{
			name: "confidence score above 1.0",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionResolved,
				Reason:              "All checks passed",
				ConfidenceScore:     1.1,
				ConfidenceThreshold: 0.7,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: true,
			errMsg:  "invalid confidence score",
		},
		{
			name: "negative threshold",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionResolved,
				Reason:              "All checks passed",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: -0.1,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: true,
			errMsg:  "invalid confidence threshold",
		},
		{
			name: "negative P1 count",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionResolved,
				Reason:              "All checks passed",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				P1Findings:          -1,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: true,
			errMsg:  "finding counts cannot be negative",
		},
		{
			name: "negative rubric score",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionResolved,
				Reason:              "All checks passed",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				RubricScore:         -1,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: true,
			errMsg:  "invalid rubric score",
		},
		{
			name: "rubric score above 100",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionResolved,
				Reason:              "All checks passed",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				RubricScore:         101,
				Timestamp:           "2024-01-15T10:00:00Z",
			},
			wantErr: true,
			errMsg:  "invalid rubric score",
		},
		{
			name: "empty timestamp",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionResolved,
				Reason:              "All checks passed",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				Timestamp:           "",
			},
			wantErr: true,
			errMsg:  "timestamp cannot be empty",
		},
		{
			name: "escalation required without reason",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionReopen,
				Reason:              "P1 findings found",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				Timestamp:           "2024-01-15T10:00:00Z",
				EscalationRequired:  true,
				EscalationReason:    "",
			},
			wantErr: true,
			errMsg:  "escalation_reason required",
		},
		{
			name: "escalation required with reason - valid",
			decision: LockDecision{
				SchemaVersion:       "codefoundry_lock_decision.v1",
				Decision:            DecisionReopen,
				Reason:              "P1 findings found",
				ConfidenceScore:     0.9,
				ConfidenceThreshold: 0.7,
				Timestamp:           "2024-01-15T10:00:00Z",
				EscalationRequired:  true,
				EscalationReason:    "Critical findings must be fixed",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.decision.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Validate() error message = %v, should contain %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestGateResult_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		wantPass   bool
		wantFail   bool
	}{
		{
			name:     "empty status",
			status:   "",
			wantPass: false,
			wantFail: false,
		},
		{
			name:     "mixed case pass",
			status:   "Pass",
			wantPass: false,
			wantFail: false,
		},
		{
			name:     "mixed case fail",
			status:   "Fail",
			wantPass: false,
			wantFail: false,
		},
		{
			name:     "running status",
			status:   "running",
			wantPass: false,
			wantFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := GateResult{Status: tt.status}
			if got := g.IsPass(); got != tt.wantPass {
				t.Errorf("IsPass() = %v, want %v", got, tt.wantPass)
			}
			if got := g.IsFail(); got != tt.wantFail {
				t.Errorf("IsFail() = %v, want %v", got, tt.wantFail)
			}
		})
	}
}
