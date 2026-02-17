package review

import (
	"fmt"
	"math"
	"testing"
)

func TestCalculateRubricScore(t *testing.T) {
	tests := []struct {
		name     string
		dims     RubricDimensions
		expected int
	}{
		{
			name: "perfect score (all 5s)",
			dims: RubricDimensions{
				Correctness:     5,
				Efficiency:      5,
				Maintainability: 5,
				Safety:          5,
			},
			// Weighted avg = 5*0.4 + 5*0.2 + 5*0.2 + 5*0.2 = 2+1+1+1 = 5
			// Score = 5 * 20 = 100
			expected: 100,
		},
		{
			name: "minimum score (all 1s)",
			dims: RubricDimensions{
				Correctness:     1,
				Efficiency:      1,
				Maintainability: 1,
				Safety:          1,
			},
			// Weighted avg = 1*0.4 + 1*0.2 + 1*0.2 + 1*0.2 = 0.4+0.2+0.2+0.2 = 1
			// Score = 1 * 20 = 20
			expected: 20,
		},
		{
			name: "average score (all 3s)",
			dims: RubricDimensions{
				Correctness:     3,
				Efficiency:      3,
				Maintainability: 3,
				Safety:          3,
			},
			// Weighted avg = 3*0.4 + 3*0.2 + 3*0.2 + 3*0.2 = 1.2+0.6+0.6+0.6 = 3
			// Score = 3 * 20 = 60
			expected: 60,
		},
		{
			name: "correctness weighted higher",
			dims: RubricDimensions{
				Correctness:     5,
				Efficiency:      1,
				Maintainability: 1,
				Safety:          1,
			},
			// Weighted avg = 5*0.4 + 1*0.2 + 1*0.2 + 1*0.2 = 2+0.2+0.2+0.2 = 2.6
			// Score = 2.6 * 20 = 52
			expected: 52,
		},
		{
			name: "mixed scores",
			dims: RubricDimensions{
				Correctness:     4,
				Efficiency:      3,
				Maintainability: 4,
				Safety:          5,
			},
			// Weighted avg = 4*0.4 + 3*0.2 + 4*0.2 + 5*0.2 = 1.6+0.6+0.8+1.0 = 4.0
			// Score = 4.0 * 20 = 80
			expected: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateRubricScore(tt.dims)
			if got != tt.expected {
				t.Errorf("CalculateRubricScore() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestValidateDimensions(t *testing.T) {
	tests := []struct {
		name    string
		dims    RubricDimensions
		wantErr bool
	}{
		{
			name: "valid dimensions",
			dims: RubricDimensions{
				Correctness:     5,
				Efficiency:      4,
				Maintainability: 3,
				Safety:          2,
			},
			wantErr: false,
		},
		{
			name: "correctness too low",
			dims: RubricDimensions{
				Correctness:     0,
				Efficiency:      3,
				Maintainability: 3,
				Safety:          3,
			},
			wantErr: true,
		},
		{
			name: "correctness too high",
			dims: RubricDimensions{
				Correctness:     6,
				Efficiency:      3,
				Maintainability: 3,
				Safety:          3,
			},
			wantErr: true,
		},
		{
			name: "efficiency too low",
			dims: RubricDimensions{
				Correctness:     3,
				Efficiency:      0,
				Maintainability: 3,
				Safety:          3,
			},
			wantErr: true,
		},
		{
			name: "maintainability too high",
			dims: RubricDimensions{
				Correctness:     3,
				Efficiency:      3,
				Maintainability: 6,
				Safety:          3,
			},
			wantErr: true,
		},
		{
			name: "safety out of range",
			dims: RubricDimensions{
				Correctness:     3,
				Efficiency:      3,
				Maintainability: 3,
				Safety:          0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDimensions(tt.dims)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDimensions() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCalculateDimensionAverage(t *testing.T) {
	dims := RubricDimensions{
		Correctness:     5,
		Efficiency:      3,
		Maintainability: 4,
		Safety:          2,
	}

	expected := 3.5 // (5 + 3 + 4 + 2) / 4 = 3.5
	got := CalculateDimensionAverage(dims)

	if got != expected {
		t.Errorf("CalculateDimensionAverage() = %v, want %v", got, expected)
	}
}

func TestNormalizeScore(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.0, 0.0},  // minimum
		{3.0, 0.5},  // middle
		{5.0, 1.0},  // maximum
		{2.0, 0.25}, // quarter
		{4.0, 0.75}, // three quarters
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("score_%.1f", tt.input), func(t *testing.T) {
			got := NormalizeScore(tt.input)
			diff := math.Abs(got - tt.expected)
			if diff > 0.0001 {
				t.Errorf("NormalizeScore(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestCalculateRubricScore_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		dims     RubricDimensions
		expected int
	}{
		{
			name: "all zeros (invalid but testable)",
			dims: RubricDimensions{
				Correctness:     0,
				Efficiency:      0,
				Maintainability: 0,
				Safety:          0,
			},
			expected: 0,
		},
		{
			name: "negative values (invalid but testable)",
			dims: RubricDimensions{
				Correctness:     -1,
				Efficiency:      -2,
				Maintainability: -3,
				Safety:          -4,
			},
			// Weighted avg = -1*0.4 + -2*0.2 + -3*0.2 + -4*0.2 = -0.4 - 0.4 - 0.6 - 0.8 = -2.2
			// Score = -2.2 * 20 = -44
			expected: -44,
		},
		{
			name: "values above 5 (invalid but testable)",
			dims: RubricDimensions{
				Correctness:     10,
				Efficiency:      10,
				Maintainability: 10,
				Safety:          10,
			},
			expected: 200,
		},
		{
			name: "only correctness scored",
			dims: RubricDimensions{
				Correctness:     3,
				Efficiency:      1,
				Maintainability: 1,
				Safety:          1,
			},
			// Weighted avg = 3*0.4 + 1*0.2 + 1*0.2 + 1*0.2 = 1.2 + 0.2 + 0.2 + 0.2 = 1.8
			// Score = 1.8 * 20 = 36
			expected: 36,
		},
		{
			name: "only safety scored",
			dims: RubricDimensions{
				Correctness:     1,
				Efficiency:      1,
				Maintainability: 1,
				Safety:          5,
			},
			// Weighted avg = 1*0.4 + 1*0.2 + 1*0.2 + 5*0.2 = 0.4 + 0.2 + 0.2 + 1.0 = 1.8
			// Score = 1.8 * 20 = 36
			expected: 36,
		},
		{
			name: "mixed with decimals",
			dims: RubricDimensions{
				Correctness:     2,
				Efficiency:      4,
				Maintainability: 3,
				Safety:          5,
			},
			// Weighted avg = 2*0.4 + 4*0.2 + 3*0.2 + 5*0.2 = 0.8 + 0.8 + 0.6 + 1.0 = 3.2
			// Score = 3.2 * 20 = 64
			expected: 64,
		},
		{
			name: "boundary case - all 2s",
			dims: RubricDimensions{
				Correctness:     2,
				Efficiency:      2,
				Maintainability: 2,
				Safety:          2,
			},
			// Weighted avg = 2*0.4 + 2*0.2 + 2*0.2 + 2*0.2 = 0.8 + 0.4 + 0.4 + 0.4 = 2.0
			// Score = 2.0 * 20 = 40
			expected: 40,
		},
		{
			name: "boundary case - all 4s",
			dims: RubricDimensions{
				Correctness:     4,
				Efficiency:      4,
				Maintainability: 4,
				Safety:          4,
			},
			// Weighted avg = 4*0.4 + 4*0.2 + 4*0.2 + 4*0.2 = 1.6 + 0.8 + 0.8 + 0.8 = 4.0
			// Score = 4.0 * 20 = 80
			expected: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateRubricScore(tt.dims)
			if got != tt.expected {
				t.Errorf("CalculateRubricScore() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestValidateDimensions_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		dims    RubricDimensions
		wantErr bool
		errMsg  string
	}{
		{
			name: "all at lower boundary",
			dims: RubricDimensions{
				Correctness:     1,
				Efficiency:      1,
				Maintainability: 1,
				Safety:          1,
			},
			wantErr: false,
		},
		{
			name: "all at upper boundary",
			dims: RubricDimensions{
				Correctness:     5,
				Efficiency:      5,
				Maintainability: 5,
				Safety:          5,
			},
			wantErr: false,
		},
		{
			name: "one below lower boundary",
			dims: RubricDimensions{
				Correctness:     0,
				Efficiency:      3,
				Maintainability: 3,
				Safety:          3,
			},
			wantErr: true,
			errMsg:  "correctness",
		},
		{
			name: "one above upper boundary",
			dims: RubricDimensions{
				Correctness:     3,
				Efficiency:      3,
				Maintainability: 3,
				Safety:          6,
			},
			wantErr: true,
			errMsg:  "safety",
		},
		{
			name: "negative values",
			dims: RubricDimensions{
				Correctness:     -1,
				Efficiency:      -1,
				Maintainability: -1,
				Safety:          -1,
			},
			wantErr: true,
		},
		{
			name: "mixed valid and invalid",
			dims: RubricDimensions{
				Correctness:     3,
				Efficiency:      10,
				Maintainability: 3,
				Safety:          3,
			},
			wantErr: true,
			errMsg:  "efficiency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDimensions(tt.dims)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDimensions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				errStr := err.Error()
				if !containsSubstring(errStr, tt.errMsg) {
					t.Errorf("ValidateDimensions() error message = %v, should contain %v", errStr, tt.errMsg)
				}
			}
		})
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
