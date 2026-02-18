package review

import (
	"encoding/json"
	"fmt"
	"time"
)

// ReviewResult represents the output of a review stage
type ReviewResult struct {
	SchemaVersion       string           `json:"schema_version"`
	RubricScore         int              `json:"rubric_score"`         // 0-100
	ConfidenceScore     float64          `json:"confidence_score"`     // 0.0-1.0
	ConfidenceThreshold float64          `json:"confidence_threshold"` // 0.0-1.0
	Dimensions          RubricDimensions `json:"dimensions"`           // 1-5 each
	Findings            []Finding        `json:"findings"`             // All findings
	P1Count             int              `json:"p1_count"`             // Must fix
	P2Count             int              `json:"p2_count"`             // Should fix
	P3Count             int              `json:"p3_count"`             // Nice to fix
	Summary             string           `json:"summary"`              // Human summary
	Timestamp           time.Time        `json:"timestamp"`            // UTC
	StageID             string           `json:"stage_id,omitempty"`   // Source stage
	RunID               string           `json:"run_id,omitempty"`     // Source run
}

// Validate validates the review result
func (r *ReviewResult) Validate() error {
	// Validate schema version
	if r.SchemaVersion != "codefoundry_review_result.v1" {
		return fmt.Errorf("invalid schema version: %s", r.SchemaVersion)
	}

	// Validate rubric score
	if r.RubricScore < 0 || r.RubricScore > 100 {
		return fmt.Errorf("invalid rubric score: %d (must be 0-100)", r.RubricScore)
	}

	// Validate confidence score
	if r.ConfidenceScore < 0.0 || r.ConfidenceScore > 1.0 {
		return fmt.Errorf("invalid confidence score: %.2f (must be 0.0-1.0)", r.ConfidenceScore)
	}

	// Validate confidence threshold
	if r.ConfidenceThreshold < 0.0 || r.ConfidenceThreshold > 1.0 {
		return fmt.Errorf("invalid confidence threshold: %.2f (must be 0.0-1.0)", r.ConfidenceThreshold)
	}

	// Validate dimensions
	if err := ValidateDimensions(r.Dimensions); err != nil {
		return err
	}

	// Verify rubric score matches dimensions
	expectedScore := CalculateRubricScore(r.Dimensions)
	if r.RubricScore != expectedScore {
		return fmt.Errorf("rubric score %d doesn't match calculated score %d from dimensions",
			r.RubricScore, expectedScore)
	}

	// Validate finding counts
	p1, p2, p3 := CountFindingsBySeverity(r.Findings)
	if p1 != r.P1Count || p2 != r.P2Count || p3 != r.P3Count {
		return fmt.Errorf("finding counts don't match: P1=%d/%d, P2=%d/%d, P3=%d/%d",
			p1, r.P1Count, p2, r.P2Count, p3, r.P3Count)
	}

	// Validate each finding
	for i, f := range r.Findings {
		if f.ID == "" {
			return fmt.Errorf("finding %d has empty ID", i)
		}
		if f.Message == "" {
			return fmt.Errorf("finding %s has empty message", f.ID)
		}
		if !IsValidSeverity(string(f.Severity)) {
			return fmt.Errorf("finding %s has invalid severity: %s", f.ID, f.Severity)
		}
	}

	return nil
}

// IsPass returns true if the review passes all criteria
func (r *ReviewResult) IsPass(confidenceThreshold float64) bool {
	// Check confidence
	if r.ConfidenceScore < confidenceThreshold {
		return false
	}

	// Check for P1 findings
	if r.P1Count > 0 {
		return false
	}

	return true
}

// GetFindingsBySeverity returns findings filtered by severity
func (r *ReviewResult) GetFindingsBySeverity(severity Severity) []Finding {
	return FilterFindingsBySeverity(r.Findings, severity)
}

// AddFinding adds a finding and updates counts
func (r *ReviewResult) AddFinding(finding Finding) {
	r.Findings = append(r.Findings, finding)
	switch finding.Severity {
	case SeverityP1:
		r.P1Count++
	case SeverityP2:
		r.P2Count++
	case SeverityP3:
		r.P3Count++
	}
}

// SetCounts recalculates P1/P2/P3 counts from findings
func (r *ReviewResult) SetCounts() {
	r.P1Count, r.P2Count, r.P3Count = CountFindingsBySeverity(r.Findings)
}

// ToJSON serializes the result to JSON
func (r *ReviewResult) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// FromJSON deserializes a result from JSON
func FromJSON(data []byte) (*ReviewResult, error) {
	var result ReviewResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal review result: %w", err)
	}
	return &result, nil
}

// NewReviewResult creates a new review result with defaults
func NewReviewResult() *ReviewResult {
	return &ReviewResult{
		SchemaVersion:       "codefoundry_review_result.v1",
		ConfidenceThreshold: 0.7, // Default threshold
		Timestamp:           time.Now().UTC(),
		Findings:            []Finding{},
	}
}

// CalculateWeightedScore calculates overall score with custom weights
func (r *ReviewResult) CalculateWeightedScore(weights RubricWeights) float64 {
	score := float64(r.Dimensions.Correctness)*weights.Correctness +
		float64(r.Dimensions.Efficiency)*weights.Efficiency +
		float64(r.Dimensions.Maintainability)*weights.Maintainability +
		float64(r.Dimensions.Safety)*weights.Safety
	return score
}

// RubricWeights for weighted scoring
type RubricWeights struct {
	Correctness     float64
	Efficiency      float64
	Maintainability float64
	Safety          float64
}

// DefaultWeights returns the default rubric weights
func DefaultWeights() RubricWeights {
	return RubricWeights{
		Correctness:     0.4,
		Efficiency:      0.2,
		Maintainability: 0.2,
		Safety:          0.2,
	}
}

// Normalize normalizes the confidence score to a fixed range
func NormalizeConfidence(score float64) float64 {
	if score < 0.0 {
		return 0.0
	}
	if score > 1.0 {
		return 1.0
	}
	return score
}
