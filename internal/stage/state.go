package stage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

const (
	StateVersion = "codefoundry_state.v1"
)

// State represents the runtime state of a workflow run
type State struct {
	SchemaVersion   string                 `json:"schema_version"`
	RunID          string                 `json:"run_id"`
	ProtocolVersion string                `json:"protocol_version"`
	Stages         map[string]*StageState  `json:"stages"`
	CurrentStage   string                  `json:"current_stage,omitempty"`
	Checkpoint     *Checkpoint             `json:"checkpoint,omitempty"`
	Metadata       *RunMetadata            `json:"metadata,omitempty"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// StageState represents the state of a single stage
type StageState struct {
	Status       string     `json:"status"`
	ArtifactPath string     `json:"artifact_path,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Error        string     `json:"error,omitempty"`
}

// Checkpoint represents a checkpoint for resume
type Checkpoint struct {
	StageID string                 `json:"stage_id"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// RunMetadata contains run-level metadata
type RunMetadata struct {
	StartedAt  *time.Time `json:"started_at,omitempty"`
	Repository string     `json:"repository,omitempty"`
	Branch     string     `json:"branch,omitempty"`
	User       string     `json:"user,omitempty"`
}

// Status represents stage status values
type Status string

const (
	StatusPending  Status = "pending"
	StatusRunning  Status = "running"
	StatusPass     Status = "pass"
	StatusFail     Status = "fail"
	StatusSkipped  Status = "skipped"
)

// StateManager handles state persistence
type StateManager struct {
	statePath string
	state     *State
}

// NewStateManager creates a new state manager
func NewStateManager(basePath string) *StateManager {
	statePath := filepath.Join(basePath, "state", "state.json")
	return &StateManager{
		statePath: statePath,
	}
}

// NewStateManagerWithPath creates a state manager with a specific path
func NewStateManagerWithPath(statePath string) *StateManager {
	return &StateManager{
		statePath: statePath,
	}
}

// Initialize creates a new state for a run
func (sm *StateManager) Initialize(protocolVersion string) error {
	runID := generateRunID()
	
	state := &State{
		SchemaVersion:   StateVersion,
		RunID:          runID,
		ProtocolVersion: protocolVersion,
		Stages:         make(map[string]*StageState),
		UpdatedAt:      time.Now().UTC(),
		Metadata: &RunMetadata{
			StartedAt: timePtr(time.Now().UTC()),
		},
	}

	sm.state = state
	return sm.Save()
}

// Load loads the state from disk
func (sm *StateManager) Load() error {
	data, err := os.ReadFile(sm.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("state file not found: %w", err)
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to parse state JSON: %w", err)
	}

	// Validate schema version
	if state.SchemaVersion != StateVersion {
		return fmt.Errorf("incompatible state version: %s (expected %s)", state.SchemaVersion, StateVersion)
	}

	sm.state = &state
	return nil
}

// Save persists the state to disk
func (sm *StateManager) Save() error {
	// Ensure directory exists
	dir := filepath.Dir(sm.statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create state directory: %w", err)
	}

	// Update timestamp
	sm.state.UpdatedAt = time.Now().UTC()

	// Marshal with indentation
	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write atomically
	tmpPath := sm.statePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tmpPath, sm.statePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize state file: %w", err)
	}

	return nil
}

// GetState returns the current state
func (sm *StateManager) GetState() *State {
	return sm.state
}

// GetRunID returns the current run ID
func (sm *StateManager) GetRunID() string {
	if sm.state == nil {
		return ""
	}
	return sm.state.RunID
}

// GetStageState returns the state of a stage
func (sm *StateManager) GetStageState(stageID string) (*StageState, error) {
	if sm.state == nil {
		return nil, fmt.Errorf("state not initialized")
	}

	stageState, exists := sm.state.Stages[stageID]
	if !exists {
		return nil, fmt.Errorf("stage not found: %s", stageID)
	}

	return stageState, nil
}

// InitializeStage initializes a stage with pending status
func (sm *StateManager) InitializeStage(stageID string) error {
	if sm.state == nil {
		return fmt.Errorf("state not initialized")
	}

	sm.state.Stages[stageID] = &StageState{
		Status: string(StatusPending),
	}
	return sm.Save()
}

// StartStage marks a stage as running
func (sm *StateManager) StartStage(stageID, artifactPath string) error {
	if sm.state == nil {
		return fmt.Errorf("state not initialized")
	}

	now := time.Now().UTC()
	sm.state.Stages[stageID] = &StageState{
		Status:       string(StatusRunning),
		ArtifactPath: artifactPath,
		StartedAt:    &now,
	}
	sm.state.CurrentStage = stageID
	return sm.Save()
}

// CompleteStage marks a stage as completed with a status
func (sm *StateManager) CompleteStage(stageID string, status Status, summary string) error {
	if sm.state == nil {
		return fmt.Errorf("state not initialized")
	}

	stageState, exists := sm.state.Stages[stageID]
	if !exists {
		return fmt.Errorf("stage not found: %s", stageID)
	}

	now := time.Now().UTC()
	stageState.Status = string(status)
	stageState.CompletedAt = &now

	return sm.Save()
}

// FailStage marks a stage as failed with an error
func (sm *StateManager) FailStage(stageID string, err error) error {
	if sm.state == nil {
		return fmt.Errorf("state not initialized")
	}

	stageState, exists := sm.state.Stages[stageID]
	if !exists {
		return fmt.Errorf("stage not found: %s", stageID)
	}

	now := time.Now().UTC()
	stageState.Status = string(StatusFail)
	stageState.Error = err.Error()
	stageState.CompletedAt = &now

	return sm.Save()
}

// SkipStage marks a stage as skipped
func (sm *StateManager) SkipStage(stageID string) error {
	if sm.state == nil {
		return fmt.Errorf("state not initialized")
	}

	now := time.Now().UTC()
	sm.state.Stages[stageID] = &StageState{
		Status:      string(StatusSkipped),
		CompletedAt: &now,
	}

	return sm.Save()
}

// IsStageComplete returns true if a stage is complete
func (sm *StateManager) IsStageComplete(stageID string) bool {
	if sm.state == nil {
		return false
	}

	stageState, exists := sm.state.Stages[stageID]
	if !exists {
		return false
	}

	return stageState.Status == string(StatusPass) || 
	       stageState.Status == string(StatusFail) || 
	       stageState.Status == string(StatusSkipped)
}

// GetCompletedStages returns all completed stage IDs
func (sm *StateManager) GetCompletedStages() []string {
	if sm.state == nil {
		return []string{}
	}

	completed := []string{}
	for id, state := range sm.state.Stages {
		if state.Status == string(StatusPass) {
			completed = append(completed, id)
		}
	}
	return completed
}

// GetFailedStages returns all failed stage IDs
func (sm *StateManager) GetFailedStages() []string {
	if sm.state == nil {
		return []string{}
	}

	failed := []string{}
	for id, state := range sm.state.Stages {
		if state.Status == string(StatusFail) {
			failed = append(failed, id)
		}
	}
	return failed
}

// GetStageStatuses returns status of all stages
func (sm *StateManager) GetStageStatuses() (map[string]string, error) {
	if sm.state == nil {
		return nil, fmt.Errorf("state not initialized")
	}

	statuses := make(map[string]string)
	for id, stageState := range sm.state.Stages {
		statuses[id] = stageState.Status
	}

	return statuses, nil
}

// SetCheckpoint sets a checkpoint for resume
func (sm *StateManager) SetCheckpoint(stageID string, data map[string]interface{}) error {
	if sm.state == nil {
		return fmt.Errorf("state not initialized")
	}

	sm.state.Checkpoint = &Checkpoint{
		StageID: stageID,
		Data:    data,
	}

	return sm.Save()
}

// ClearCheckpoint clears the current checkpoint
func (sm *StateManager) ClearCheckpoint() error {
	if sm.state == nil {
		return fmt.Errorf("state not initialized")
	}

	sm.state.Checkpoint = nil
	return sm.Save()
}

// GetCheckpoint returns the current checkpoint
func (sm *StateManager) GetCheckpoint() *Checkpoint {
	if sm.state == nil {
		return nil
	}
	return sm.state.Checkpoint
}

// SetMetadata sets run metadata
func (sm *StateManager) SetMetadata(repo, branch, user string) error {
	if sm.state == nil {
		return fmt.Errorf("state not initialized")
	}

	if sm.state.Metadata == nil {
		sm.state.Metadata = &RunMetadata{}
	}

	sm.state.Metadata.Repository = repo
	sm.state.Metadata.Branch = branch
	sm.state.Metadata.User = user

	return sm.Save()
}

// GetMetadata returns run metadata
func (sm *StateManager) GetMetadata() *RunMetadata {
	if sm.state == nil {
		return nil
	}
	return sm.state.Metadata
}

// StateExists checks if a state file exists
func (sm *StateManager) StateExists() bool {
	_, err := os.Stat(sm.statePath)
	return err == nil
}

// GetStatePath returns the state file path
func (sm *StateManager) GetStatePath() string {
	return sm.statePath
}

// generateRunID generates a unique run ID
func generateRunID() string {
	return fmt.Sprintf("run-%s-%s", time.Now().UTC().Format("20060102-150405"), uuid.New().String()[:8])
}

func timePtr(t time.Time) *time.Time {
	return &t
}

// GetStageStatus returns the status of a stage
func (sm *StateManager) GetStageStatus(stageID string) (Status, error) {
	state, err := sm.GetStageState(stageID)
	if err != nil {
		return "", err
	}
	return Status(state.Status), nil
}
