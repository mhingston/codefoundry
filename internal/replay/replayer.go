package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	stagepkg "github.com/mhingston/codefoundry/internal/stage"
)

// ReplayResult represents the result of a replay execution
type ReplayResult struct {
	OriginalRunID string       `json:"original_run_id"`
	ReplayRunID   string       `json:"replay_run_id"`
	Matches       bool         `json:"matches"`
	Differences   []Difference `json:"differences"`
	DurationMs    int64        `json:"duration_ms"`
	ReplayCount   int          `json:"replay_count"`
	Determinism   float64      `json:"determinism"`
}

// Difference represents a difference between expected and actual
type Difference struct {
	StageID  string      `json:"stage_id,omitempty"`
	Field    string      `json:"field"`
	Expected interface{} `json:"expected"`
	Actual   interface{} `json:"actual"`
	Type     string      `json:"type"` // output, status, error
}

// Comparison represents a comparison between two runs
type Comparison struct {
	RunID1      string       `json:"run_id_1"`
	RunID2      string       `json:"run_id_2"`
	Matches     bool         `json:"matches"`
	Differences []Difference `json:"differences"`
}

// LoadTrace loads an execution trace from the artifact store
func LoadTrace(runID string, basePath string) (*ExecutionTrace, error) {
	ns := artifact.NewNamespace(basePath, runID)
	store := artifact.NewStore(ns)

	return LoadTraceFromStore(store)
}

// LoadTraceFromStore loads a trace from an artifact store
func LoadTraceFromStore(store *artifact.Store) (*ExecutionTrace, error) {
	data, err := store.Read("_trace", "execution-trace.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load execution trace: %w", err)
	}

	var trace ExecutionTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		return nil, fmt.Errorf("failed to unmarshal execution trace: %w", err)
	}

	return &trace, nil
}

// Replay replays an execution and compares results
func Replay(runID string, runner *stagepkg.Runner, basePath string) (*ReplayResult, error) {
	// Load original trace
	originalTrace, err := LoadTrace(runID, basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load original trace: %w", err)
	}

	startTime := time.Now()

	// Create replay result
	result := &ReplayResult{
		OriginalRunID: runID,
		ReplayRunID:   generateReplayID(),
		Differences:   make([]Difference, 0),
	}

	// Replay each stage
	for _, originalStage := range originalTrace.Stages {
		ctx := context.Background()

		// Run the stage
		if err := runner.RunSingleStage(ctx, originalStage.StageID); err != nil {
			result.Differences = append(result.Differences, Difference{
				StageID:  originalStage.StageID,
				Field:    "execution",
				Expected: "success",
				Actual:   err.Error(),
				Type:     "error",
			})
			continue
		}

		// Compare outputs
		diffs := compareStageOutputs(&originalStage, runner)
		result.Differences = append(result.Differences, diffs...)
	}

	result.DurationMs = time.Since(startTime).Milliseconds()
	result.Matches = len(result.Differences) == 0
	result.ReplayCount = 1
	if result.Matches {
		result.Determinism = 1
	}

	return result, nil
}

// ReplayMultiple runs multiple replays for flake detection
func ReplayMultiple(runID string, runner *stagepkg.Runner, basePath string, count int) (*ReplayResult, error) {
	if count <= 0 {
		count = 5 // Default
	}

	allDifferences := make([]Difference, 0)
	var totalDuration int64

	for i := 0; i < count; i++ {
		result, err := Replay(runID, runner, basePath)
		if err != nil {
			return nil, fmt.Errorf("replay %d failed: %w", i+1, err)
		}

		allDifferences = append(allDifferences, result.Differences...)
		totalDuration += result.DurationMs
	}

	// Consolidate results
	consolidated := consolidateDifferences(allDifferences)

	return &ReplayResult{
		OriginalRunID: runID,
		ReplayRunID:   generateReplayID(),
		Matches:       len(consolidated) == 0,
		Differences:   consolidated,
		DurationMs:    totalDuration / int64(count),
		ReplayCount:   count,
		Determinism:   determinismFromDifferences(consolidated),
	}, nil
}

// CompareRuns compares two execution runs
func CompareRuns(runID1, runID2 string, basePath string) (*Comparison, error) {
	trace1, err := LoadTrace(runID1, basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load trace 1: %w", err)
	}

	trace2, err := LoadTrace(runID2, basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load trace 2: %w", err)
	}

	comparison := &Comparison{
		RunID1:      runID1,
		RunID2:      runID2,
		Differences: make([]Difference, 0),
	}

	// Compare stage counts
	if len(trace1.Stages) != len(trace2.Stages) {
		comparison.Differences = append(comparison.Differences, Difference{
			Field:    "stage_count",
			Expected: len(trace1.Stages),
			Actual:   len(trace2.Stages),
			Type:     "output",
		})
	}

	// Compare each stage
	for i, stage1 := range trace1.Stages {
		if i >= len(trace2.Stages) {
			break
		}

		stage2 := trace2.Stages[i]

		// Compare stage status
		if stage1.Status != stage2.Status {
			comparison.Differences = append(comparison.Differences, Difference{
				StageID:  stage1.StageID,
				Field:    "status",
				Expected: stage1.Status,
				Actual:   stage2.Status,
				Type:     "status",
			})
		}

		// Compare outputs
		diffs := compareValues(stage1.StageID, "output", stage1.Outputs, stage2.Outputs)
		comparison.Differences = append(comparison.Differences, diffs...)
	}

	comparison.Matches = len(comparison.Differences) == 0

	return comparison, nil
}

// compareStageOutputs compares stage outputs with current execution
func compareStageOutputs(original *StageExecution, runner *stagepkg.Runner) []Difference {
	differences := make([]Difference, 0)

	// Get current stage status from runner
	statuses, err := runner.GetStageStatuses()
	if err != nil {
		return []Difference{{
			StageID:  original.StageID,
			Field:    "status_check",
			Expected: "accessible",
			Actual:   err.Error(),
			Type:     "error",
		}}
	}

	currentStatus, ok := statuses[original.StageID]
	if !ok {
		return []Difference{{
			StageID:  original.StageID,
			Field:    "status",
			Expected: original.Status,
			Actual:   "not_found",
			Type:     "status",
		}}
	}

	if currentStatus != original.Status {
		differences = append(differences, Difference{
			StageID:  original.StageID,
			Field:    "status",
			Expected: original.Status,
			Actual:   currentStatus,
			Type:     "status",
		})
	}

	return differences
}

// compareValues recursively compares two values and returns differences
func compareValues(stageID, field string, expected, actual interface{}) []Difference {
	differences := make([]Difference, 0)

	if expected == nil && actual == nil {
		return differences
	}

	if expected == nil || actual == nil {
		return []Difference{{
			StageID:  stageID,
			Field:    field,
			Expected: expected,
			Actual:   actual,
			Type:     "output",
		}}
	}

	// Use reflection for deep comparison
	if !reflect.DeepEqual(expected, actual) {
		differences = append(differences, Difference{
			StageID:  stageID,
			Field:    field,
			Expected: expected,
			Actual:   actual,
			Type:     "output",
		})
	}

	return differences
}

// consolidateDifferences removes duplicate differences
func consolidateDifferences(diffs []Difference) []Difference {
	seen := make(map[string]bool)
	result := make([]Difference, 0)

	for _, diff := range diffs {
		key := fmt.Sprintf("%s:%s:%v", diff.StageID, diff.Field, diff.Expected)
		if !seen[key] {
			seen[key] = true
			result = append(result, diff)
		}
	}

	return result
}

// generateReplayID generates a unique replay run ID
func generateReplayID() string {
	return fmt.Sprintf("replay-%d", time.Now().UnixNano())
}

// SaveReplayResult saves a replay result to the artifact store
func SaveReplayResult(store *artifact.Store, result *ReplayResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal replay result: %w", err)
	}

	if err := store.Write("_replay", "replay-result.json", data); err != nil {
		return fmt.Errorf("failed to save replay result: %w", err)
	}

	return nil
}

// LoadReplayResult loads a replay result from the artifact store
func LoadReplayResult(store *artifact.Store) (*ReplayResult, error) {
	data, err := store.Read("_replay", "replay-result.json")
	if err != nil {
		return nil, fmt.Errorf("failed to load replay result: %w", err)
	}

	var result ReplayResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal replay result: %w", err)
	}

	return &result, nil
}

// ListTraces returns a list of all execution traces in the base path
func ListTraces(basePath string) ([]string, error) {
	traces := make([]string, 0)

	artifactsPath := filepath.Join(basePath, "artifacts")
	entries, err := os.ReadDir(artifactsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return traces, nil
		}
		return nil, fmt.Errorf("failed to read artifacts directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Check if trace exists
			tracePath := filepath.Join(artifactsPath, entry.Name(), "_trace", "execution-trace.json")
			if _, err := os.Stat(tracePath); err == nil {
				traces = append(traces, entry.Name())
			}
		}
	}

	return traces, nil
}

func determinismFromDifferences(diffs []Difference) float64 {
	if len(diffs) == 0 {
		return 1
	}
	return 0
}
