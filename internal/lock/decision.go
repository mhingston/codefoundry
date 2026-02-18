package lock

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mhingston/codefoundry/internal/review"
)

// LockDecision represents the final lock stage decision
type LockDecision struct {
	SchemaVersion       string                 `json:"schema_version"`
	Decision            string                 `json:"decision"` // resolved|reopen
	Reason              string                 `json:"reason"`
	RequiredGateIDs     []string               `json:"required_gate_ids"`
	PassedGateIDs       []string               `json:"passed_gate_ids"`
	FailedGateIDs       []string               `json:"failed_gate_ids"`
	ConfidenceScore     float64                `json:"confidence_score"`
	ConfidenceThreshold float64                `json:"confidence_threshold"`
	P1Findings          int                    `json:"p1_findings"`
	P2Findings          int                    `json:"p2_findings"`
	P3Findings          int                    `json:"p3_findings"`
	RubricScore         int                    `json:"rubric_score"`
	EscalationRequired  bool                   `json:"escalation_required"`
	EscalationReason    string                 `json:"escalation_reason,omitempty"`
	Timestamp           string                 `json:"timestamp"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

// Decision values
const (
	DecisionResolved = "resolved"
	DecisionReopen   = "reopen"
)

// IsResolved returns true if the decision is resolved
func (d *LockDecision) IsResolved() bool {
	return d.Decision == DecisionResolved
}

// IsReopen returns true if the decision is reopen
func (d *LockDecision) IsReopen() bool {
	return d.Decision == DecisionReopen
}

// RequiresEscalation returns true if escalation is required
func (d *LockDecision) RequiresEscalation() bool {
	return d.EscalationRequired
}

// ToJSON serializes the decision to JSON
func (d *LockDecision) ToJSON() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

// FromJSON deserializes a decision from JSON
func FromJSON(data []byte) (*LockDecision, error) {
	var decision LockDecision
	if err := json.Unmarshal(data, &decision); err != nil {
		return nil, fmt.Errorf("failed to unmarshal lock decision: %w", err)
	}
	return &decision, nil
}

// Validate validates the lock decision
func (d *LockDecision) Validate() error {
	// Validate schema version
	if d.SchemaVersion != "codefoundry_lock_decision.v1" {
		return fmt.Errorf("invalid schema version: %s", d.SchemaVersion)
	}

	// Validate decision
	if d.Decision != DecisionResolved && d.Decision != DecisionReopen {
		return fmt.Errorf("invalid decision: %s", d.Decision)
	}

	// Validate reason
	if d.Reason == "" {
		return fmt.Errorf("reason cannot be empty")
	}

	// Validate confidence scores
	if d.ConfidenceScore < 0.0 || d.ConfidenceScore > 1.0 {
		return fmt.Errorf("invalid confidence score: %.2f", d.ConfidenceScore)
	}

	if d.ConfidenceThreshold < 0.0 || d.ConfidenceThreshold > 1.0 {
		return fmt.Errorf("invalid confidence threshold: %.2f", d.ConfidenceThreshold)
	}

	// Validate counts
	if d.P1Findings < 0 || d.P2Findings < 0 || d.P3Findings < 0 {
		return fmt.Errorf("finding counts cannot be negative")
	}

	// Validate rubric score
	if d.RubricScore < 0 || d.RubricScore > 100 {
		return fmt.Errorf("invalid rubric score: %d", d.RubricScore)
	}

	// Validate timestamp
	if d.Timestamp == "" {
		return fmt.Errorf("timestamp cannot be empty")
	}

	// If escalation required, must have reason
	if d.EscalationRequired && d.EscalationReason == "" {
		return fmt.Errorf("escalation_reason required when escalation_required is true")
	}

	return nil
}

// GateResult represents a gate result for evaluation
type GateResult struct {
	GateID   string
	Status   string
	Required bool
}

// IsPass returns true if gate passed
func (g GateResult) IsPass() bool {
	return g.Status == "pass"
}

// IsFail returns true if gate failed
func (g GateResult) IsFail() bool {
	return g.Status == "fail"
}

// NewLockDecision creates a new lock decision with defaults
func NewLockDecision() *LockDecision {
	return &LockDecision{
		SchemaVersion: "codefoundry_lock_decision.v1",
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Metadata:      make(map[string]interface{}),
	}
}

// BuildFromInputs builds a decision from gate results and review result
func BuildFromInputs(
	gateResults []GateResult,
	reviewResult *review.ReviewResult,
	config LockConfig,
) *LockDecision {
	decision := NewLockDecision()

	// Set gate information
	decision.RequiredGateIDs = getRequiredGateIDs(gateResults)
	decision.PassedGateIDs = getPassedGateIDs(gateResults)
	decision.FailedGateIDs = getFailedGateIDs(gateResults)

	// Set review information
	if reviewResult != nil {
		decision.ConfidenceScore = reviewResult.ConfidenceScore
		decision.ConfidenceThreshold = config.ConfidenceThreshold
		if decision.ConfidenceThreshold == 0 {
			decision.ConfidenceThreshold = reviewResult.ConfidenceThreshold
		}
		decision.P1Findings = reviewResult.P1Count
		decision.P2Findings = reviewResult.P2Count
		decision.P3Findings = reviewResult.P3Count
		decision.RubricScore = reviewResult.RubricScore
	} else {
		decision.ConfidenceThreshold = config.ConfidenceThreshold
		if decision.ConfidenceThreshold == 0 {
			decision.ConfidenceThreshold = 0.7 // Default
		}
	}

	return decision
}

func getRequiredGateIDs(results []GateResult) []string {
	var ids []string
	for _, r := range results {
		if r.Required {
			ids = append(ids, r.GateID)
		}
	}
	return ids
}

func getPassedGateIDs(results []GateResult) []string {
	var ids []string
	for _, r := range results {
		if r.IsPass() {
			ids = append(ids, r.GateID)
		}
	}
	return ids
}

func getFailedGateIDs(results []GateResult) []string {
	var ids []string
	for _, r := range results {
		if r.IsFail() {
			ids = append(ids, r.GateID)
		}
	}
	return ids
}
