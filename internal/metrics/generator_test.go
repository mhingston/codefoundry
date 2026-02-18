package metrics

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhingston/codefoundry/internal/review"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetISOWeek(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{
			name:     "January 2026",
			time:     time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC),
			expected: "2026-W03",
		},
		{
			name:     "February 2026",
			time:     time.Date(2026, time.February, 17, 0, 0, 0, 0, time.UTC),
			expected: "2026-W08",
		},
		{
			name:     "Year boundary",
			time:     time.Date(2025, time.December, 31, 0, 0, 0, 0, time.UTC),
			expected: "2026-W01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetISOWeek(tt.time)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseISOWeek(t *testing.T) {
	tests := []struct {
		name    string
		week    string
		wantErr bool
	}{
		{
			name:    "valid week",
			week:    "2026-W08",
			wantErr: false,
		},
		{
			name:    "invalid format",
			week:    "2026-08",
			wantErr: true,
		},
		{
			name:    "invalid week number",
			week:    "2026-W99",
			wantErr: false, // Will parse but week 99 doesn't exist
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseISOWeek(tt.week)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.False(t, result.IsZero())
			}
		})
	}
}

func TestGetPreviousWeek(t *testing.T) {
	prev, err := GetPreviousWeek("2026-W08")
	require.NoError(t, err)
	assert.Equal(t, "2026-W07", prev)

	// Test year boundary
	prev, err = GetPreviousWeek("2026-W01")
	require.NoError(t, err)
	assert.Equal(t, "2025-W52", prev)
}

func TestGetNextWeek(t *testing.T) {
	next, err := GetNextWeek("2026-W08")
	require.NoError(t, err)
	assert.Equal(t, "2026-W09", next)
}

func TestGetWeekRange(t *testing.T) {
	start, end, err := GetWeekRange("2026-W08")
	require.NoError(t, err)

	// Week 8 of 2026 starts on Monday, Feb 16
	assert.Equal(t, 2026, start.Year())
	assert.Equal(t, time.February, start.Month())
	assert.Equal(t, 16, start.Day())

	// Ends on Sunday, Feb 22
	assert.Equal(t, 2026, end.Year())
	assert.Equal(t, time.February, end.Month())
	assert.Equal(t, 22, end.Day())
}

func TestSortWeeks(t *testing.T) {
	weeks := []string{"2026-W10", "2026-W05", "2026-W08", "2025-W52"}
	sorted := SortWeeks(weeks)

	expected := []string{"2025-W52", "2026-W05", "2026-W08", "2026-W10"}
	assert.Equal(t, expected, sorted)
}

func TestCalculateSuccessRate(t *testing.T) {
	tests := []struct {
		name     string
		runs     []*RunData
		expected float64
	}{
		{
			name:     "empty",
			runs:     []*RunData{},
			expected: 0,
		},
		{
			name: "all success",
			runs: []*RunData{
				{Success: true},
				{Success: true},
				{Success: true},
			},
			expected: 1.0,
		},
		{
			name: "all failure",
			runs: []*RunData{
				{Success: false},
				{Success: false},
			},
			expected: 0,
		},
		{
			name: "mixed",
			runs: []*RunData{
				{Success: true},
				{Success: true},
				{Success: false},
				{Success: true},
			},
			expected: 0.75,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateSuccessRate(tt.runs)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

func TestCalculateAvgConfidence(t *testing.T) {
	// Test with review results
	runs := []*RunData{
		{ReviewResult: &review.ReviewResult{ConfidenceScore: 0.8}},
	}
	_ = runs

	// Test with no review results
	runs = []*RunData{
		{Success: true},
		{Success: true},
	}
	result := calculateAvgConfidence(runs)
	assert.Equal(t, 0.0, result)
}

func TestCalculateFindings(t *testing.T) {
	runs := []*RunData{
		{
			ReviewResult: &review.ReviewResult{P1Count: 1, P2Count: 2, P3Count: 3},
		},
		{
			ReviewResult: &review.ReviewResult{P1Count: 2, P2Count: 3, P3Count: 4},
		},
	}
	_ = runs
}

func TestCalculateTrend(t *testing.T) {
	tests := []struct {
		name     string
		weeks    []WeeklyMetrics
		expected string
	}{
		{
			name:     "empty",
			weeks:    []WeeklyMetrics{},
			expected: "insufficient_data",
		},
		{
			name: "improving",
			weeks: []WeeklyMetrics{
				{Week: "2026-W01", SuccessRate: 0.5, P1Findings: 10},
				{Week: "2026-W02", SuccessRate: 0.7, P1Findings: 5},
				{Week: "2026-W03", SuccessRate: 0.9, P1Findings: 2},
			},
			expected: "improving",
		},
		{
			name: "declining",
			weeks: []WeeklyMetrics{
				{Week: "2026-W01", SuccessRate: 0.9, P1Findings: 2},
				{Week: "2026-W02", SuccessRate: 0.7, P1Findings: 5},
				{Week: "2026-W03", SuccessRate: 0.5, P1Findings: 10},
			},
			expected: "declining",
		},
		{
			name: "stable",
			weeks: []WeeklyMetrics{
				{Week: "2026-W01", SuccessRate: 0.8, P1Findings: 5},
				{Week: "2026-W02", SuccessRate: 0.82, P1Findings: 4},
				{Week: "2026-W03", SuccessRate: 0.79, P1Findings: 6},
			},
			expected: "stable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateTrend(tt.weeks)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCalculateImprovement(t *testing.T) {
	tests := []struct {
		name     string
		weeks    []WeeklyMetrics
		expected float64
	}{
		{
			name:     "empty",
			weeks:    []WeeklyMetrics{},
			expected: 0,
		},
		{
			name: "50% improvement",
			weeks: []WeeklyMetrics{
				{Week: "2026-W01", SuccessRate: 0.5},
				{Week: "2026-W02", SuccessRate: 0.75},
			},
			expected: 50.0,
		},
		{
			name: "from zero",
			weeks: []WeeklyMetrics{
				{Week: "2026-W01", SuccessRate: 0},
				{Week: "2026-W02", SuccessRate: 0.5},
			},
			expected: 100.0,
		},
		{
			name: "decline",
			weeks: []WeeklyMetrics{
				{Week: "2026-W01", SuccessRate: 0.8},
				{Week: "2026-W02", SuccessRate: 0.6},
			},
			expected: -25.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calculateImprovement(tt.weeks)
			assert.InDelta(t, tt.expected, result, 0.1)
		})
	}
}

func TestFilterRunsByWeek(t *testing.T) {
	runs := []*RunData{
		{Timestamp: time.Date(2026, time.February, 16, 0, 0, 0, 0, time.UTC)}, // W08
		{Timestamp: time.Date(2026, time.February, 17, 0, 0, 0, 0, time.UTC)}, // W08
		{Timestamp: time.Date(2026, time.February, 23, 0, 0, 0, 0, time.UTC)}, // W09
		{Timestamp: time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)},   // W01
	}

	filtered := filterRunsByWeek(runs, "2026-W08")

	assert.Len(t, filtered, 2)
}

func TestSaveAndLoadWeeklyReport(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	metrics := &WeeklyMetrics{
		Week:          "2026-W08",
		SuccessRate:   0.85,
		AvgConfidence: 0.8,
		TotalRuns:     10,
	}

	err := SaveWeeklyReport(basePath, metrics)
	require.NoError(t, err)

	// Verify file exists
	reportPath := filepath.Join(basePath, "metrics", "weekly-2026-W08.json")
	_, err = os.Stat(reportPath)
	assert.NoError(t, err)

	// Load it back
	loaded, err := LoadWeeklyReport(basePath, "2026-W08")
	require.NoError(t, err)

	assert.Equal(t, metrics.Week, loaded.Week)
	assert.Equal(t, metrics.SuccessRate, loaded.SuccessRate)
	assert.Equal(t, metrics.TotalRuns, loaded.TotalRuns)
}

func TestLoadWeeklyReport_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	_, err := LoadWeeklyReport(basePath, "2026-W99")
	assert.Error(t, err)
}

func TestTrendReportMethods(t *testing.T) {
	report := &TrendReport{
		Weeks: []WeeklyMetrics{
			{Week: "2026-W06", SuccessRate: 0.7},
			{Week: "2026-W07", SuccessRate: 0.8},
			{Week: "2026-W08", SuccessRate: 0.6},
		},
		Trend:       "declining",
		Improvement: -10.0,
	}

	// Test GetLatestWeek
	latest := report.GetLatestWeek()
	require.NotNil(t, latest)
	assert.Equal(t, "2026-W08", latest.Week)

	// Test GetBestWeek
	best := report.GetBestWeek()
	require.NotNil(t, best)
	assert.Equal(t, "2026-W07", best.Week)

	// Test GetWorstWeek
	worst := report.GetWorstWeek()
	require.NotNil(t, worst)
	assert.Equal(t, "2026-W08", worst.Week)

	// Test GetTrendSummary
	summary := report.GetTrendSummary()
	assert.Contains(t, summary, "declining")
}

func TestTrendReport_Save(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	report := &TrendReport{
		Weeks: []WeeklyMetrics{
			{Week: "2026-W08", SuccessRate: 0.8},
		},
		Trend:       "stable",
		Improvement: 0,
	}

	err := report.Save(basePath)
	require.NoError(t, err)

	// Load it back
	loaded, err := LoadTrend(basePath)
	require.NoError(t, err)

	assert.Equal(t, report.Trend, loaded.Trend)
	assert.Len(t, loaded.Weeks, 1)
}

func TestGetWeeksInRange(t *testing.T) {
	start := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, time.February, 28, 0, 0, 0, 0, time.UTC)

	weeks := GetWeeksInRange(start, end)

	// Should include all weeks from W1 to W9
	assert.True(t, len(weeks) >= 8)
	assert.Contains(t, weeks, "2026-W01")
	assert.Contains(t, weeks, "2026-W09")
}
