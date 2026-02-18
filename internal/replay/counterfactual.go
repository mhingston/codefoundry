package replay

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/metrics"
)

// CandidateConfig models protocol/harness parameter tweaks for counterfactual replay.
type CandidateConfig struct {
	Name                     string  `json:"name" yaml:"name"`
	Description              string  `json:"description,omitempty" yaml:"description,omitempty"`
	ScoreMultiplier          float64 `json:"score_multiplier,omitempty" yaml:"score_multiplier,omitempty"`
	ScoreDelta               float64 `json:"score_delta,omitempty" yaml:"score_delta,omitempty"`
	GatePassDelta            float64 `json:"gate_pass_delta,omitempty" yaml:"gate_pass_delta,omitempty"`
	FlakeRateDelta           float64 `json:"flake_rate_delta,omitempty" yaml:"flake_rate_delta,omitempty"`
	ConfidenceThresholdDelta float64 `json:"confidence_threshold_delta,omitempty" yaml:"confidence_threshold_delta,omitempty"`
	MinimumRuns              int     `json:"minimum_runs,omitempty" yaml:"minimum_runs,omitempty"`
}

// ConfidenceInterval provides a 95% confidence interval over a mean effect.
type ConfidenceInterval struct {
	Mean  float64 `json:"mean"`
	Lower float64 `json:"lower"`
	Upper float64 `json:"upper"`
}

// CounterfactualDelta captures baseline/candidate metric changes.
type CounterfactualDelta struct {
	ScoreDelta     ConfidenceInterval `json:"score_delta"`
	GateDelta      ConfidenceInterval `json:"gate_delta"`
	FlakeDelta     ConfidenceInterval `json:"flake_delta"`
	AdoptionSignal ConfidenceInterval `json:"adoption_signal"`
}

// CounterfactualReport summarizes historical evidence and recommendation.
type CounterfactualReport struct {
	Candidate          CandidateConfig     `json:"candidate"`
	RunsAnalyzed       int                 `json:"runs_analyzed"`
	RunsWithReplayData int                 `json:"runs_with_replay_data"`
	Deltas             CounterfactualDelta `json:"deltas"`
	Recommendation     string              `json:"recommendation"`
	Rationale          string              `json:"rationale"`
}

type runSnapshot struct {
	score     float64
	gateRate  float64
	flakeRate float64
	hasReplay bool
}

// LoadCandidateConfig parses a candidate file in YAML or JSON.
func LoadCandidateConfig(path string) (*CandidateConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read candidate file: %w", err)
	}

	candidate := &CandidateConfig{}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, candidate); err != nil {
			return nil, fmt.Errorf("failed to parse YAML candidate: %w", err)
		}
	default:
		if err := json.Unmarshal(data, candidate); err != nil {
			return nil, fmt.Errorf("failed to parse JSON candidate: %w", err)
		}
	}

	if candidate.Name == "" {
		candidate.Name = filepath.Base(path)
	}
	if candidate.ScoreMultiplier == 0 {
		candidate.ScoreMultiplier = 1
	}
	if candidate.MinimumRuns <= 0 {
		candidate.MinimumRuns = 10
	}
	return candidate, nil
}

// AnalyzeCounterfactual replays historical runs against candidate parameter changes.
func AnalyzeCounterfactual(basePath string, candidate *CandidateConfig) (*CounterfactualReport, error) {
	snapshots, replayCount, err := collectHistoricalSnapshots(basePath)
	if err != nil {
		return nil, err
	}
	if len(snapshots) == 0 {
		return nil, fmt.Errorf("no historical runs available for counterfactual analysis")
	}

	scoreDeltas := make([]float64, 0, len(snapshots))
	gateDeltas := make([]float64, 0, len(snapshots))
	flakeDeltas := make([]float64, 0, len(snapshots))
	adoptionSignals := make([]float64, 0, len(snapshots))

	for _, snap := range snapshots {
		candidateScore := clamp01(snap.score*candidate.ScoreMultiplier + candidate.ScoreDelta)
		candidateGate := clamp01(snap.gateRate + candidate.GatePassDelta)

		candidateFlake := snap.flakeRate
		if snap.hasReplay {
			candidateFlake = clamp01(snap.flakeRate + candidate.FlakeRateDelta)
		}

		scoreDelta := candidateScore - snap.score
		gateDelta := candidateGate - snap.gateRate
		flakeDelta := candidateFlake - snap.flakeRate

		scoreDeltas = append(scoreDeltas, scoreDelta)
		gateDeltas = append(gateDeltas, gateDelta)
		flakeDeltas = append(flakeDeltas, flakeDelta)
		adoptionSignals = append(adoptionSignals, scoreDelta+gateDelta-flakeDelta)
	}

	report := &CounterfactualReport{
		Candidate:          *candidate,
		RunsAnalyzed:       len(snapshots),
		RunsWithReplayData: replayCount,
		Deltas: CounterfactualDelta{
			ScoreDelta:     meanCI(scoreDeltas),
			GateDelta:      meanCI(gateDeltas),
			FlakeDelta:     meanCI(flakeDeltas),
			AdoptionSignal: meanCI(adoptionSignals),
		},
	}

	report.Recommendation, report.Rationale = classifyRecommendation(report)
	return report, nil
}

func collectHistoricalSnapshots(basePath string) ([]runSnapshot, int, error) {
	traces, err := ListTraces(basePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to enumerate traces: %w", err)
	}

	snapshots := make([]runSnapshot, 0, len(traces))
	runsWithReplay := 0
	for _, runID := range traces {
		ns := artifact.NewNamespace(basePath, runID)
		store := artifact.NewStore(ns)

		runData, err := metrics.ExtractRunData(runID, store)
		if err != nil {
			continue
		}

		snap := runSnapshot{
			score:    extractScore(runData),
			gateRate: extractGateRate(runData),
		}

		if replayResult, err := LoadReplayResult(store); err == nil {
			runsWithReplay++
			snap.hasReplay = true
			snap.flakeRate = deriveFlakeRate(replayResult)
		}

		snapshots = append(snapshots, snap)
	}
	return snapshots, runsWithReplay, nil
}

func extractScore(runData *metrics.RunData) float64 {
	if runData == nil || runData.ReviewResult == nil {
		return 0
	}
	return clamp01(float64(runData.ReviewResult.RubricScore) / 100.0)
}

func extractGateRate(runData *metrics.RunData) float64 {
	if runData == nil || len(runData.GateResults) == 0 {
		if runData != nil && runData.Success {
			return 1
		}
		return 0
	}

	passes := 0
	for _, g := range runData.GateResults {
		if g.Status == "pass" {
			passes++
		}
	}
	return float64(passes) / float64(len(runData.GateResults))
}

func deriveFlakeRate(result *ReplayResult) float64 {
	if result == nil {
		return 0
	}
	if result.ReplayCount <= 1 {
		if result.Matches {
			return 0
		}
		return 1
	}
	if result.Matches {
		return 0
	}
	// Approximate observed flake-rate with unique difference density.
	rate := float64(len(result.Differences)) / float64(result.ReplayCount)
	return clamp01(rate)
}

func meanCI(values []float64) ConfidenceInterval {
	if len(values) == 0 {
		return ConfidenceInterval{}
	}
	mean := mean(values)
	if len(values) == 1 {
		return ConfidenceInterval{Mean: mean, Lower: mean, Upper: mean}
	}
	stdErr := stddev(values, mean) / math.Sqrt(float64(len(values)))
	margin := 1.96 * stdErr
	return ConfidenceInterval{Mean: mean, Lower: mean - margin, Upper: mean + margin}
}

func classifyRecommendation(report *CounterfactualReport) (string, string) {
	if report.RunsAnalyzed < report.Candidate.MinimumRuns {
		return "hold", fmt.Sprintf("insufficient evidence: %d runs analyzed, need at least %d", report.RunsAnalyzed, report.Candidate.MinimumRuns)
	}
	signal := report.Deltas.AdoptionSignal
	switch {
	case signal.Lower > 0:
		return "adopt", fmt.Sprintf("positive adoption signal %.3f (95%% CI %.3f..%.3f)", signal.Mean, signal.Lower, signal.Upper)
	case signal.Upper < 0:
		return "reject", fmt.Sprintf("negative adoption signal %.3f (95%% CI %.3f..%.3f)", signal.Mean, signal.Lower, signal.Upper)
	default:
		return "hold", fmt.Sprintf("uncertain impact %.3f (95%% CI %.3f..%.3f)", signal.Mean, signal.Lower, signal.Upper)
	}
}

func mean(values []float64) float64 {
	total := 0.0
	for _, v := range values {
		total += v
	}
	return total / float64(len(values))
}

func stddev(values []float64, m float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	var sum float64
	for _, v := range values {
		d := v - m
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(values)-1))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
