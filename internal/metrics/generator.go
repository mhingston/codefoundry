package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhingston/codefoundry/internal/artifact"
)

// Generator generates metrics reports
type Generator struct {
	artifactStore *artifact.Store
	basePath      string
}

// NewGenerator creates a new metrics generator
func NewGenerator(artifactStore *artifact.Store, basePath string) *Generator {
	return &Generator{
		artifactStore: artifactStore,
		basePath:      basePath,
	}
}

// GenerateWeekly generates metrics for a specific week
func (g *Generator) GenerateWeekly(week string) (*WeeklyMetrics, error) {
	// Parse week
	start, end, err := GetWeekRange(week)
	if err != nil {
		return nil, fmt.Errorf("invalid week format: %w", err)
	}

	// Discover all runs
	runs, err := g.discoverRuns()
	if err != nil {
		return nil, fmt.Errorf("failed to discover runs: %w", err)
	}

	// Filter runs by week
	weekRuns := make([]*RunData, 0)
	for _, run := range runs {
		if !run.Timestamp.IsZero() &&
			(run.Timestamp.Equal(start) || run.Timestamp.After(start)) &&
			(run.Timestamp.Equal(end) || run.Timestamp.Before(end)) {
			weekRuns = append(weekRuns, run)
		}
	}

	// Generate metrics
	metrics := &WeeklyMetrics{
		Week:           week,
		SuccessRate:    calculateSuccessRate(weekRuns),
		AvgConfidence:  calculateAvgConfidence(weekRuns),
		AvgRubricScore: calculateAvgRubricScore(weekRuns),
		GatePassRate:   calculateGatePassRate(weekRuns),
		TotalRuns:      len(weekRuns),
	}

	// Calculate runs completed/failed
	for _, run := range weekRuns {
		if run.Success {
			metrics.RunsCompleted++
		} else {
			metrics.RunsFailed++
		}
	}

	// Calculate findings
	metrics.P1Findings, metrics.P2Findings, metrics.P3Findings = calculateFindings(weekRuns)

	// Calculate artifact-backed aggregate metrics.
	metrics.AvgCycleTime = calculateAvgCycleTime(weekRuns)
	replayPassRate, replaySuccesses, replaySamples := calculateReplayPassRate(weekRuns)
	metrics.ReplayPassRate = replayPassRate

	metrics.SuccessRateConfidence = calculateBinomialConfidence(metrics.RunsCompleted, metrics.TotalRuns, 0.95)
	metrics.DeterminismConfidence = calculateBinomialConfidence(
		replaySuccesses,
		replaySamples,
		0.95,
	)

	return metrics, nil
}

// GenerateTrend generates a trend report for the last N weeks
func (g *Generator) GenerateTrend(lastNWeeks int) (*TrendReport, error) {
	if lastNWeeks <= 0 {
		lastNWeeks = 4 // Default
	}

	// Get available weeks
	availableWeeks, err := g.GetAvailableWeeks()
	if err != nil {
		return nil, fmt.Errorf("failed to get available weeks: %w", err)
	}

	if len(availableWeeks) == 0 {
		// Try to generate from current week backwards
		availableWeeks = make([]string, lastNWeeks)
		currentWeek := GetCurrentWeek()
		availableWeeks[0] = currentWeek
		for i := 1; i < lastNWeeks; i++ {
			prevWeek, _ := GetPreviousWeek(availableWeeks[i-1])
			availableWeeks[i] = prevWeek
		}
		// Reverse to chronological order
		for i, j := 0, len(availableWeeks)-1; i < j; i, j = i+1, j-1 {
			availableWeeks[i], availableWeeks[j] = availableWeeks[j], availableWeeks[i]
		}
	}

	// Take last N weeks
	if len(availableWeeks) > lastNWeeks {
		availableWeeks = availableWeeks[len(availableWeeks)-lastNWeeks:]
	}

	// Sort weeks
	availableWeeks = SortWeeks(availableWeeks)

	// Generate metrics for each week
	weeks := make([]WeeklyMetrics, 0, len(availableWeeks))
	for _, week := range availableWeeks {
		// Try to load existing report first
		metrics, err := LoadWeeklyReport(g.basePath, week)
		if err != nil {
			// Generate new report
			metrics, err = g.GenerateWeekly(week)
			if err != nil {
				continue // Skip this week
			}
		}
		weeks = append(weeks, *metrics)
	}

	// Calculate trend
	trend := &TrendReport{
		Weeks: weeks,
	}

	if len(weeks) >= 2 {
		trend.Trend = calculateTrend(weeks)
		trend.Improvement = calculateImprovement(weeks)
	} else if len(weeks) == 1 {
		trend.Trend = "insufficient_data"
		trend.Improvement = 0
	} else {
		trend.Trend = "no_data"
		trend.Improvement = 0
	}

	return trend, nil
}

// GetAvailableWeeks returns a list of weeks with available metrics
func (g *Generator) GetAvailableWeeks() ([]string, error) {
	weeks := make([]string, 0)

	// Check metrics directory
	metricsPath := filepath.Join(g.basePath, "metrics")
	entries, err := os.ReadDir(metricsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return weeks, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasPrefix(name, "weekly-") || !strings.HasSuffix(name, ".json") {
			continue
		}

		// Extract week from filename (weekly-YYYY-WXX.json)
		week := name[7 : len(name)-5] // Remove "weekly-" prefix and ".json" suffix
		weeks = append(weeks, week)
	}

	return SortWeeks(weeks), nil
}

// discoverRuns discovers all runs in the artifact store
func (g *Generator) discoverRuns() ([]*RunData, error) {
	runs := make([]*RunData, 0)

	// Read all run directories
	artifactsPath := filepath.Join(g.basePath, "artifacts")
	entries, err := os.ReadDir(artifactsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return runs, nil
		}
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runID := entry.Name()
		ns := artifact.NewNamespace(g.basePath, runID)
		store := artifact.NewStore(ns)

		data, err := ExtractRunData(runID, store)
		if err != nil {
			continue // Skip runs we can't read
		}

		runs = append(runs, data)
	}

	return runs, nil
}

// TrendReport represents a trend analysis across multiple weeks
type TrendReport struct {
	Weeks       []WeeklyMetrics `json:"weeks"`
	Trend       string          `json:"trend"`       // improving, stable, declining, no_data
	Improvement float64         `json:"improvement"` // Percentage change
}

// calculateTrend determines the overall trend
func calculateTrend(weeks []WeeklyMetrics) string {
	if len(weeks) < 2 {
		return "insufficient_data"
	}

	// Compare first and last week
	first := weeks[0]
	last := weeks[len(weeks)-1]

	// Calculate change in success rate
	successChange := last.SuccessRate - first.SuccessRate

	// Calculate change in findings (fewer is better)
	firstFindings := first.P1Findings + first.P2Findings + first.P3Findings
	lastFindings := last.P1Findings + last.P2Findings + last.P3Findings
	findingsChange := float64(firstFindings - lastFindings)

	// Determine trend
	score := successChange + (findingsChange / 100) // Weight findings less

	switch {
	case score > 0.1:
		return "improving"
	case score < -0.1:
		return "declining"
	default:
		return "stable"
	}
}

// calculateImprovement calculates the percentage improvement
func calculateImprovement(weeks []WeeklyMetrics) float64 {
	if len(weeks) < 2 {
		return 0
	}

	first := weeks[0]
	last := weeks[len(weeks)-1]

	// Calculate improvement based on success rate
	if first.SuccessRate == 0 {
		if last.SuccessRate > 0 {
			return 100 // From 0 to something is 100% improvement
		}
		return 0
	}

	improvement := ((last.SuccessRate - first.SuccessRate) / first.SuccessRate) * 100
	return improvement
}

// Save saves the trend report
func (t *TrendReport) Save(basePath string) error {
	reportPath := filepath.Join(basePath, "metrics", "trend.json")

	// Ensure directory exists
	dir := filepath.Dir(reportPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create metrics directory: %w", err)
	}

	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trend report: %w", err)
	}

	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write trend report: %w", err)
	}

	return nil
}

// LoadTrend loads a trend report
func LoadTrend(basePath string) (*TrendReport, error) {
	reportPath := filepath.Join(basePath, "metrics", "trend.json")

	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load trend report: %w", err)
	}

	var trend TrendReport
	if err := json.Unmarshal(data, &trend); err != nil {
		return nil, fmt.Errorf("failed to unmarshal trend report: %w", err)
	}

	return &trend, nil
}

// GetTrendSummary returns a human-readable summary
func (t *TrendReport) GetTrendSummary() string {
	switch t.Trend {
	case "improving":
		return fmt.Sprintf("Trend is improving (+%.1f%%)", t.Improvement)
	case "declining":
		return fmt.Sprintf("Trend is declining (%.1f%%)", t.Improvement)
	case "stable":
		return "Trend is stable"
	case "no_data":
		return "No data available"
	default:
		return "Insufficient data for trend analysis"
	}
}

// GetLatestWeek returns the most recent week in the trend
func (t *TrendReport) GetLatestWeek() *WeeklyMetrics {
	if len(t.Weeks) == 0 {
		return nil
	}
	return &t.Weeks[len(t.Weeks)-1]
}

// GetBestWeek returns the week with the highest success rate
func (t *TrendReport) GetBestWeek() *WeeklyMetrics {
	if len(t.Weeks) == 0 {
		return nil
	}

	best := &t.Weeks[0]
	for i := range t.Weeks {
		if t.Weeks[i].SuccessRate > best.SuccessRate {
			best = &t.Weeks[i]
		}
	}
	return best
}

// GetWorstWeek returns the week with the lowest success rate
func (t *TrendReport) GetWorstWeek() *WeeklyMetrics {
	if len(t.Weeks) == 0 {
		return nil
	}

	worst := &t.Weeks[0]
	for i := range t.Weeks {
		if t.Weeks[i].SuccessRate < worst.SuccessRate {
			worst = &t.Weeks[i]
		}
	}
	return worst
}
