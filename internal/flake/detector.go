package flake

import (
	"fmt"
	"math"
	"time"

	"github.com/mhingston/codefoundry/internal/replay"
	stagepkg "github.com/mhingston/codefoundry/internal/stage"
)

// Detector detects flaky tests/runs by replaying them multiple times
type Detector struct {
	runner    *stagepkg.Runner
	basePath  string
	threshold float64 // Success rate threshold (e.g., 0.95 for 95%)
}

// FlakeReport contains the results of flake detection
type FlakeReport struct {
	RunID       string           `json:"run_id"`
	ReplayCount int              `json:"replay_count"`
	Successes   int              `json:"successes"`
	Failures    int              `json:"failures"`
	SuccessRate float64          `json:"success_rate"`
	IsFlaky     bool             `json:"is_flaky"`
	Variance    float64          `json:"variance"`
	Differences []DiffSummary    `json:"differences"`
	Duration    time.Duration    `json:"duration"`
	Threshold   float64          `json:"threshold"`
	Results     []*ReplaySummary `json:"results,omitempty"`
}

// ReplaySummary summarizes a single replay result
type ReplaySummary struct {
	ReplayID    string    `json:"replay_id"`
	Success     bool      `json:"success"`
	Differences int       `json:"differences"`
	DurationMs  int64     `json:"duration_ms"`
}

// DiffSummary aggregates differences across replays
type DiffSummary struct {
	StageID   string `json:"stage_id,omitempty"`
	Field     string `json:"field"`
	Type      string `json:"type"`
	Count     int    `json:"count"`
	Expected  string `json:"expected,omitempty"`
	Variations []string `json:"variations,omitempty"`
}

// NewDetector creates a new flake detector
func NewDetector(runner *stagepkg.Runner, basePath string, threshold float64) *Detector {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.95 // Default: 95% success rate required
	}

	return &Detector{
		runner:    runner,
		basePath:  basePath,
		threshold: threshold,
	}
}

// Detect runs flake detection by replaying the run multiple times
func (d *Detector) Detect(runID string, replayCount int) (*FlakeReport, error) {
	if replayCount <= 0 {
		replayCount = 5 // Default
	}

	startTime := time.Now()

	report := &FlakeReport{
		RunID:       runID,
		ReplayCount: replayCount,
		Threshold:   d.threshold,
		Differences: make([]DiffSummary, 0),
		Results:     make([]*ReplaySummary, 0),
	}

	// Run multiple replays
	allDifferences := make(map[string][]replay.Difference)

	for i := 0; i < replayCount; i++ {
		result, err := replay.Replay(runID, d.runner, d.basePath)
		if err != nil {
			report.Failures++
			report.Results = append(report.Results, &ReplaySummary{
				ReplayID:    result.ReplayRunID,
				Success:     false,
				Differences: 0,
				DurationMs:  result.DurationMs,
			})
			continue
		}

		replaySummary := &ReplaySummary{
			ReplayID:    result.ReplayRunID,
			Success:     result.Matches,
			Differences: len(result.Differences),
			DurationMs:  result.DurationMs,
		}
		report.Results = append(report.Results, replaySummary)

		if result.Matches {
			report.Successes++
		} else {
			report.Failures++
			// Track differences
			for _, diff := range result.Differences {
				key := fmt.Sprintf("%s:%s:%s", diff.StageID, diff.Field, diff.Type)
				allDifferences[key] = append(allDifferences[key], diff)
			}
		}
	}

	// Calculate metrics
	report.SuccessRate = float64(report.Successes) / float64(replayCount)
	report.IsFlaky = report.SuccessRate < d.threshold
	report.Duration = time.Since(startTime)

	// Calculate variance in success rate
	if replayCount > 1 {
		// Variance = p * (1 - p) where p is success rate
		report.Variance = report.SuccessRate * (1 - report.SuccessRate)
	}

	// Aggregate differences
	report.Differences = aggregateDifferences(allDifferences)

	return report, nil
}

// aggregateDifferences aggregates differences by stage and field
func aggregateDifferences(diffMap map[string][]replay.Difference) []DiffSummary {
	summaries := make([]DiffSummary, 0)

	for _, diffs := range diffMap {
		if len(diffs) == 0 {
			continue
		}

		// Use first diff as template
		first := diffs[0]
		summary := DiffSummary{
			StageID:  first.StageID,
			Field:    first.Field,
			Type:     first.Type,
			Count:    len(diffs),
			Expected: fmt.Sprintf("%v", first.Expected),
			Variations: make([]string, 0),
		}

		// Collect unique actual values
		seen := make(map[string]bool)
		for _, diff := range diffs {
			actualStr := fmt.Sprintf("%v", diff.Actual)
			if !seen[actualStr] {
				seen[actualStr] = true
				summary.Variations = append(summary.Variations, actualStr)
			}
		}

		summaries = append(summaries, summary)
	}

	return summaries
}

// AnalyzeVariance performs statistical analysis on replay results
func (d *Detector) AnalyzeVariance(runID string, replayCount int) (*VarianceAnalysis, error) {
	report, err := d.Detect(runID, replayCount)
	if err != nil {
		return nil, err
	}

	analysis := &VarianceAnalysis{
		RunID:       runID,
		ReplayCount: report.ReplayCount,
		SuccessRate: report.SuccessRate,
		Variance:    report.Variance,
		StdDev:      math.Sqrt(report.Variance),
		IsFlaky:     report.IsFlaky,
		Confidence:  calculateConfidence(report.SuccessRate, report.ReplayCount),
	}

	// Categorize the type of flakiness
	if report.IsFlaky {
		analysis.FlakeType = categorizeFlakeType(report)
		analysis.Recommendation = generateRecommendation(report)
	}

	return analysis, nil
}

// VarianceAnalysis provides statistical analysis of variance
type VarianceAnalysis struct {
	RunID          string  `json:"run_id"`
	ReplayCount    int     `json:"replay_count"`
	SuccessRate    float64 `json:"success_rate"`
	Variance       float64 `json:"variance"`
	StdDev         float64 `json:"std_dev"`
	IsFlaky        bool    `json:"is_flaky"`
	Confidence     float64 `json:"confidence"`
	FlakeType      string  `json:"flake_type,omitempty"`
	Recommendation string  `json:"recommendation,omitempty"`
}

// calculateConfidence calculates the confidence interval for the success rate
func calculateConfidence(successRate float64, n int) float64 {
	if n == 0 {
		return 0
	}
	
	// Wilson score interval (simplified)
	// Confidence = success_rate ± 1.96 * sqrt(success_rate * (1 - success_rate) / n)
	z := 1.96 // 95% confidence
	margin := z * math.Sqrt(successRate*(1-successRate)/float64(n))
	
	// Return the confidence as 1 - margin (higher is better)
	return math.Max(0, 1-margin)
}

// categorizeFlakeType categorizes the type of flakiness
func categorizeFlakeType(report *FlakeReport) string {
	if report.SuccessRate == 0 {
		return "consistent_failure"
	}
	
	if report.SuccessRate == 1 {
		return "consistent_success"
	}

	if report.SuccessRate < 0.5 {
		return "mostly_failing"
	}

	if len(report.Differences) > 0 {
		// Check for timing-related differences
		hasTimingDiffs := false
		hasOutputDiffs := false
		
		for _, diff := range report.Differences {
			switch diff.Type {
			case "timing", "duration":
				hasTimingDiffs = true
			case "output", "result":
				hasOutputDiffs = true
			}
		}
		
		if hasTimingDiffs && !hasOutputDiffs {
			return "timing_flaky"
		}
		
		if hasOutputDiffs {
			return "output_flaky"
		}
	}

	return "intermittent"
}

// generateRecommendation generates a recommendation based on the report
func generateRecommendation(report *FlakeReport) string {
	if report.SuccessRate >= report.Threshold {
		return "No action needed - success rate is acceptable"
	}

	if report.SuccessRate == 0 {
		return "Run consistently fails - investigate and fix underlying issues"
	}

	if report.SuccessRate < 0.5 {
		return "Run is mostly failing - review recent changes and environment"
	}

	if len(report.Differences) > 0 {
		firstDiff := report.Differences[0]
		return fmt.Sprintf("Investigate %s differences in stage %s (field: %s)", 
			firstDiff.Type, firstDiff.StageID, firstDiff.Field)
	}

	return "Review run for non-deterministic behavior"
}

// IsFlaky returns true if the run is considered flaky
func (d *Detector) IsFlaky(runID string, replayCount int) (bool, error) {
	report, err := d.Detect(runID, replayCount)
	if err != nil {
		return false, err
	}
	return report.IsFlaky, nil
}
