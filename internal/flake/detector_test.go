package flake

import (
	"math"
	"testing"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/replay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDetector(t *testing.T) {
	detector := NewDetector(nil, "/tmp", 0.95)

	assert.NotNil(t, detector)
	assert.Equal(t, 0.95, detector.threshold)
	assert.Equal(t, "/tmp", detector.basePath)
}

func TestNewDetector_DefaultThreshold(t *testing.T) {
	// Test with invalid threshold (0)
	detector := NewDetector(nil, "/tmp", 0)
	assert.Equal(t, 0.95, detector.threshold)

	// Test with invalid threshold (> 1)
	detector2 := NewDetector(nil, "/tmp", 1.5)
	assert.Equal(t, 0.95, detector2.threshold)
}

func TestAggregateDifferences(t *testing.T) {
	diffMap := map[string][]replay.Difference{
		"stage-1:status:status": {
			{StageID: "stage-1", Field: "status", Type: "status", Expected: "pass", Actual: "fail"},
			{StageID: "stage-1", Field: "status", Type: "status", Expected: "pass", Actual: "fail"},
			{StageID: "stage-1", Field: "status", Type: "status", Expected: "pass", Actual: "error"},
		},
		"stage-2:output:output": {
			{StageID: "stage-2", Field: "result", Type: "output", Expected: "success", Actual: "failure"},
		},
	}

	summaries := aggregateDifferences(diffMap)

	require.Len(t, summaries, 2)

	// Check stage-1 summary
	var stage1Summary *DiffSummary
	for i := range summaries {
		if summaries[i].StageID == "stage-1" {
			stage1Summary = &summaries[i]
			break
		}
	}

	require.NotNil(t, stage1Summary)
	assert.Equal(t, "status", stage1Summary.Field)
	assert.Equal(t, "status", stage1Summary.Type)
	assert.Equal(t, 3, stage1Summary.Count)
	assert.Equal(t, "pass", stage1Summary.Expected)
	assert.Len(t, stage1Summary.Variations, 2) // fail and error
	assert.Contains(t, stage1Summary.Variations, "fail")
	assert.Contains(t, stage1Summary.Variations, "error")
}

func TestCalculateConfidence(t *testing.T) {
	tests := []struct {
		name        string
		successRate float64
		n           int
		minExpected float64
		maxExpected float64
	}{
		{
			name:        "perfect success",
			successRate: 1.0,
			n:           10,
			minExpected: 0.8,
			maxExpected: 1.0,
		},
		{
			name:        "50% success",
			successRate: 0.5,
			n:           10,
			minExpected: 0.0,
			maxExpected: 0.8,
		},
		{
			name:        "zero samples",
			successRate: 0.5,
			n:           0,
			minExpected: 0.0,
			maxExpected: 0.0,
		},
		{
			name:        "large sample",
			successRate: 0.8,
			n:           100,
			minExpected: 0.7,
			maxExpected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := calculateConfidence(tt.successRate, tt.n)
			assert.GreaterOrEqual(t, confidence, tt.minExpected)
			assert.LessOrEqual(t, confidence, tt.maxExpected)
		})
	}
}

func TestCategorizeFlakeType(t *testing.T) {
	tests := []struct {
		name     string
		report   FlakeReport
		expected string
	}{
		{
			name: "consistent failure",
			report: FlakeReport{
				SuccessRate: 0,
			},
			expected: "consistent_failure",
		},
		{
			name: "consistent success",
			report: FlakeReport{
				SuccessRate: 1,
			},
			expected: "consistent_success",
		},
		{
			name: "mostly failing",
			report: FlakeReport{
				SuccessRate: 0.3,
			},
			expected: "mostly_failing",
		},
		{
			name: "timing flaky",
			report: FlakeReport{
				SuccessRate: 0.8,
				Differences: []DiffSummary{
					{Type: "timing"},
				},
			},
			expected: "timing_flaky",
		},
		{
			name: "output flaky",
			report: FlakeReport{
				SuccessRate: 0.8,
				Differences: []DiffSummary{
					{Type: "output"},
				},
			},
			expected: "output_flaky",
		},
		{
			name: "intermittent",
			report: FlakeReport{
				SuccessRate: 0.7,
				Differences: []DiffSummary{
					{Type: "status"},
				},
			},
			expected: "intermittent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := categorizeFlakeType(&tt.report)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateRecommendation(t *testing.T) {
	tests := []struct {
		name     string
		report   FlakeReport
		expected string
	}{
		{
			name: "acceptable success rate",
			report: FlakeReport{
				SuccessRate: 0.96,
				Threshold:   0.95,
			},
			expected: "No action needed - success rate is acceptable",
		},
		{
			name: "consistent failure",
			report: FlakeReport{
				SuccessRate: 0,
				Threshold:   0.95,
			},
			expected: "Run consistently fails - investigate and fix underlying issues",
		},
		{
			name: "mostly failing",
			report: FlakeReport{
				SuccessRate: 0.3,
				Threshold:   0.95,
			},
			expected: "Run is mostly failing - review recent changes and environment",
		},
		{
			name: "with differences",
			report: FlakeReport{
				SuccessRate: 0.8,
				Threshold:   0.95,
				Differences: []DiffSummary{
					{StageID: "stage-1", Field: "status", Type: "status"},
				},
			},
			expected: "Investigate status differences in stage stage-1 (field: status)",
		},
		{
			name: "generic flaky",
			report: FlakeReport{
				SuccessRate: 0.8,
				Threshold:   0.95,
				Differences: []DiffSummary{},
			},
			expected: "Review run for non-deterministic behavior",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateRecommendation(&tt.report)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFlakeReport_IsFlaky(t *testing.T) {
	tests := []struct {
		name      string
		successes int
		failures  int
		threshold float64
		wantFlaky bool
		wantRate  float64
	}{
		{
			name:      "not flaky - above threshold",
			successes: 5,
			failures:  0,
			threshold: 0.95,
			wantFlaky: false,
			wantRate:  1.0,
		},
		{
			name:      "flaky - below threshold",
			successes: 4,
			failures:  1,
			threshold: 0.95,
			wantFlaky: true,
			wantRate:  0.8,
		},
		{
			name:      "not flaky - at threshold",
			successes: 19,
			failures:  1,
			threshold: 0.95,
			wantFlaky: false,
			wantRate:  0.95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report := &FlakeReport{
				Successes:   tt.successes,
				Failures:    tt.failures,
				ReplayCount: tt.successes + tt.failures,
				Threshold:   tt.threshold,
			}
			report.SuccessRate = float64(tt.successes) / float64(report.ReplayCount)
			report.IsFlaky = report.SuccessRate < tt.threshold

			assert.Equal(t, tt.wantFlaky, report.IsFlaky)
			assert.InDelta(t, tt.wantRate, report.SuccessRate, 0.01)
		})
	}
}

func TestVarianceAnalysis(t *testing.T) {
	analysis := &VarianceAnalysis{
		RunID:       "test-run",
		ReplayCount: 10,
		SuccessRate: 0.8,
		Variance:    0.16,
		StdDev:      math.Sqrt(0.16),
		IsFlaky:     true,
		Confidence:  0.75,
	}

	assert.Equal(t, "test-run", analysis.RunID)
	assert.Equal(t, 10, analysis.ReplayCount)
	assert.Equal(t, 0.8, analysis.SuccessRate)
	assert.Equal(t, 0.16, analysis.Variance)
	assert.Equal(t, 0.4, analysis.StdDev)
	assert.True(t, analysis.IsFlaky)
	assert.Equal(t, 0.75, analysis.Confidence)
}

func TestReplaySummary(t *testing.T) {
	summary := &ReplaySummary{
		ReplayID:    "replay-123",
		Success:     true,
		Differences: 0,
		DurationMs:  1000,
	}

	assert.Equal(t, "replay-123", summary.ReplayID)
	assert.True(t, summary.Success)
	assert.Equal(t, 0, summary.Differences)
	assert.Equal(t, int64(1000), summary.DurationMs)
}

func TestDiffSummary(t *testing.T) {
	summary := DiffSummary{
		StageID:    "stage-1",
		Field:      "status",
		Type:       "status",
		Count:      3,
		Expected:   "pass",
		Variations: []string{"fail", "error"},
	}

	assert.Equal(t, "stage-1", summary.StageID)
	assert.Equal(t, "status", summary.Field)
	assert.Equal(t, "status", summary.Type)
	assert.Equal(t, 3, summary.Count)
	assert.Equal(t, "pass", summary.Expected)
	assert.Len(t, summary.Variations, 2)
}

func TestSaveAndLoadReport(t *testing.T) {
	tmp := t.TempDir()
	ns := artifact.NewNamespace(tmp, "run-1")
	store := artifact.NewStore(ns)

	report := &FlakeReport{RunID: "run-1", ReplayCount: 3, SuccessRate: 0.66}
	require.NoError(t, SaveReport(store, report))

	loaded, err := LoadReport(store)
	require.NoError(t, err)
	assert.Equal(t, report.RunID, loaded.RunID)
	assert.Equal(t, report.ReplayCount, loaded.ReplayCount)
	assert.InDelta(t, report.SuccessRate, loaded.SuccessRate, 0.0001)
}
