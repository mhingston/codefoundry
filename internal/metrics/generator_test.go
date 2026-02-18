package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/review"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xeipuuv/gojsonschema"
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
	runs := []*RunData{
		{ReviewResult: &review.ReviewResult{ConfidenceScore: 0.8}},
		{ReviewResult: &review.ReviewResult{ConfidenceScore: 0.6}},
	}
	assert.InDelta(t, 0.7, calculateAvgConfidence(runs), 0.001)

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
	p1, p2, p3 := calculateFindings(runs)
	assert.Equal(t, 3, p1)
	assert.Equal(t, 5, p2)
	assert.Equal(t, 7, p3)
}

func TestCalculateAvgCycleTime(t *testing.T) {
	runs := []*RunData{
		{CycleTime: 3 * time.Minute},
		{CycleTime: 9 * time.Minute},
		{CycleTime: 0}, // ignored
	}

	assert.Equal(t, 6*time.Minute, calculateAvgCycleTime(runs))
}

func TestCalculateReplayPassRate(t *testing.T) {
	runs := []*RunData{
		{HasReplay: true, ReplayPassed: true},
		{HasReplay: true, ReplayPassed: false},
		{HasReplay: false},
	}

	rate, successes, samples := calculateReplayPassRate(runs)
	assert.InDelta(t, 0.5, rate, 0.001)
	assert.Equal(t, 1, successes)
	assert.Equal(t, 2, samples)
}

func TestCalculateBinomialConfidence(t *testing.T) {
	band := calculateBinomialConfidence(8, 10, 0.95)
	assert.Equal(t, 8, band.Successes)
	assert.Equal(t, 10, band.Samples)
	assert.Equal(t, 0.95, band.Level)
	assert.True(t, band.Lower > 0.4)
	assert.True(t, band.Upper < 1.0)
	assert.True(t, band.Lower < band.Upper)
}

func TestGenerateWeekly_ArtifactBackedMetrics(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	createMetricsTestRun(t, basePath, "run-1", true, true,
		time.Date(2026, time.February, 16, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 16, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 16, 10, 5, 0, 0, time.UTC),
		time.Date(2026, time.February, 16, 10, 6, 0, 0, time.UTC),
		time.Date(2026, time.February, 16, 10, 12, 0, 0, time.UTC),
	)

	createMetricsTestRun(t, basePath, "run-2", false, false,
		time.Date(2026, time.February, 17, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 17, 10, 0, 0, 0, time.UTC),
		time.Date(2026, time.February, 17, 10, 10, 0, 0, time.UTC),
		time.Date(2026, time.February, 17, 10, 11, 0, 0, time.UTC),
		time.Date(2026, time.February, 17, 10, 23, 0, 0, time.UTC),
	)

	gen := NewGenerator(nil, basePath)
	weekly, err := gen.GenerateWeekly("2026-W08")
	require.NoError(t, err)

	assert.Equal(t, 2, weekly.TotalRuns)
	assert.Equal(t, 1, weekly.RunsCompleted)
	assert.Equal(t, 1, weekly.RunsFailed)
	assert.Equal(t, 17*time.Minute+30*time.Second, weekly.AvgCycleTime)
	assert.InDelta(t, 0.5, weekly.ReplayPassRate, 0.001)
	assert.Equal(t, 1, weekly.SuccessRateConfidence.Successes)
	assert.Equal(t, 2, weekly.SuccessRateConfidence.Samples)
	assert.Equal(t, 1, weekly.DeterminismConfidence.Successes)
	assert.Equal(t, 2, weekly.DeterminismConfidence.Samples)
	assert.InDelta(t, 0.95, weekly.SuccessRateConfidence.Level, 0.0001)
}

func TestWeeklyMetricsJSONSchema(t *testing.T) {
	metrics := WeeklyMetrics{
		Week:           "2026-W08",
		SuccessRate:    0.5,
		AvgConfidence:  0.75,
		AvgRubricScore: 80,
		AvgCycleTime:   2 * time.Minute,
		P1Findings:     1,
		P2Findings:     2,
		P3Findings:     3,
		GatePassRate:   0.75,
		ReplayPassRate: 0.66,
		SuccessRateConfidence: BinomialConfidence{
			Successes: 1,
			Samples:   2,
			Lower:     0.10,
			Upper:     0.90,
			Level:     0.95,
		},
		DeterminismConfidence: BinomialConfidence{
			Successes: 2,
			Samples:   3,
			Lower:     0.20,
			Upper:     0.95,
			Level:     0.95,
		},
		RunsCompleted: 1,
		RunsFailed:    1,
		TotalRuns:     2,
	}

	payload, err := json.Marshal(metrics)
	require.NoError(t, err)

	schemaPath, err := filepath.Abs(filepath.Join("..", "..", "schemas", "weekly-metrics.schema.json"))
	require.NoError(t, err)

	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaPath)
	docLoader := gojsonschema.NewBytesLoader(payload)
	result, err := gojsonschema.Validate(schemaLoader, docLoader)
	require.NoError(t, err)

	if !result.Valid() {
		errs := make([]string, 0, len(result.Errors()))
		for _, e := range result.Errors() {
			errs = append(errs, e.String())
		}
		t.Fatalf("metrics payload failed schema: %v", errs)
	}
}

func createMetricsTestRun(
	t *testing.T,
	basePath, runID string,
	resolved bool,
	replayPass bool,
	timestamp time.Time,
	stage1Start, stage1End, stage2Start, stage2End time.Time,
) {
	t.Helper()

	ns := artifact.NewNamespace(basePath, runID)
	store := artifact.NewStore(ns)

	reviewResult := map[string]interface{}{
		"schema_version":       "codefoundry_review_result.v1",
		"rubric_score":         80,
		"confidence_score":     0.75,
		"confidence_threshold": 0.70,
		"dimensions":           map[string]int{"correctness": 4, "efficiency": 4, "maintainability": 4, "safety": 4},
		"findings":             []interface{}{},
		"p1_count":             0,
		"p2_count":             0,
		"p3_count":             0,
		"summary":              "ok",
		"timestamp":            timestamp.Format(time.RFC3339),
	}
	require.NoError(t, store.WriteJSON("review", "review-result.json", reviewResult))

	decision := "reopen"
	if resolved {
		decision = "resolved"
	}
	lockDecision := map[string]interface{}{
		"schema_version":       "codefoundry_lock_decision.v1",
		"decision":             decision,
		"reason":               "test",
		"required_gate_ids":    []string{"unit"},
		"passed_gate_ids":      []string{"unit"},
		"failed_gate_ids":      []string{},
		"confidence_score":     0.75,
		"confidence_threshold": 0.70,
		"p1_findings":          0,
		"p2_findings":          0,
		"p3_findings":          0,
		"rubric_score":         80,
		"escalation_required":  false,
		"timestamp":            timestamp.Format(time.RFC3339),
	}
	require.NoError(t, store.WriteJSON("lock", "lock-decision.json", lockDecision))

	trace := map[string]interface{}{
		"timestamp": timestamp.Format(time.RFC3339),
	}
	require.NoError(t, store.WriteJSON("_trace", "execution-trace.json", trace))

	require.NoError(t, store.WriteJSON("spec", "status.json", map[string]interface{}{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       "spec",
		"status":         "pass",
		"started_at":     stage1Start.Format(time.RFC3339),
		"completed_at":   stage1End.Format(time.RFC3339),
	}))
	require.NoError(t, store.WriteJSON("build", "status.json", map[string]interface{}{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       "build",
		"status":         "pass",
		"started_at":     stage2Start.Format(time.RFC3339),
		"completed_at":   stage2End.Format(time.RFC3339),
	}))

	replayResult := map[string]interface{}{
		"original_run_id": runID,
		"replay_run_id":   fmt.Sprintf("replay-%s", runID),
		"matches":         replayPass,
		"differences":     []interface{}{},
		"duration_ms":     100,
		"replay_count":    1,
	}
	require.NoError(t, store.WriteJSON("_replay", "replay-result.json", replayResult))
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
