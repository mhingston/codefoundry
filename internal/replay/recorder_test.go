package replay

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRecorder(t *testing.T) {
	runID := "test-run-123"
	recorder := NewRecorder(runID)

	assert.NotNil(t, recorder)
	assert.NotNil(t, recorder.trace)
	assert.Equal(t, runID, recorder.trace.RunID)
	assert.NotZero(t, recorder.trace.Timestamp)
	assert.NotNil(t, recorder.trace.Stages)
	assert.NotNil(t, recorder.trace.Inputs)
	assert.NotNil(t, recorder.trace.Outputs)
}

func TestRecorder_SetProtocol(t *testing.T) {
	recorder := NewRecorder("test-run")
	recorder.SetProtocol("default", "1.0.0")

	assert.Equal(t, "default", recorder.trace.Protocol)
	assert.Equal(t, "1.0.0", recorder.trace.ProtocolVer)
}

func TestRecorder_RecordInput(t *testing.T) {
	recorder := NewRecorder("test-run")
	recorder.RecordInput("key1", "value1")
	recorder.RecordInput("key2", 42)

	assert.Equal(t, "value1", recorder.trace.Inputs["key1"])
	assert.Equal(t, 42, recorder.trace.Inputs["key2"])
}

func TestRecorder_RecordOutput(t *testing.T) {
	recorder := NewRecorder("test-run")
	recorder.RecordOutput("result", "success")
	recorder.RecordOutput("count", 100)

	assert.Equal(t, "success", recorder.trace.Outputs["result"])
	assert.Equal(t, 100, recorder.trace.Outputs["count"])
}

func TestRecorder_RecordStage(t *testing.T) {
	recorder := NewRecorder("test-run")

	input := map[string]string{"key": "value"}
	output := map[string]string{"result": "success"}

	stage := recorder.RecordStage("stage-1", input, output)

	assert.NotNil(t, stage)
	assert.Equal(t, "stage-1", stage.StageID)
	assert.NotZero(t, stage.StartedAt)
	assert.Equal(t, "value", stage.Inputs["key"])
	assert.Equal(t, "success", stage.Outputs["result"])

	// Verify stage was added to trace
	assert.Len(t, recorder.trace.Stages, 1)
	assert.Equal(t, "stage-1", recorder.trace.Stages[0].StageID)
}

func TestRecorder_CompleteStage(t *testing.T) {
	recorder := NewRecorder("test-run")
	recorder.RecordStage("stage-1", nil, nil)

	time.Sleep(10 * time.Millisecond) // Ensure some duration
	recorder.CompleteStage("stage-1", "pass", nil)

	stage := recorder.trace.Stages[0]
	assert.Equal(t, "pass", stage.Status)
	assert.NotZero(t, stage.CompletedAt)
	assert.True(t, stage.DurationMs >= 10)
}

func TestRecorder_CompleteStage_WithError(t *testing.T) {
	recorder := NewRecorder("test-run")
	recorder.RecordStage("stage-1", nil, nil)

	testErr := errors.New("test error")
	recorder.CompleteStage("stage-1", "fail", testErr)

	stage := recorder.trace.Stages[0]
	assert.Equal(t, "fail", stage.Status)
	assert.Equal(t, "test error", stage.Error)
}

func TestRecorder_CaptureEnvironment(t *testing.T) {
	recorder := NewRecorder("test-run")
	err := recorder.CaptureEnvironment()

	require.NoError(t, err)

	env := recorder.trace.Environment
	assert.NotEmpty(t, env.GoVersion)
	assert.NotEmpty(t, env.OS)
	assert.NotEmpty(t, env.Arch)
	assert.NotEmpty(t, env.WorkingDir)
	assert.True(t, env.NumCPU > 0)
	assert.NotNil(t, env.EnvVars)
}

func TestRecorder_Finalize(t *testing.T) {
	recorder := NewRecorder("test-run")

	time.Sleep(5 * time.Millisecond)
	recorder.Finalize()

	assert.True(t, recorder.trace.DurationMs >= 5)
}

func TestRecorder_Save(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	basePath := filepath.Join(tempDir, ".codefoundry")
	
	ns := artifact.NewNamespace(basePath, "test-run")
	store := artifact.NewStore(ns)

	recorder := NewRecorder("test-run")
	recorder.SetProtocol("default", "1.0.0")
	recorder.RecordInput("test", "value")
	recorder.RecordStage("stage-1", map[string]string{"input": "data"}, map[string]string{"output": "result"})
	recorder.CompleteStage("stage-1", "pass", nil)

	err := recorder.CaptureEnvironment()
	require.NoError(t, err)

	err = recorder.Save(store)
	require.NoError(t, err)

	// Verify file was created
	tracePath := filepath.Join(basePath, "artifacts", "test-run", "_trace", "execution-trace.json")
	_, err = os.Stat(tracePath)
	assert.NoError(t, err)

	// Verify content
	data, err := os.ReadFile(tracePath)
	require.NoError(t, err)

	var trace ExecutionTrace
	err = json.Unmarshal(data, &trace)
	require.NoError(t, err)

	assert.Equal(t, "test-run", trace.RunID)
	assert.Equal(t, "default", trace.Protocol)
	assert.Equal(t, "1.0.0", trace.ProtocolVer)
	assert.Len(t, trace.Stages, 1)
	assert.Equal(t, "stage-1", trace.Stages[0].StageID)
}

func TestRecorder_ToJSON(t *testing.T) {
	recorder := NewRecorder("test-run")
	recorder.SetProtocol("default", "1.0.0")

	data, err := recorder.ToJSON()
	require.NoError(t, err)

	var trace ExecutionTrace
	err = json.Unmarshal(data, &trace)
	require.NoError(t, err)

	assert.Equal(t, "test-run", trace.RunID)
}

func TestToMap(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected map[string]interface{}
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "simple struct",
			input: struct {
				Name  string `json:"name"`
				Value int    `json:"value"`
			}{Name: "test", Value: 42},
			expected: map[string]interface{}{"name": "test", "value": float64(42)},
		},
		{
			name:     "map",
			input:    map[string]string{"key": "value"},
			expected: map[string]interface{}{"key": "value"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toMap(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToMap_InvalidData(t *testing.T) {
	// Test with channel (can't be marshaled)
	ch := make(chan int)
	result := toMap(ch)
	
	assert.NotNil(t, result)
	assert.Contains(t, result, "_error")
}
