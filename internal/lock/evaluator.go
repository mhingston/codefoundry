package lock

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/review"
)

// LockConfig contains configuration for lock evaluation
type LockConfig struct {
	ConfidenceThreshold float64 // Threshold for auto-approval (0.0-1.0)
	AutoResolve         bool    // Whether to auto-resolve when conditions are met
}

// DefaultLockConfig returns default configuration
func DefaultLockConfig() LockConfig {
	return LockConfig{
		ConfidenceThreshold: 0.7,
		AutoResolve:         true,
	}
}

// Evaluator evaluates lock decisions
type Evaluator struct {
	config        LockConfig
	artifactStore *artifact.Store
}

// EvaluatorOptions configures the evaluator
type EvaluatorOptions struct {
	Config        LockConfig
	ArtifactStore *artifact.Store
}

// NewEvaluator creates a new lock evaluator
func NewEvaluator(opts EvaluatorOptions) *Evaluator {
	return &Evaluator{
		config:        opts.Config,
		artifactStore: opts.ArtifactStore,
	}
}

// WithArtifactStore sets the artifact store
func (e *Evaluator) WithArtifactStore(store *artifact.Store) *Evaluator {
	e.artifactStore = store
	return e
}

// Evaluate evaluates the lock decision based on gate results and review
func (e *Evaluator) Evaluate(
	gateResults []GateResult,
	reviewResult *review.ReviewResult,
) (*LockDecision, error) {
	decision := BuildFromInputs(gateResults, reviewResult, e.config)

	// Check 1: Required gates must pass (fail-closed)
	for _, gate := range gateResults {
		if gate.IsFail() && gate.Required {
			decision.Decision = DecisionReopen
			decision.Reason = fmt.Sprintf("Required gate '%s' failed", gate.GateID)
			return decision, nil
		}
	}

	// Check 2: Confidence threshold
	if reviewResult != nil {
		if reviewResult.ConfidenceScore < e.config.ConfidenceThreshold {
			decision.Decision = DecisionReopen
			decision.EscalationRequired = true
			decision.EscalationReason = fmt.Sprintf(
				"Confidence score %.2f is below threshold %.2f",
				reviewResult.ConfidenceScore,
				e.config.ConfidenceThreshold,
			)
			return decision, nil
		}
	}

	// Check 3: P1 findings (must fix)
	if reviewResult != nil && reviewResult.P1Count > 0 {
		decision.Decision = DecisionReopen
		decision.EscalationRequired = true
		decision.EscalationReason = fmt.Sprintf(
			"%d P1 finding(s) must be fixed before proceeding",
			reviewResult.P1Count,
		)
		return decision, nil
	}

	// All checks passed - resolve
	decision.Decision = DecisionResolved
	decision.Reason = "All required gates passed, confidence adequate, no P1 findings"
	decision.EscalationRequired = false

	return decision, nil
}

// EvaluateAndStore evaluates and stores the decision
func (e *Evaluator) EvaluateAndStore(
	stageID string,
	gateResults []GateResult,
	reviewResult *review.ReviewResult,
) (*LockDecision, error) {
	decision, err := e.Evaluate(gateResults, reviewResult)
	if err != nil {
		return nil, err
	}

	if e.artifactStore != nil {
		if err := e.StoreDecision(stageID, decision); err != nil {
			return nil, fmt.Errorf("failed to store lock decision: %w", err)
		}
	}

	return decision, nil
}

// StoreDecision stores the lock decision as an artifact
func (e *Evaluator) StoreDecision(stageID string, decision *LockDecision) error {
	if e.artifactStore == nil {
		return fmt.Errorf("artifact store not configured")
	}

	data, err := decision.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal lock decision: %w", err)
	}

	if err := e.artifactStore.Write(stageID, "lock-decision.json", data); err != nil {
		return fmt.Errorf("failed to store lock decision: %w", err)
	}

	return nil
}

// LoadDecision loads a lock decision from artifacts
func (e *Evaluator) LoadDecision(stageID string) (*LockDecision, error) {
	if e.artifactStore == nil {
		return nil, fmt.Errorf("artifact store not configured")
	}

	data, err := e.artifactStore.Read(stageID, "lock-decision.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read lock decision: %w", err)
	}

	return FromJSON(data)
}

// DecisionExists checks if a lock decision exists
func (e *Evaluator) DecisionExists(stageID string) bool {
	if e.artifactStore == nil {
		return false
	}
	return e.artifactStore.Exists(stageID, "lock-decision.json")
}

// LoadGateResultsFromStage loads gate results from a stage
func (e *Evaluator) LoadGateResultsFromStage(stageID string) ([]GateResult, error) {
	if e.artifactStore == nil {
		return nil, fmt.Errorf("artifact store not configured")
	}

	// List all artifacts in stage
	artifacts, err := e.artifactStore.List(stageID)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	var results []GateResult
	for _, artifact := range artifacts {
		// Skip non-JSON files and special files
		if filepath.Ext(artifact) != ".json" {
			continue
		}
		if artifact == "status.json" || 
		   artifact == "review-result.json" || 
		   artifact == "lock-decision.json" {
			continue
		}

		data, err := e.artifactStore.Read(stageID, artifact)
		if err != nil {
			continue
		}

		var gateResult map[string]interface{}
		if err := json.Unmarshal(data, &gateResult); err != nil {
			continue
		}

		// Check if it's a gate result
		if schema, ok := gateResult["schema_version"].(string); ok && 
		   schema == "codefoundry_gate_report.v1" {
			gateID, _ := gateResult["gate_id"].(string)
			status, _ := gateResult["status"].(string)
			
			results = append(results, GateResult{
				GateID:   gateID,
				Status:   status,
				Required: true, // Assume required if not specified
			})
		}
	}

	return results, nil
}

// LoadReviewResultFromStage loads review result from a stage
func (e *Evaluator) LoadReviewResultFromStage(stageID string) (*review.ReviewResult, error) {
	if e.artifactStore == nil {
		return nil, fmt.Errorf("artifact store not configured")
	}

	data, err := e.artifactStore.Read(stageID, "review-result.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read review result: %w", err)
	}

	return review.FromJSON(data)
}

// EvaluateStage evaluates a stage and returns the decision
func (e *Evaluator) EvaluateStage(ctx context.Context, stageID string) (*LockDecision, error) {
	// Load gate results
	gateResults, err := e.LoadGateResultsFromStage(stageID)
	if err != nil {
		return nil, fmt.Errorf("failed to load gate results: %w", err)
	}

	// Load review result (optional for lock stage)
	reviewResult, err := e.LoadReviewResultFromStage(stageID)
	if err != nil {
		// Review result may not exist - continue without it
		reviewResult = nil
	}

	return e.EvaluateAndStore(stageID, gateResults, reviewResult)
}

// Reevaluate reevaluates a decision with new parameters
func (e *Evaluator) Reevaluate(
	decision *LockDecision,
	gateResults []GateResult,
	reviewResult *review.ReviewResult,
) (*LockDecision, error) {
	return e.Evaluate(gateResults, reviewResult)
}
