package stage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CheckpointManager handles checkpoint creation and restoration
type CheckpointManager struct {
	stateManager *StateManager
	basePath     string
}

// NewCheckpointManager creates a new checkpoint manager
func NewCheckpointManager(stateManager *StateManager, basePath string) *CheckpointManager {
	return &CheckpointManager{
		stateManager: stateManager,
		basePath:     basePath,
	}
}

// CheckpointData holds checkpoint information for a stage
type CheckpointData struct {
	StageID  string                 `json:"stage_id"`
	RunID    string                 `json:"run_id"`
	Step     int                    `json:"step,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Create creates a checkpoint for a stage
func (cm *CheckpointManager) Create(stageID string, step int, data map[string]interface{}) error {
	checkpoint := &CheckpointData{
		StageID: stageID,
		RunID:   cm.stateManager.GetRunID(),
		Step:    step,
		Data:    data,
	}

	// Save to file for durability
	checkpointPath := cm.getCheckpointPath(stageID)
	if err := cm.saveCheckpointFile(checkpointPath, checkpoint); err != nil {
		return fmt.Errorf("failed to save checkpoint file: %w", err)
	}

	// Also update state
	if err := cm.stateManager.SetCheckpoint(stageID, data); err != nil {
		return fmt.Errorf("failed to set checkpoint in state: %w", err)
	}

	return nil
}

// Restore restores a checkpoint for a stage
func (cm *CheckpointManager) Restore(stageID string) (*CheckpointData, error) {
	checkpointPath := cm.getCheckpointPath(stageID)

	// Try to load from file first
	checkpoint, err := cm.loadCheckpointFile(checkpointPath)
	if err != nil {
		// Fall back to state checkpoint
		stateCheckpoint := cm.stateManager.GetCheckpoint()
		if stateCheckpoint == nil || stateCheckpoint.StageID != stageID {
			return nil, fmt.Errorf("no checkpoint found for stage: %s", stageID)
		}

		checkpoint = &CheckpointData{
			StageID: stateCheckpoint.StageID,
			RunID:   cm.stateManager.GetRunID(),
			Data:    stateCheckpoint.Data,
		}
	}

	return checkpoint, nil
}

// Delete removes a checkpoint for a stage
func (cm *CheckpointManager) Delete(stageID string) error {
	checkpointPath := cm.getCheckpointPath(stageID)

	// Remove checkpoint file if it exists
	if err := os.Remove(checkpointPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete checkpoint file: %w", err)
	}

	// Clear from state
	currentCheckpoint := cm.stateManager.GetCheckpoint()
	if currentCheckpoint != nil && currentCheckpoint.StageID == stageID {
		if err := cm.stateManager.ClearCheckpoint(); err != nil {
			return fmt.Errorf("failed to clear checkpoint from state: %w", err)
		}
	}

	return nil
}

// Exists checks if a checkpoint exists for a stage
func (cm *CheckpointManager) Exists(stageID string) bool {
	checkpointPath := cm.getCheckpointPath(stageID)
	_, err := os.Stat(checkpointPath)
	if err == nil {
		return true
	}

	// Check state checkpoint
	stateCheckpoint := cm.stateManager.GetCheckpoint()
	return stateCheckpoint != nil && stateCheckpoint.StageID == stageID
}

// List returns all checkpoint files for the current run
func (cm *CheckpointManager) List() ([]string, error) {
	checkpointDir := filepath.Join(cm.basePath, "state", "checkpoints")

	entries, err := os.ReadDir(checkpointDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list checkpoints: %w", err)
	}

	checkpoints := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			checkpoints = append(checkpoints, entry.Name())
		}
	}

	return checkpoints, nil
}

// Cleanup removes all checkpoints for the current run
func (cm *CheckpointManager) Cleanup() error {
	checkpointDir := filepath.Join(cm.basePath, "state", "checkpoints")

	// Remove checkpoint directory
	if err := os.RemoveAll(checkpointDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to cleanup checkpoints: %w", err)
	}

	// Clear state checkpoint
	if err := cm.stateManager.ClearCheckpoint(); err != nil {
		return fmt.Errorf("failed to clear checkpoint from state: %w", err)
	}

	return nil
}

// SaveStep saves the current step progress
func (cm *CheckpointManager) SaveStep(stageID string, step int, data map[string]interface{}) error {
	return cm.Create(stageID, step, data)
}

// ResumeFrom attempts to resume a stage from its last checkpoint
func (cm *CheckpointManager) ResumeFrom(stageID string) (bool, *CheckpointData, error) {
	if !cm.Exists(stageID) {
		return false, nil, nil
	}

	checkpoint, err := cm.Restore(stageID)
	if err != nil {
		return false, nil, err
	}

	return true, checkpoint, nil
}

// GetLastStep returns the last saved step for a stage
func (cm *CheckpointManager) GetLastStep(stageID string) (int, error) {
	checkpoint, err := cm.Restore(stageID)
	if err != nil {
		return 0, err
	}

	return checkpoint.Step, nil
}

// UpdateCheckpointData updates the data for an existing checkpoint
func (cm *CheckpointManager) UpdateCheckpointData(stageID string, data map[string]interface{}) error {
	checkpoint, err := cm.Restore(stageID)
	if err != nil {
		return err
	}

	// Merge new data with existing
	for key, value := range data {
		checkpoint.Data[key] = value
	}

	// Save updated checkpoint
	checkpointPath := cm.getCheckpointPath(stageID)
	if err := cm.saveCheckpointFile(checkpointPath, checkpoint); err != nil {
		return fmt.Errorf("failed to save updated checkpoint: %w", err)
	}

	// Update state checkpoint
	if err := cm.stateManager.SetCheckpoint(stageID, checkpoint.Data); err != nil {
		return fmt.Errorf("failed to update checkpoint in state: %w", err)
	}

	return nil
}

// getCheckpointPath returns the file path for a stage's checkpoint
func (cm *CheckpointManager) getCheckpointPath(stageID string) string {
	return filepath.Join(cm.basePath, "state", "checkpoints", fmt.Sprintf("%s.json", stageID))
}

// saveCheckpointFile saves a checkpoint to a file
func (cm *CheckpointManager) saveCheckpointFile(path string, checkpoint *CheckpointData) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	// Write atomically
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to finalize checkpoint file: %w", err)
	}

	return nil
}

// loadCheckpointFile loads a checkpoint from a file
func (cm *CheckpointManager) loadCheckpointFile(path string) (*CheckpointData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	var checkpoint CheckpointData
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to parse checkpoint JSON: %w", err)
	}

	return &checkpoint, nil
}

// CanResume checks if the run can be resumed from a checkpoint
func (cm *CheckpointManager) CanResume() (bool, string, error) {
	// Check if state exists
	if !cm.stateManager.StateExists() {
		return false, "", nil
	}

	// Load state
	if err := cm.stateManager.Load(); err != nil {
		return false, "", fmt.Errorf("failed to load state: %w", err)
	}

	// Check if there's a current running stage
	state := cm.stateManager.GetState()
	if state == nil {
		return false, "", nil
	}

	// Check for current stage that might have a checkpoint
	if state.CurrentStage != "" {
		stageState, err := cm.stateManager.GetStageState(state.CurrentStage)
		if err == nil && stageState.Status == string(StatusRunning) {
			// Check if checkpoint exists
			if cm.Exists(state.CurrentStage) {
				return true, state.CurrentStage, nil
			}
		}
	}

	// Check for any checkpoint
	checkpoints, err := cm.List()
	if err != nil {
		return false, "", err
	}

	if len(checkpoints) > 0 {
		// Get the most recent one based on file modification time
		checkpointDir := filepath.Join(cm.basePath, "state", "checkpoints")
		var mostRecent string
		var mostRecentTime int64

		for _, cp := range checkpoints {
			info, err := os.Stat(filepath.Join(checkpointDir, cp))
			if err != nil {
				continue
			}
			if info.ModTime().Unix() > mostRecentTime {
				mostRecentTime = info.ModTime().Unix()
				mostRecent = cp
			}
		}

		if mostRecent != "" {
			stageID := mostRecent[:len(mostRecent)-5] // Remove .json
			return true, stageID, nil
		}
	}

	return false, "", nil
}

// ResumeHandler is called when resuming from a checkpoint
type ResumeHandler func(checkpoint *CheckpointData) error

// ResumeOrStart resumes from checkpoint or starts fresh
func (cm *CheckpointManager) ResumeOrStart(stageID string, resumeHandler ResumeHandler, freshHandler func() error) error {
	canResume, checkpoint, err := cm.ResumeFrom(stageID)
	if err != nil {
		return err
	}

	if canResume {
		return resumeHandler(checkpoint)
	}

	return freshHandler()
}
