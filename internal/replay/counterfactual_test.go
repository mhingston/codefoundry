package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/gate"
	"github.com/mhingston/codefoundry/internal/lock"
	"github.com/mhingston/codefoundry/internal/review"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCandidateConfigYAML(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "candidate.yaml")
	err := os.WriteFile(path, []byte("name: fast-harness\nscore_delta: 0.05\nminimum_runs: 3\n"), 0644)
	require.NoError(t, err)

	cfg, err := LoadCandidateConfig(path)
	require.NoError(t, err)
	assert.Equal(t, "fast-harness", cfg.Name)
	assert.Equal(t, 1.0, cfg.ScoreMultiplier)
	assert.Equal(t, 0.05, cfg.ScoreDelta)
	assert.Equal(t, 3, cfg.MinimumRuns)
}

func TestLoadCandidateConfigInvalid(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "candidate.json")
	err := os.WriteFile(path, []byte(`{"name":"bad","flake_rate_delta":2}`), 0644)
	require.NoError(t, err)

	_, err = LoadCandidateConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "flake_rate_delta")
}

func TestAnalyzeCounterfactual(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), ".codefoundry")

	for i, runID := range []string{"run-1", "run-2", "run-3"} {
		ns := artifact.NewNamespace(basePath, runID)
		store := artifact.NewStore(ns)

		trace := ExecutionTrace{RunID: runID, Timestamp: time.Now().UTC()}
		traceData, _ := json.Marshal(trace)
		require.NoError(t, store.Write("_trace", "execution-trace.json", traceData))

		reviewResult := review.NewReviewResult()
		reviewResult.RubricScore = 60 + i*10
		reviewResult.ConfidenceScore = 0.9
		reviewData, _ := reviewResult.ToJSON()
		require.NoError(t, store.Write("review", "review-result.json", reviewData))

		lockDecision := lock.NewLockDecision()
		lockDecision.Decision = lock.DecisionResolved
		lockData, _ := lockDecision.ToJSON()
		require.NoError(t, store.Write("lock", "lock-decision.json", lockData))

		gateResult := gate.GateResult{SchemaVersion: "codefoundry_gate_report.v1", GateID: "test", Status: "pass"}
		gateData, _ := json.Marshal(gateResult)
		require.NoError(t, store.Write("lock", "gate.json", gateData))

		replayResult := ReplayResult{OriginalRunID: runID, ReplayRunID: "r", Matches: true, ReplayCount: 5}
		if i == 2 {
			replayResult.Matches = false
			replayResult.Differences = []Difference{{StageID: "lock", Field: "status", Expected: "pass", Actual: "fail"}}
		}
		require.NoError(t, SaveReplayResult(store, &replayResult))
	}

	candidate := &CandidateConfig{
		Name:            "candidate-a",
		ScoreMultiplier: 1.0,
		ScoreDelta:      0.20,
		GatePassDelta:   0.00,
		FlakeRateDelta:  -0.03,
		MinimumRuns:     2,
	}

	report, err := AnalyzeCounterfactual(basePath, candidate)
	require.NoError(t, err)
	assert.Equal(t, 3, report.RunsAnalyzed)
	assert.Equal(t, 3, report.RunsWithReplayData)
	assert.Greater(t, report.Deltas.ScoreDelta.Mean, 0.0)
	assert.LessOrEqual(t, report.Deltas.GateDelta.Mean, 0.0)
	assert.Less(t, report.Deltas.FlakeDelta.Mean, 0.0)
	assert.Equal(t, "adopt", report.Recommendation)
}

func TestAnalyzeCounterfactualConfidenceThresholdPenalty(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), ".codefoundry")
	ns := artifact.NewNamespace(basePath, "run-1")
	store := artifact.NewStore(ns)
	trace := ExecutionTrace{RunID: "run-1", Timestamp: time.Now().UTC()}
	traceData, _ := json.Marshal(trace)
	require.NoError(t, store.Write("_trace", "execution-trace.json", traceData))

	reviewResult := review.NewReviewResult()
	reviewResult.RubricScore = 80
	reviewResult.ConfidenceScore = 0.75
	reviewResult.ConfidenceThreshold = 0.70
	reviewData, _ := reviewResult.ToJSON()
	require.NoError(t, store.Write("review", "review-result.json", reviewData))

	lockDecision := lock.NewLockDecision()
	lockDecision.Decision = lock.DecisionResolved
	lockData, _ := lockDecision.ToJSON()
	require.NoError(t, store.Write("lock", "lock-decision.json", lockData))

	candidate := &CandidateConfig{Name: "strict", ConfidenceThresholdDelta: 0.2, MinimumRuns: 1, ScoreMultiplier: 1}
	report, err := AnalyzeCounterfactual(basePath, candidate)
	require.NoError(t, err)
	assert.Equal(t, "reject", report.Recommendation)
}

func TestAnalyzeCounterfactualInsufficientRuns(t *testing.T) {
	basePath := filepath.Join(t.TempDir(), ".codefoundry")
	ns := artifact.NewNamespace(basePath, "run-only")
	store := artifact.NewStore(ns)
	trace := ExecutionTrace{RunID: "run-only", Timestamp: time.Now().UTC()}
	traceData, _ := json.Marshal(trace)
	require.NoError(t, store.Write("_trace", "execution-trace.json", traceData))

	candidate := &CandidateConfig{Name: "tiny", MinimumRuns: 5}
	report, err := AnalyzeCounterfactual(basePath, candidate)
	require.NoError(t, err)
	assert.Equal(t, "hold", report.Recommendation)
	assert.Contains(t, report.Rationale, "insufficient evidence")
}
