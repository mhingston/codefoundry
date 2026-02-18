package metrics

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/gate"
	"github.com/mhingston/codefoundry/internal/lock"
	"github.com/mhingston/codefoundry/internal/review"
)

// WeeklyMetrics represents metrics for a single week
type WeeklyMetrics struct {
	Week           string        `json:"week"` // ISO week format: 2026-W08
	SuccessRate    float64       `json:"success_rate"`
	AvgConfidence  float64       `json:"avg_confidence"`
	AvgRubricScore int           `json:"avg_rubric_score"`
	AvgCycleTime   time.Duration `json:"avg_cycle_time"`
	P1Findings     int           `json:"p1_findings"`
	P2Findings     int           `json:"p2_findings"`
	P3Findings     int           `json:"p3_findings"`
	GatePassRate   float64       `json:"gate_pass_rate"`
	ReplayPassRate float64       `json:"replay_pass_rate"`
	RunsCompleted  int           `json:"runs_completed"`
	RunsFailed     int           `json:"runs_failed"`
	TotalRuns      int           `json:"total_runs"`
}

// GetISOWeek returns the ISO week string (e.g., "2026-W08")
func GetISOWeek(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-W%02d", year, week)
}

// ParseISOWeek parses an ISO week string
func ParseISOWeek(weekStr string) (time.Time, error) {
	re := regexp.MustCompile(`^(\d{4})-W(\d{2})$`)
	matches := re.FindStringSubmatch(weekStr)
	if matches == nil {
		return time.Time{}, fmt.Errorf("invalid ISO week format: %s (expected YYYY-WXX)", weekStr)
	}

	year, _ := strconv.Atoi(matches[1])
	week, _ := strconv.Atoi(matches[2])

	// Find the first day of the week
	// January 4th is always in week 1
	// The Monday of week 1 is calculated correctly for any weekday of Jan 4
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	// Days to subtract to get to Monday (0=Monday in ISO, but Sunday=0 in Go)
	daysToSubtract := (int(jan4.Weekday()) + 6) % 7
	week1Start := jan4.AddDate(0, 0, -daysToSubtract) // Monday of week 1

	// Add (week-1) weeks
	return week1Start.AddDate(0, 0, (week-1)*7), nil
}

// GetCurrentWeek returns the current ISO week
func GetCurrentWeek() string {
	return GetISOWeek(time.Now())
}

// GetPreviousWeek returns the previous ISO week
func GetPreviousWeek(weekStr string) (string, error) {
	t, err := ParseISOWeek(weekStr)
	if err != nil {
		return "", err
	}
	return GetISOWeek(t.AddDate(0, 0, -7)), nil
}

// GetNextWeek returns the next ISO week
func GetNextWeek(weekStr string) (string, error) {
	t, err := ParseISOWeek(weekStr)
	if err != nil {
		return "", err
	}
	return GetISOWeek(t.AddDate(0, 0, 7)), nil
}

// GetWeekRange returns the start and end dates for an ISO week
func GetWeekRange(weekStr string) (start, end time.Time, err error) {
	start, err = ParseISOWeek(weekStr)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	end = start.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	return start, end, nil
}

// RunData represents data extracted from a single run
type RunData struct {
	RunID        string
	Timestamp    time.Time
	Success      bool
	ReviewResult *review.ReviewResult
	LockDecision *lock.LockDecision
	GateResults  []gate.GateResult
	CycleTime    time.Duration
}

// ExtractRunData extracts metrics data from a run's artifacts
func ExtractRunData(runID string, store *artifact.Store) (*RunData, error) {
	data := &RunData{
		RunID: runID,
	}

	// Try to get timestamp from trace
	traceData, err := store.Read("_trace", "execution-trace.json")
	if err == nil {
		var trace struct {
			Timestamp string `json:"timestamp"`
		}
		if json.Unmarshal(traceData, &trace) == nil && trace.Timestamp != "" {
			data.Timestamp, _ = time.Parse(time.RFC3339, trace.Timestamp)
		}
	}

	// Load review result
	reviewData, err := store.Read("review", "review-result.json")
	if err == nil {
		result, err := review.FromJSON(reviewData)
		if err == nil {
			data.ReviewResult = result
		}
	}

	// Load lock decision
	lockData, err := store.Read("lock", "lock-decision.json")
	if err == nil {
		decision, err := lock.FromJSON(lockData)
		if err == nil {
			data.LockDecision = decision
			data.Success = decision.Decision == lock.DecisionResolved
		}
	}

	// Load gate results
	data.GateResults = loadGateResults(store)

	return data, nil
}

// loadGateResults loads all gate results from a run
func loadGateResults(store *artifact.Store) []gate.GateResult {
	results := make([]gate.GateResult, 0)

	// Check all stages for gate results
	// We need to discover stages first
	artifactsPath := store.Namespace().RunPath()
	entries, err := os.ReadDir(artifactsPath)
	if err != nil {
		return results
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), "_") {
			continue
		}

		stageID := entry.Name()
		artifacts, err := store.List(stageID)
		if err != nil {
			continue
		}

		for _, artifact := range artifacts {
			if !strings.HasSuffix(artifact, ".json") ||
				artifact == "status.json" ||
				artifact == "review-result.json" ||
				artifact == "lock-decision.json" {
				continue
			}

			data, err := store.Read(stageID, artifact)
			if err != nil {
				continue
			}

			var result gate.GateResult
			if err := json.Unmarshal(data, &result); err != nil {
				continue
			}

			if result.SchemaVersion == "codefoundry_gate_report.v1" {
				results = append(results, result)
			}
		}
	}

	return results
}

// calculateSuccessRate calculates the success rate from runs
func calculateSuccessRate(runs []*RunData) float64 {
	if len(runs) == 0 {
		return 0
	}

	successes := 0
	for _, run := range runs {
		if run.Success {
			successes++
		}
	}

	return float64(successes) / float64(len(runs))
}

// calculateAvgConfidence calculates average confidence from review results
func calculateAvgConfidence(runs []*RunData) float64 {
	total := 0.0
	count := 0

	for _, run := range runs {
		if run.ReviewResult != nil {
			total += run.ReviewResult.ConfidenceScore
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return total / float64(count)
}

// calculateAvgRubricScore calculates average rubric score
func calculateAvgRubricScore(runs []*RunData) int {
	total := 0
	count := 0

	for _, run := range runs {
		if run.ReviewResult != nil {
			total += run.ReviewResult.RubricScore
			count++
		}
	}

	if count == 0 {
		return 0
	}

	return total / count
}

// calculateFindings calculates total findings by severity
func calculateFindings(runs []*RunData) (p1, p2, p3 int) {
	for _, run := range runs {
		if run.ReviewResult != nil {
			p1 += run.ReviewResult.P1Count
			p2 += run.ReviewResult.P2Count
			p3 += run.ReviewResult.P3Count
		}
	}
	return
}

// calculateGatePassRate calculates the gate pass rate
func calculateGatePassRate(runs []*RunData) float64 {
	totalGates := 0
	passedGates := 0

	for _, run := range runs {
		for _, gate := range run.GateResults {
			totalGates++
			if gate.Status == "pass" {
				passedGates++
			}
		}
	}

	if totalGates == 0 {
		return 0
	}

	return float64(passedGates) / float64(totalGates)
}

// filterRunsByWeek filters runs to only those within a specific week
func filterRunsByWeek(runs []*RunData, weekStr string) []*RunData {
	start, end, err := GetWeekRange(weekStr)
	if err != nil {
		return runs
	}

	filtered := make([]*RunData, 0)
	for _, run := range runs {
		if !run.Timestamp.IsZero() &&
			(run.Timestamp.Equal(start) || run.Timestamp.After(start)) &&
			(run.Timestamp.Equal(end) || run.Timestamp.Before(end)) {
			filtered = append(filtered, run)
		}
	}

	return filtered
}

// SortWeeks sorts ISO week strings chronologically
func SortWeeks(weeks []string) []string {
	sorted := make([]string, len(weeks))
	copy(sorted, weeks)

	sort.Slice(sorted, func(i, j int) bool {
		t1, _ := ParseISOWeek(sorted[i])
		t2, _ := ParseISOWeek(sorted[j])
		return t1.Before(t2)
	})

	return sorted
}

// GetWeeksInRange returns all ISO weeks between two dates
func GetWeeksInRange(start, end time.Time) []string {
	weeks := make([]string, 0)

	current := start
	for current.Before(end) || current.Equal(end) {
		week := GetISOWeek(current)
		// Avoid duplicates
		if len(weeks) == 0 || weeks[len(weeks)-1] != week {
			weeks = append(weeks, week)
		}
		current = current.AddDate(0, 0, 7)
	}

	return weeks
}

// LoadWeeklyReport loads a previously saved weekly report
func LoadWeeklyReport(basePath, weekStr string) (*WeeklyMetrics, error) {
	reportPath := filepath.Join(basePath, "metrics", fmt.Sprintf("weekly-%s.json", weekStr))

	data, err := os.ReadFile(reportPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load weekly report: %w", err)
	}

	var metrics WeeklyMetrics
	if err := json.Unmarshal(data, &metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal weekly report: %w", err)
	}

	return &metrics, nil
}

// SaveWeeklyReport saves a weekly report
func SaveWeeklyReport(basePath string, metrics *WeeklyMetrics) error {
	reportPath := filepath.Join(basePath, "metrics", fmt.Sprintf("weekly-%s.json", metrics.Week))

	// Ensure directory exists
	dir := filepath.Dir(reportPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create metrics directory: %w", err)
	}

	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal weekly report: %w", err)
	}

	if err := os.WriteFile(reportPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write weekly report: %w", err)
	}

	return nil
}
