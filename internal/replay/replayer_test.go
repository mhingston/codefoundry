package replay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestArtifacts(t *testing.T, runID string) (string, *artifact.Store) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")
	
	ns := artifact.NewNamespace(basePath, runID)
	store := artifact.NewStore(ns)
	
	return basePath, store
}

func TestLoadTrace(t *testing.T) {
	runID := "test-run-123"
	basePath, store := setupTestArtifacts(t, runID)

	// Create a trace
	trace := &ExecutionTrace{
		RunID:       runID,
		Timestamp:   time.Now().UTC(),
		Protocol:    "default",
		ProtocolVer: "1.0.0",
		Stages: []StageExecution{
			{
				StageID:   "stage-1",
				Status:    "pass",
				StartedAt: time.Now().UTC(),
				Outputs:   map[string]interface{}{"result": "success"},
			},
		},
	}

	// Save the trace
	data, _ := json.Marshal(trace)
	store.Write("_trace", "execution-trace.json", data)

	// Load it back
	loaded, err := LoadTrace(runID, basePath)
	require.NoError(t, err)

	assert.Equal(t, runID, loaded.RunID)
	assert.Equal(t, "default", loaded.Protocol)
	assert.Len(t, loaded.Stages, 1)
	assert.Equal(t, "stage-1", loaded.Stages[0].StageID)
}

func TestLoadTrace_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	_, err := LoadTrace("non-existent", basePath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load execution trace")
}

func TestCompareRuns(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	// Create two traces
	trace1 := &ExecutionTrace{
		RunID:     "run-1",
		Timestamp: time.Now().UTC(),
		Stages: []StageExecution{
			{
				StageID: "stage-1",
				Status:  "pass",
				Outputs: map[string]interface{}{"result": "success"},
			},
		},
	}

	trace2 := &ExecutionTrace{
		RunID:     "run-2",
		Timestamp: time.Now().UTC(),
		Stages: []StageExecution{
			{
				StageID: "stage-1",
				Status:  "pass",
				Outputs: map[string]interface{}{"result": "success"},
			},
		},
	}

	// Save traces
	ns1 := artifact.NewNamespace(basePath, "run-1")
	store1 := artifact.NewStore(ns1)
	data1, _ := json.Marshal(trace1)
	store1.Write("_trace", "execution-trace.json", data1)

	ns2 := artifact.NewNamespace(basePath, "run-2")
	store2 := artifact.NewStore(ns2)
	data2, _ := json.Marshal(trace2)
	store2.Write("_trace", "execution-trace.json", data2)

	// Compare
	comparison, err := CompareRuns("run-1", "run-2", basePath)
	require.NoError(t, err)

	assert.True(t, comparison.Matches)
	assert.Len(t, comparison.Differences, 0)
}

func TestCompareRuns_WithDifferences(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	// Create two traces with different outputs
	trace1 := &ExecutionTrace{
		RunID:     "run-1",
		Timestamp: time.Now().UTC(),
		Stages: []StageExecution{
			{
				StageID: "stage-1",
				Status:  "pass",
				Outputs: map[string]interface{}{"result": "success", "count": 10},
			},
		},
	}

	trace2 := &ExecutionTrace{
		RunID:     "run-2",
		Timestamp: time.Now().UTC(),
		Stages: []StageExecution{
			{
				StageID: "stage-1",
				Status:  "fail",
				Outputs: map[string]interface{}{"result": "failure", "count": 5},
			},
		},
	}

	// Save traces
	ns1 := artifact.NewNamespace(basePath, "run-1")
	store1 := artifact.NewStore(ns1)
	data1, _ := json.Marshal(trace1)
	store1.Write("_trace", "execution-trace.json", data1)

	ns2 := artifact.NewNamespace(basePath, "run-2")
	store2 := artifact.NewStore(ns2)
	data2, _ := json.Marshal(trace2)
	store2.Write("_trace", "execution-trace.json", data2)

	// Compare
	comparison, err := CompareRuns("run-1", "run-2", basePath)
	require.NoError(t, err)

	assert.False(t, comparison.Matches)
	assert.Len(t, comparison.Differences, 2) // status and output
}

func TestCompareRuns_DifferentStageCounts(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	// Create traces with different stage counts
	trace1 := &ExecutionTrace{
		RunID:     "run-1",
		Timestamp: time.Now().UTC(),
		Stages: []StageExecution{
			{StageID: "stage-1", Status: "pass"},
			{StageID: "stage-2", Status: "pass"},
		},
	}

	trace2 := &ExecutionTrace{
		RunID:     "run-2",
		Timestamp: time.Now().UTC(),
		Stages: []StageExecution{
			{StageID: "stage-1", Status: "pass"},
		},
	}

	// Save traces
	ns1 := artifact.NewNamespace(basePath, "run-1")
	store1 := artifact.NewStore(ns1)
	data1, _ := json.Marshal(trace1)
	store1.Write("_trace", "execution-trace.json", data1)

	ns2 := artifact.NewNamespace(basePath, "run-2")
	store2 := artifact.NewStore(ns2)
	data2, _ := json.Marshal(trace2)
	store2.Write("_trace", "execution-trace.json", data2)

	// Compare
	comparison, err := CompareRuns("run-1", "run-2", basePath)
	require.NoError(t, err)

	assert.False(t, comparison.Matches)
	assert.True(t, len(comparison.Differences) > 0)
}

func TestConsolidateDifferences(t *testing.T) {
	diffs := []Difference{
		{StageID: "stage-1", Field: "status", Expected: "pass", Actual: "fail"},
		{StageID: "stage-1", Field: "status", Expected: "pass", Actual: "fail"}, // Duplicate
		{StageID: "stage-2", Field: "output", Expected: "a", Actual: "b"},
		{StageID: "stage-1", Field: "status", Expected: "pass", Actual: "fail"}, // Another duplicate
	}

	consolidated := consolidateDifferences(diffs)

	// Should have only 2 unique differences
	assert.Len(t, consolidated, 2)
}

func TestListTraces(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	// Create some traces
	for _, runID := range []string{"run-1", "run-2", "run-3"} {
		ns := artifact.NewNamespace(basePath, runID)
		store := artifact.NewStore(ns)
		trace := &ExecutionTrace{RunID: runID, Timestamp: time.Now().UTC()}
		data, _ := json.Marshal(trace)
		store.Write("_trace", "execution-trace.json", data)
	}

	// Create a directory without a trace
	os.MkdirAll(filepath.Join(basePath, "artifacts", "run-4"), 0755)

	// List traces
	traces, err := ListTraces(basePath)
	require.NoError(t, err)

	assert.Len(t, traces, 3)
	assert.Contains(t, traces, "run-1")
	assert.Contains(t, traces, "run-2")
	assert.Contains(t, traces, "run-3")
}

func TestListTraces_NoArtifactsDir(t *testing.T) {
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")

	// Don't create artifacts directory
	traces, err := ListTraces(basePath)
	require.NoError(t, err)
	assert.Len(t, traces, 0)
}

func TestSaveAndLoadReplayResult(t *testing.T) {
	runID := "test-run"
	_, store := setupTestArtifacts(t, runID)

	result := &ReplayResult{
		OriginalRunID: runID,
		ReplayRunID:   "replay-1",
		Matches:       true,
		Differences:   []Difference{},
		DurationMs:    1000,
		ReplayCount:   1,
	}

	err := SaveReplayResult(store, result)
	require.NoError(t, err)

	loaded, err := LoadReplayResult(store)
	require.NoError(t, err)

	assert.Equal(t, result.OriginalRunID, loaded.OriginalRunID)
	assert.Equal(t, result.ReplayRunID, loaded.ReplayRunID)
	assert.Equal(t, result.Matches, loaded.Matches)
	assert.Equal(t, result.DurationMs, loaded.DurationMs)
}

func TestLoadReplayResult_NotFound(t *testing.T) {
	runID := "test-run"
	_, store := setupTestArtifacts(t, runID)

	_, err := LoadReplayResult(store)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load replay result")
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name     string
		expected interface{}
		actual   interface{}
		wantDiff bool
	}{
		{
			name:     "both nil",
			expected: nil,
			actual:   nil,
			wantDiff: false,
		},
		{
			name:     "expected nil",
			expected: nil,
			actual:   "value",
			wantDiff: true,
		},
		{
			name:     "actual nil",
			expected: "value",
			actual:   nil,
			wantDiff: true,
		},
		{
			name:     "equal strings",
			expected: "value",
			actual:   "value",
			wantDiff: false,
		},
		{
			name:     "different strings",
			expected: "value1",
			actual:   "value2",
			wantDiff: true,
		},
		{
			name:     "equal maps",
			expected: map[string]string{"key": "value"},
			actual:   map[string]string{"key": "value"},
			wantDiff: false,
		},
		{
			name:     "different maps",
			expected: map[string]string{"key": "value1"},
			actual:   map[string]string{"key": "value2"},
			wantDiff: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diffs := compareValues("stage-1", "field", tt.expected, tt.actual)
			if tt.wantDiff {
				assert.Len(t, diffs, 1)
			} else {
				assert.Len(t, diffs, 0)
			}
		})
	}
}
