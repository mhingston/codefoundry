package stage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewStateManager(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	assert.NotNil(t, sm)
	assert.Nil(t, sm.state) // State is nil before initialization
	assert.Contains(t, sm.statePath, filepath.Join(tmpDir, "state", "state.json"))
}

func TestNewStateManagerWithPath(t *testing.T) {
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom", "state.json")
	sm := NewStateManagerWithPath(customPath)

	assert.NotNil(t, sm)
	assert.Equal(t, customPath, sm.statePath)
}

func TestStateManager_Initialize(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	assert.NotNil(t, sm.state)
	assert.Equal(t, StateVersion, sm.state.SchemaVersion)
	assert.Equal(t, "1.0.0", sm.state.ProtocolVersion)
	assert.NotEmpty(t, sm.state.RunID)
	assert.NotNil(t, sm.state.Stages)
	assert.NotNil(t, sm.state.Metadata)
	assert.NotNil(t, sm.state.Metadata.StartedAt)

	// Verify file was created
	assert.FileExists(t, sm.statePath)
}

func TestStateManager_Load(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Initialize and save
	err := sm.Initialize("1.0.0")
	require.NoError(t, err)
	runID := sm.state.RunID

	// Load in new instance
	sm2 := NewStateManagerWithPath(sm.statePath)
	err = sm2.Load()
	require.NoError(t, err)

	assert.Equal(t, runID, sm2.state.RunID)
	assert.Equal(t, "1.0.0", sm2.state.ProtocolVersion)
}

func TestStateManager_Load_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManagerWithPath(filepath.Join(tmpDir, "nonexistent", "state.json"))

	err := sm.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestStateManager_Load_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Write invalid JSON
	err := os.WriteFile(statePath, []byte("not valid json"), 0644)
	require.NoError(t, err)

	sm := NewStateManagerWithPath(statePath)
	err = sm.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse state JSON")
}

func TestStateManager_Load_WrongVersion(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "state.json")

	// Write state with wrong version
	stateContent := `{"schema_version": "wrong.version", "run_id": "test"}`
	err := os.WriteFile(statePath, []byte(stateContent), 0644)
	require.NoError(t, err)

	sm := NewStateManagerWithPath(statePath)
	err = sm.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "incompatible state version")
}

func TestStateManager_Save(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// Verify file exists
	assert.FileExists(t, sm.statePath)

	// Read and verify content
	data, err := os.ReadFile(sm.statePath)
	require.NoError(t, err)
	assert.Contains(t, string(data), StateVersion)
}

func TestStateManager_GetState(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Before initialization
	assert.Nil(t, sm.GetState())

	// After initialization
	err := sm.Initialize("1.0.0")
	require.NoError(t, err)
	assert.NotNil(t, sm.GetState())
}

func TestStateManager_GetRunID(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Before initialization
	assert.Empty(t, sm.GetRunID())

	// After initialization
	err := sm.Initialize("1.0.0")
	require.NoError(t, err)
	assert.NotEmpty(t, sm.GetRunID())
}

func TestStateManager_InitializeStage(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Before initialization
	err := sm.InitializeStage("stage1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")

	// After initialization
	err = sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.InitializeStage("stage1")
	require.NoError(t, err)

	stageState, err := sm.GetStageState("stage1")
	require.NoError(t, err)
	assert.Equal(t, string(StatusPending), stageState.Status)
}

func TestStateManager_StartStage(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.InitializeStage("stage1")
	require.NoError(t, err)

	err = sm.StartStage("stage1", "/path/to/artifacts")
	require.NoError(t, err)

	stageState, err := sm.GetStageState("stage1")
	require.NoError(t, err)
	assert.Equal(t, string(StatusRunning), stageState.Status)
	assert.Equal(t, "/path/to/artifacts", stageState.ArtifactPath)
	assert.NotNil(t, stageState.StartedAt)
}

func TestStateManager_CompleteStage(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.InitializeStage("stage1")
	require.NoError(t, err)

	err = sm.CompleteStage("stage1", StatusPass, "Completed successfully")
	require.NoError(t, err)

	stageState, err := sm.GetStageState("stage1")
	require.NoError(t, err)
	assert.Equal(t, string(StatusPass), stageState.Status)
	assert.NotNil(t, stageState.CompletedAt)
}

func TestStateManager_CompleteStage_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.CompleteStage("nonexistent", StatusPass, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stage not found")
}

func TestStateManager_FailStage(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.InitializeStage("stage1")
	require.NoError(t, err)

	testErr := assert.AnError
	err = sm.FailStage("stage1", testErr)
	require.NoError(t, err)

	stageState, err := sm.GetStageState("stage1")
	require.NoError(t, err)
	assert.Equal(t, string(StatusFail), stageState.Status)
	assert.Equal(t, testErr.Error(), stageState.Error)
}

func TestStateManager_SkipStage(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.InitializeStage("stage1")
	require.NoError(t, err)

	err = sm.SkipStage("stage1")
	require.NoError(t, err)

	stageState, err := sm.GetStageState("stage1")
	require.NoError(t, err)
	assert.Equal(t, string(StatusSkipped), stageState.Status)
	assert.NotNil(t, stageState.CompletedAt)
}

func TestStateManager_IsStageComplete(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Before initialization
	assert.False(t, sm.IsStageComplete("stage1"))

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// Stage not initialized
	assert.False(t, sm.IsStageComplete("stage1"))

	// Pending stage
	err = sm.InitializeStage("stage1")
	require.NoError(t, err)
	assert.False(t, sm.IsStageComplete("stage1"))

	// Running stage
	err = sm.StartStage("stage1", "/path")
	require.NoError(t, err)
	assert.False(t, sm.IsStageComplete("stage1"))

	// Completed stage
	err = sm.CompleteStage("stage1", StatusPass, "")
	require.NoError(t, err)
	assert.True(t, sm.IsStageComplete("stage1"))

	// Failed stage
	err = sm.InitializeStage("stage2")
	require.NoError(t, err)
	err = sm.FailStage("stage2", assert.AnError)
	require.NoError(t, err)
	assert.True(t, sm.IsStageComplete("stage2"))

	// Skipped stage
	err = sm.InitializeStage("stage3")
	require.NoError(t, err)
	err = sm.SkipStage("stage3")
	require.NoError(t, err)
	assert.True(t, sm.IsStageComplete("stage3"))
}

func TestStateManager_GetCompletedStages(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Before initialization
	completed := sm.GetCompletedStages()
	assert.Empty(t, completed)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// No completed stages
	completed = sm.GetCompletedStages()
	assert.Empty(t, completed)

	// Complete a stage
	err = sm.InitializeStage("stage1")
	require.NoError(t, err)
	err = sm.CompleteStage("stage1", StatusPass, "")
	require.NoError(t, err)

	completed = sm.GetCompletedStages()
	assert.Len(t, completed, 1)
	assert.Contains(t, completed, "stage1")

	// Fail a stage (should not be in completed)
	err = sm.InitializeStage("stage2")
	require.NoError(t, err)
	err = sm.FailStage("stage2", assert.AnError)
	require.NoError(t, err)

	completed = sm.GetCompletedStages()
	assert.Len(t, completed, 1) // Still only 1
}

func TestStateManager_GetFailedStages(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// Pass a stage
	err = sm.InitializeStage("stage1")
	require.NoError(t, err)
	err = sm.CompleteStage("stage1", StatusPass, "")
	require.NoError(t, err)

	// Fail a stage
	err = sm.InitializeStage("stage2")
	require.NoError(t, err)
	err = sm.FailStage("stage2", assert.AnError)
	require.NoError(t, err)

	failed := sm.GetFailedStages()
	assert.Len(t, failed, 1)
	assert.Contains(t, failed, "stage2")
}

func TestStateManager_GetStageStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Before initialization
	_, err := sm.GetStageStatuses()
	assert.Error(t, err)

	err = sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.InitializeStage("stage1")
	require.NoError(t, err)
	err = sm.InitializeStage("stage2")
	require.NoError(t, err)

	statuses, err := sm.GetStageStatuses()
	require.NoError(t, err)
	assert.Len(t, statuses, 2)
	assert.Equal(t, string(StatusPending), statuses["stage1"])
	assert.Equal(t, string(StatusPending), statuses["stage2"])
}

func TestStateManager_SetCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	checkpointData := map[string]interface{}{
		"step":  5,
		"value": "test",
	}

	err = sm.SetCheckpoint("stage1", checkpointData)
	require.NoError(t, err)

	checkpoint := sm.GetCheckpoint()
	require.NotNil(t, checkpoint)
	assert.Equal(t, "stage1", checkpoint.StageID)
	assert.Equal(t, checkpointData, checkpoint.Data)
}

func TestStateManager_ClearCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// Set checkpoint
	err = sm.SetCheckpoint("stage1", map[string]interface{}{"step": 1})
	require.NoError(t, err)
	assert.NotNil(t, sm.GetCheckpoint())

	// Clear checkpoint
	err = sm.ClearCheckpoint()
	require.NoError(t, err)
	assert.Nil(t, sm.GetCheckpoint())
}

func TestStateManager_SetMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.SetMetadata("repo-url", "main", "user123")
	require.NoError(t, err)

	metadata := sm.GetMetadata()
	require.NotNil(t, metadata)
	assert.Equal(t, "repo-url", metadata.Repository)
	assert.Equal(t, "main", metadata.Branch)
	assert.Equal(t, "user123", metadata.User)
}

func TestStateManager_StateExists(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Before save
	assert.False(t, sm.StateExists())

	// After save
	err := sm.Initialize("1.0.0")
	require.NoError(t, err)
	assert.True(t, sm.StateExists())
}

func TestStateManager_GetStatePath(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	expectedPath := filepath.Join(tmpDir, "state", "state.json")
	assert.Equal(t, expectedPath, sm.GetStatePath())
}

func TestStateManager_GetStageState_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	_, err := sm.GetStageState("stage1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_GetStageState_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	_, err = sm.GetStageState("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stage not found")
}

func TestStateManager_GetStageStatus(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.InitializeStage("stage1")
	require.NoError(t, err)

	status, err := sm.GetStageStatus("stage1")
	require.NoError(t, err)
	assert.Equal(t, StatusPending, status)
}

func TestStatus_Constants(t *testing.T) {
	assert.Equal(t, Status("pending"), StatusPending)
	assert.Equal(t, Status("running"), StatusRunning)
	assert.Equal(t, Status("pass"), StatusPass)
	assert.Equal(t, Status("fail"), StatusFail)
	assert.Equal(t, Status("skipped"), StatusSkipped)
}

func Test_generateRunID(t *testing.T) {
	runID1 := generateRunID()
	runID2 := generateRunID()

	// Should be unique
	assert.NotEqual(t, runID1, runID2)

	// Should start with "run-"
	assert.Contains(t, runID1, "run-")
	assert.Contains(t, runID2, "run-")
}

func Test_timePtr(t *testing.T) {
	now := time.Now()
	ptr := timePtr(now)
	assert.NotNil(t, ptr)
	assert.Equal(t, now, *ptr)
}

func TestStateManager_SetCheckpoint_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.SetCheckpoint("stage1", map[string]interface{}{"step": 1})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_ClearCheckpoint_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.ClearCheckpoint()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_GetCheckpoint_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	checkpoint := sm.GetCheckpoint()
	assert.Nil(t, checkpoint)
}

func TestStateManager_SetMetadata_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.SetMetadata("repo", "main", "user")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_GetMetadata_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	metadata := sm.GetMetadata()
	assert.Nil(t, metadata)
}

func TestStateManager_GetMetadata_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)
	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// Clear metadata
	sm.state.Metadata = nil

	metadata := sm.GetMetadata()
	assert.Nil(t, metadata)
}

func TestStateManager_GetCheckpoint_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)
	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// No checkpoint set
	checkpoint := sm.GetCheckpoint()
	assert.Nil(t, checkpoint)
}

func TestStateManager_Save_MarshalError(t *testing.T) {
	// This test is hard to trigger as JSON marshaling rarely fails
	// Skip this test
	t.Skip("Cannot reliably trigger JSON marshal error")
}

func TestStateManager_FailStage_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)
	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	err = sm.FailStage("nonexistent", assert.AnError)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stage not found")
}

func TestStateManager_FailStage_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.FailStage("stage1", assert.AnError)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_SkipStage_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.SkipStage("stage1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_StartStage_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.StartStage("stage1", "/path")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_CompleteStage_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.CompleteStage("stage1", StatusPass, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_InitializeStage_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.InitializeStage("stage1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_GetFailedStages_Mixed(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)
	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// Pass, fail, skip stages
	err = sm.InitializeStage("pass-stage")
	require.NoError(t, err)
	err = sm.StartStage("pass-stage", "/path")
	require.NoError(t, err)
	err = sm.CompleteStage("pass-stage", StatusPass, "")
	require.NoError(t, err)

	err = sm.InitializeStage("fail-stage")
	require.NoError(t, err)
	err = sm.StartStage("fail-stage", "/path")
	require.NoError(t, err)
	err = sm.FailStage("fail-stage", assert.AnError)
	require.NoError(t, err)

	err = sm.InitializeStage("skip-stage")
	require.NoError(t, err)
	err = sm.SkipStage("skip-stage")
	require.NoError(t, err)

	failed := sm.GetFailedStages()
	assert.Len(t, failed, 1)
	assert.Contains(t, failed, "fail-stage")
}

func TestStateManager_Load_ReadError(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	// Create a directory instead of file
	err := os.MkdirAll(sm.statePath, 0755)
	require.NoError(t, err)

	err = sm.Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read state file")
}

func TestStateManager_Save_StateDirError(t *testing.T) {
	// This is hard to test as root
	t.Skip("Cannot reliably test MkdirAll error as root")
}

func TestStateManager_Save_RenameError(t *testing.T) {
	// This is hard to test reliably
	t.Skip("Cannot reliably test file rename error")
}

func TestStateManager_Save_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// Test concurrent saves don't panic
	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			sm.Save()
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}

func TestStateManager_GetStageStatus_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	_, err = sm.GetStageStatus("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stage not found")
}

func TestStateManager_GetStageStatus_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	_, err := sm.GetStageStatus("stage1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_GetCompletedStages_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	completed := sm.GetCompletedStages()
	assert.Empty(t, completed)
}

func TestStateManager_GetFailedStages_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	failed := sm.GetFailedStages()
	assert.Empty(t, failed)
}

func TestStateManager_GetStageStatuses_NotInitialized(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	_, err := sm.GetStageStatuses()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "state not initialized")
}

func TestStateManager_Save_AfterMultipleOperations(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)

	// Perform multiple operations
	err = sm.InitializeStage("stage1")
	require.NoError(t, err)
	
	err = sm.StartStage("stage1", "/path")
	require.NoError(t, err)
	
	err = sm.CompleteStage("stage1", StatusPass, "done")
	require.NoError(t, err)
	
	// Save should still work
	err = sm.Save()
	require.NoError(t, err)
	
	// Verify state
	state := sm.GetState()
	require.NotNil(t, state)
	assert.Equal(t, string(StatusPass), state.Stages["stage1"].Status)
}

func TestStateManager_Initialize_MultipleCalls(t *testing.T) {
	tmpDir := t.TempDir()
	sm := NewStateManager(tmpDir)

	err := sm.Initialize("1.0.0")
	require.NoError(t, err)
	
	firstRunID := sm.GetRunID()
	
	// Initialize again
	err = sm.Initialize("2.0.0")
	require.NoError(t, err)
	
	secondRunID := sm.GetRunID()
	
	// Should have new run ID
	assert.NotEqual(t, firstRunID, secondRunID)
	assert.Equal(t, "2.0.0", sm.GetState().ProtocolVersion)
}
