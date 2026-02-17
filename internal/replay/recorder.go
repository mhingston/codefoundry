package replay

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
)

// ExecutionTrace represents a complete execution trace for replay
type ExecutionTrace struct {
	RunID       string                 `json:"run_id"`
	Timestamp   time.Time              `json:"timestamp"`
	Protocol    string                 `json:"protocol"`
	ProtocolVer string                 `json:"protocol_version"`
	Stages      []StageExecution       `json:"stages"`
	Inputs      map[string]interface{} `json:"inputs"`
	Outputs     map[string]interface{} `json:"outputs"`
	Environment EnvironmentState       `json:"environment"`
	DurationMs  int64                  `json:"duration_ms"`
}

// StageExecution represents a single stage execution
type StageExecution struct {
	StageID     string                 `json:"stage_id"`
	Status      string                 `json:"status"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt time.Time              `json:"completed_at,omitempty"`
	DurationMs  int64                  `json:"duration_ms"`
	Inputs      map[string]interface{} `json:"inputs,omitempty"`
	Outputs     map[string]interface{} `json:"outputs,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// EnvironmentState captures the execution environment
type EnvironmentState struct {
	GoVersion    string            `json:"go_version"`
	OS           string            `json:"os"`
	Arch         string            `json:"arch"`
	GitCommit    string            `json:"git_commit,omitempty"`
	WorkingDir   string            `json:"working_dir"`
	EnvVars      map[string]string `json:"env_vars"` // Sanitized
	NumCPU       int               `json:"num_cpu"`
}

// Recorder records execution traces
type Recorder struct {
	trace     *ExecutionTrace
	startedAt time.Time
}

// NewRecorder creates a new execution recorder
func NewRecorder(runID string) *Recorder {
	return &Recorder{
		trace: &ExecutionTrace{
			RunID:     runID,
			Timestamp: time.Now().UTC(),
			Stages:    make([]StageExecution, 0),
			Inputs:    make(map[string]interface{}),
			Outputs:   make(map[string]interface{}),
			Environment: EnvironmentState{
				EnvVars: make(map[string]string),
			},
		},
		startedAt: time.Now(),
	}
}

// SetProtocol sets the protocol name and version
func (r *Recorder) SetProtocol(name, version string) {
	r.trace.Protocol = name
	r.trace.ProtocolVer = version
}

// RecordInput records an input parameter
func (r *Recorder) RecordInput(key string, value interface{}) {
	r.trace.Inputs[key] = value
}

// RecordOutput records an output parameter
func (r *Recorder) RecordOutput(key string, value interface{}) {
	r.trace.Outputs[key] = value
}

// RecordStage records a stage execution
func (r *Recorder) RecordStage(stageID string, input, output interface{}) *StageExecution {
	now := time.Now().UTC()
	
	stage := StageExecution{
		StageID:   stageID,
		StartedAt: now,
	}
	
	if input != nil {
		stage.Inputs = toMap(input)
	}
	if output != nil {
		stage.Outputs = toMap(output)
	}
	
	r.trace.Stages = append(r.trace.Stages, stage)
	return &r.trace.Stages[len(r.trace.Stages)-1]
}

// CompleteStage marks a stage as complete
func (r *Recorder) CompleteStage(stageID string, status string, err error) {
	for i := range r.trace.Stages {
		if r.trace.Stages[i].StageID == stageID {
			r.trace.Stages[i].Status = status
			r.trace.Stages[i].CompletedAt = time.Now().UTC()
			r.trace.Stages[i].DurationMs = time.Since(r.trace.Stages[i].StartedAt).Milliseconds()
			if err != nil {
				r.trace.Stages[i].Error = err.Error()
			}
			break
		}
	}
}

// CaptureEnvironment captures the current execution environment
func (r *Recorder) CaptureEnvironment() error {
	r.trace.Environment.GoVersion = runtime.Version()
	r.trace.Environment.OS = runtime.GOOS
	r.trace.Environment.Arch = runtime.GOARCH
	r.trace.Environment.NumCPU = runtime.NumCPU()
	
	// Get working directory
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	r.trace.Environment.WorkingDir = wd
	
	// Capture sanitized environment variables
	r.captureEnvVars()
	
	// Try to get git commit
	r.captureGitCommit()
	
	return nil
}

// captureEnvVars captures relevant environment variables
func (r *Recorder) captureEnvVars() {
	// List of environment variables to capture (sanitized)
	varsToCapture := []string{
		"GOVERSION",
		"GOPATH",
		"GOROOT",
		"CI",
		"GITHUB_ACTIONS",
		"GITHUB_REF",
		"GITHUB_SHA",
	}
	
	for _, key := range varsToCapture {
		if value := os.Getenv(key); value != "" {
			r.trace.Environment.EnvVars[key] = value
		}
	}
}

// captureGitCommit attempts to get the current git commit
func (r *Recorder) captureGitCommit() {
	// This would typically use git command, simplified here
	if sha := os.Getenv("GITHUB_SHA"); sha != "" {
		r.trace.Environment.GitCommit = sha
	}
}

// Finalize marks the trace as complete
func (r *Recorder) Finalize() {
	r.trace.DurationMs = time.Since(r.startedAt).Milliseconds()
}

// Save saves the execution trace to the artifact store
func (r *Recorder) Save(artifactStore *artifact.Store) error {
	r.Finalize()
	
	data, err := json.MarshalIndent(r.trace, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal execution trace: %w", err)
	}
	
	if err := artifactStore.Write("_trace", "execution-trace.json", data); err != nil {
		return fmt.Errorf("failed to save execution trace: %w", err)
	}
	
	return nil
}

// GetTrace returns the current execution trace
func (r *Recorder) GetTrace() *ExecutionTrace {
	return r.trace
}

// ToJSON returns the trace as JSON bytes
func (r *Recorder) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r.trace, "", "  ")
}

// toMap converts an interface to a map using JSON marshaling
func toMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}
	
	data, err := json.Marshal(v)
	if err != nil {
		return map[string]interface{}{"_error": err.Error()}
	}
	
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return map[string]interface{}{"_raw": v}
	}
	
	return result
}
