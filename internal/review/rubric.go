package review

// RubricDimensions stores scores for each dimension (1-5 scale)
type RubricDimensions struct {
	Correctness     int `json:"correctness"`     // 1-5
	Efficiency      int `json:"efficiency"`      // 1-5
	Maintainability int `json:"maintainability"` // 1-5
	Safety          int `json:"safety"`          // 1-5
}

// CalculateRubricScore calculates weighted average (0-100 scale)
// Weights: Correctness=0.4, Efficiency=0.2, Maintainability=0.2, Safety=0.2
func CalculateRubricScore(dims RubricDimensions) int {
	// Calculate weighted average (each dimension is 1-5)
	// Then scale to 0-100 range:
	// weighted_avg ranges from 1 (all 1s) to 5 (all 5s)
	// We want: 1 -> 20, 5 -> 100
	// Formula: (weighted_avg - 1) / 4 * 80 + 20 = weighted_avg * 20
	correctnessScore := float64(dims.Correctness) * 0.4
	efficiencyScore := float64(dims.Efficiency) * 0.2
	maintainabilityScore := float64(dims.Maintainability) * 0.2
	safetyScore := float64(dims.Safety) * 0.2

	weightedAvg := correctnessScore + efficiencyScore + maintainabilityScore + safetyScore
	total := weightedAvg * 20
	return int(total)
}

// ValidateDimensions validates that all dimensions are in range 1-5
func ValidateDimensions(dims RubricDimensions) error {
	if dims.Correctness < 1 || dims.Correctness > 5 {
		return NewValidationError("correctness", dims.Correctness)
	}
	if dims.Efficiency < 1 || dims.Efficiency > 5 {
		return NewValidationError("efficiency", dims.Efficiency)
	}
	if dims.Maintainability < 1 || dims.Maintainability > 5 {
		return NewValidationError("maintainability", dims.Maintainability)
	}
	if dims.Safety < 1 || dims.Safety > 5 {
		return NewValidationError("safety", dims.Safety)
	}
	return nil
}

// CalculateDimensionAverage calculates unweighted average
func CalculateDimensionAverage(dims RubricDimensions) float64 {
	sum := dims.Correctness + dims.Efficiency + dims.Maintainability + dims.Safety
	return float64(sum) / 4.0
}

// NormalizeScore converts 1-5 scale to 0-1 scale
func NormalizeScore(score float64) float64 {
	return (score - 1) / 4
}
