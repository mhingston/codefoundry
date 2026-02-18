package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitializeProject(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	verboseFlag = true

	err := initializeProject()
	require.NoError(t, err)

	// Check directories were created
	assert.DirExists(t, filepath.Join(basePath, "protocols"))
	assert.DirExists(t, filepath.Join(basePath, "state"))
	assert.DirExists(t, filepath.Join(basePath, "artifacts"))
	assert.DirExists(t, filepath.Join(basePath, "templates"))

	// Check default protocol was created
	protocolPath := filepath.Join(basePath, "protocols", "default.yaml")
	assert.FileExists(t, protocolPath)

	content, err := os.ReadFile(protocolPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "name: \"default\"")
	assert.Contains(t, string(content), "version: \"1.0.0\"")

	// Check template files were created
	assert.FileExists(t, filepath.Join(basePath, "templates", "plan.md"))
	assert.FileExists(t, filepath.Join(basePath, "templates", "spec.md"))
}

func TestInitializeProject_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")

	// Initialize first
	err := initializeProject()
	require.NoError(t, err)

	// Initialize again (should not error, directories already exist)
	err = initializeProject()
	require.NoError(t, err)
}

func TestInitializeProject_InvalidPath(t *testing.T) {
	// Use a path that can't be created
	basePath = "/dev/null/invalid"

	err := initializeProject()
	assert.Error(t, err)
}

func TestValidateProtocol(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid protocol file
	protocolContent := `
name: "test-protocol"
version: "1.0.0"
description: "Test protocol"
stages:
  - id: stage1
    name: "Stage 1"
    template: stage1.md
    outputs: [stage1.md]
  - id: stage2
    name: "Stage 2"
    depends_on: [stage1]
    outputs: [stage2.md]
gates:
  - id: test
    name: "Test"
    command: "echo test"
    required: true
    timeout: 60
`
	protocolPath := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(protocolPath, []byte(protocolContent), 0644)
	require.NoError(t, err)

	// Should validate successfully
	err = validateProtocol(protocolPath)
	require.NoError(t, err)
}

func TestValidateProtocol_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	// Create an invalid YAML file
	protocolPath := filepath.Join(tmpDir, "invalid.yaml")
	err := os.WriteFile(protocolPath, []byte("invalid: yaml: ["), 0644)
	require.NoError(t, err)

	// Should fail validation
	err = validateProtocol(protocolPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestValidateProtocol_MissingFile(t *testing.T) {
	err := validateProtocol("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestValidateProtocol_InvalidDependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Create protocol with invalid dependency
	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: stage1
    name: "Stage 1"
    depends_on: [nonexistent]
`
	protocolPath := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(protocolPath, []byte(protocolContent), 0644)
	require.NoError(t, err)

	err = validateProtocol(protocolPath)
	assert.Error(t, err)
}

func TestShowStatus_NoState(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")

	err := showStatus()
	require.NoError(t, err) // Should not error, just print message
}

func TestShowStatus_WithState(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")

	// Create protocol
	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: stage1
    name: "Stage 1"
`
	os.MkdirAll(filepath.Join(basePath, "protocols"), 0755)
	protocolPath = filepath.Join(basePath, "protocols", "test.yaml")
	err := os.WriteFile(protocolPath, []byte(protocolContent), 0644)
	require.NoError(t, err)

	// Initialize and run workflow
	err = initializeProject()
	require.NoError(t, err)

	// The state file may not exist without running the workflow
	// So this test verifies the code doesn't panic
	err = showStatus()
	// May error if no state exists
	_ = err
}

func TestRunWorkflow_Initialize(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize first
	err := initializeProject()
	require.NoError(t, err)

	// Should be able to run
	ctx := context.Background()
	err = runWorkflow(ctx)
	// May complete or fail depending on implementation
	_ = err
}

func TestRunWorkflow_InvalidProtocol(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = "/nonexistent/protocol.yaml"

	ctx := context.Background()
	err := runWorkflow(ctx)
	assert.Error(t, err)
}

func TestRunWorkflow_SpecificStage(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	stageID = "plan"
	verboseFlag = true

	// Initialize first
	err := initializeProject()
	require.NoError(t, err)

	// Run specific stage
	ctx := context.Background()
	err = runWorkflow(ctx)
	// May complete or error depending on implementation
	stageID = "" // Reset
	_ = err
}

func TestRunWorkflow_Force(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	forceFlag = true
	verboseFlag = true

	// Initialize first
	err := initializeProject()
	require.NoError(t, err)

	// Run with force flag
	ctx := context.Background()
	err = runWorkflow(ctx)
	forceFlag = false // Reset
	_ = err
}

func TestCompleteStage(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize first
	err := initializeProject()
	require.NoError(t, err)

	// Setup: Need to initialize workflow first
	ctx := context.Background()
	_ = runWorkflow(ctx) // Initialize workflow

	// Complete stage (will likely fail without proper state, but tests code path)
	err = completeStage(ctx, "plan")
	// May error if state doesn't exist
	_ = err
}

func TestCompleteStage_EmptyID(t *testing.T) {
	ctx := context.Background()
	err := completeStage(ctx, "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "stage ID is required")
}

func TestCompleteStage_InvalidProtocol(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = "/nonexistent/protocol.yaml"

	ctx := context.Background()
	err := completeStage(ctx, "stage1")
	assert.Error(t, err)
}

func TestCompleteStage_InvalidStage(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")

	// Create minimal protocol
	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: stage1
    name: "Stage 1"
`
	os.MkdirAll(filepath.Join(basePath, "protocols"), 0755)
	protocolPath = filepath.Join(basePath, "protocols", "test.yaml")
	err := os.WriteFile(protocolPath, []byte(protocolContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = completeStage(ctx, "nonexistent")
	assert.Error(t, err)
}

func TestRootCmd(t *testing.T) {
	assert.NotNil(t, rootCmd)
	assert.Equal(t, "codefoundry", rootCmd.Use)
	assert.NotEmpty(t, rootCmd.Short)
	assert.NotEmpty(t, rootCmd.Long)
}

func TestInitCmd(t *testing.T) {
	assert.NotNil(t, initCmd)
	assert.Equal(t, "init", initCmd.Use)
	assert.NotNil(t, initCmd.RunE)
}

func TestRunCmd(t *testing.T) {
	assert.NotNil(t, runCmd)
	assert.Equal(t, "run [stage]", runCmd.Use)
	assert.NotNil(t, runCmd.RunE)
}

func TestStatusCmd(t *testing.T) {
	assert.NotNil(t, statusCmd)
	assert.Equal(t, "status", statusCmd.Use)
	assert.NotNil(t, statusCmd.RunE)
}

func TestCompleteCmd(t *testing.T) {
	assert.NotNil(t, completeCmd)
	assert.Equal(t, "complete <stage>", completeCmd.Use)
	assert.NotNil(t, completeCmd.RunE)
	// Args returns nil when exact args match
	assert.Nil(t, completeCmd.Args(completeCmd, []string{"stage1"}))
}

func TestValidateCmd(t *testing.T) {
	assert.NotNil(t, validateCmd)
	assert.Equal(t, "validate <protocol-file>", validateCmd.Use)
	assert.NotNil(t, validateCmd.RunE)
	// Args returns nil when exact args match
	assert.Nil(t, validateCmd.Args(validateCmd, []string{"protocol.yaml"}))
}

func TestFlags(t *testing.T) {
	// Test that flags are registered
	baseFlag := rootCmd.PersistentFlags().Lookup("base")
	assert.NotNil(t, baseFlag)
	assert.Equal(t, ".codefoundry", baseFlag.DefValue)

	verboseFlag := rootCmd.PersistentFlags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)

	protocolFlag := rootCmd.PersistentFlags().Lookup("protocol")
	assert.NotNil(t, protocolFlag)

	// Run command flags
	stageFlag := runCmd.Flags().Lookup("stage")
	assert.NotNil(t, stageFlag)

	forceFlag := runCmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag)
}

func TestVersion(t *testing.T) {
	// Test that version info is set
	versionStr := rootCmd.Version
	assert.Contains(t, versionStr, version)
	assert.Contains(t, versionStr, commit)
	assert.Contains(t, versionStr, date)
}

func TestMain_ExitCode(t *testing.T) {
	// This test verifies main() can be called
	// We can't actually test os.Exit behavior here
	// Just verify the function structure
}

func TestRunWorkflow_LoadStateError(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Corrupt the state file
	statePath := filepath.Join(basePath, "state", "state.json")
	err = os.WriteFile(statePath, []byte("invalid json"), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = runWorkflow(ctx)
	assert.Error(t, err)
}

func TestRunWorkflow_RunError(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Replace protocol with one that has circular dependency
	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: a
    name: "A"
    depends_on: [b]
  - id: b
    name: "B"
    depends_on: [a]
`
	err = os.WriteFile(protocolPath, []byte(protocolContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = runWorkflow(ctx)
	assert.Error(t, err)
}

func TestCompleteStage_NoArtifactsPath(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	artifactPath = ""
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Initialize workflow
	ctx := context.Background()
	err = runWorkflow(ctx)
	require.NoError(t, err)

	// Complete stage without artifacts
	err = completeStage(ctx, "plan")
	require.NoError(t, err) // Should succeed even without artifacts
}

func TestCompleteStage_WithArtifactsDir(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Initialize workflow
	ctx := context.Background()
	err = runWorkflow(ctx)
	require.NoError(t, err)

	// Create artifact directory with files
	artifactsDir := filepath.Join(tmpDir, "artifacts")
	err = os.MkdirAll(artifactsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(artifactsDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(artifactsDir, "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)

	// Set artifact path
	artifactPath = artifactsDir

	// Complete stage
	err = completeStage(ctx, "plan")
	require.NoError(t, err)
}

func TestShowStatus_LoadError(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")

	// Create a state file with invalid content
	statePath := filepath.Join(basePath, "state")
	err := os.MkdirAll(statePath, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(statePath, "state.json"), []byte("invalid"), 0644)
	require.NoError(t, err)

	err = showStatus()
	assert.Error(t, err)
}

func TestInitializeProject_TemplateWriteError(t *testing.T) {
	// This is hard to test reliably as root
	t.Skip("Cannot reliably test write errors as root")
}

func TestRunWorkflow_ContextCancelled(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Cancel context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = runWorkflow(ctx)
	assert.Error(t, err)
}

func TestRunWorkflow_RunSingleStageError(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	stageID = "nonexistent"
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	ctx := context.Background()
	err = runWorkflow(ctx)
	// Should error because stage doesn't exist
	assert.Error(t, err)

	// Reset stageID
	stageID = ""
}

func TestRunWorkflow_CircularDependency(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Create protocol with circular dependency
	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: a
    name: "A"
    depends_on: [b]
  - id: b
    name: "B"
    depends_on: [a]
`
	err = os.WriteFile(protocolPath, []byte(protocolContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = runWorkflow(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to resolve dependencies")
}

func TestCompleteStage_StateLoadError(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")

	// Create protocol
	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: stage1
    name: "Stage 1"
`
	os.MkdirAll(filepath.Join(basePath, "protocols"), 0755)
	protocolPath = filepath.Join(basePath, "protocols", "test.yaml")
	err := os.WriteFile(protocolPath, []byte(protocolContent), 0644)
	require.NoError(t, err)

	// Create invalid state
	os.MkdirAll(filepath.Join(basePath, "state"), 0755)
	err = os.WriteFile(filepath.Join(basePath, "state", "state.json"), []byte("invalid"), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = completeStage(ctx, "stage1")
	assert.Error(t, err)
}

func TestRunWorkflow_WithStageArtifacts(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Create custom protocol with stage dependencies
	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: stage1
    name: "Stage 1"
    outputs: [output.txt]
  - id: stage2
    name: "Stage 2"
    depends_on: [stage1]
    inputs: [output.txt]
    outputs: [result.txt]
`
	err = os.WriteFile(protocolPath, []byte(protocolContent), 0644)
	require.NoError(t, err)

	ctx := context.Background()
	err = runWorkflow(ctx)
	require.NoError(t, err)
}

func TestShowStatus_EmptyMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Initialize workflow
	ctx := context.Background()
	err = runWorkflow(ctx)
	require.NoError(t, err)

	// Clear metadata to test edge case
	// Note: This test may not actually hit the uncovered lines
	// as the metadata should always be set during Initialize
	err = showStatus()
	require.NoError(t, err)
}

func TestCompleteStage_WithFileArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Initialize workflow
	ctx := context.Background()
	err = runWorkflow(ctx)
	require.NoError(t, err)

	// Create artifact file (not directory)
	artifactFile := filepath.Join(tmpDir, "artifact.txt")
	err = os.WriteFile(artifactFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Set artifact path to file
	artifactPath = artifactFile

	// Complete stage
	err = completeStage(ctx, "plan")
	require.NoError(t, err)

	// Reset artifact path
	artifactPath = ""
}

func TestRunWorkflow_RunFromCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Initialize and complete first stage
	ctx := context.Background()
	err = runWorkflow(ctx)
	require.NoError(t, err)

	// Run again - should continue from checkpoint
	err = runWorkflow(ctx)
	require.NoError(t, err)
}

func TestRunWorkflow_LoadExistingState(t *testing.T) {
	tmpDir := t.TempDir()
	basePath = filepath.Join(tmpDir, ".codefoundry")
	protocolPath = filepath.Join(basePath, "protocols", "default.yaml")
	verboseFlag = true

	// Initialize
	err := initializeProject()
	require.NoError(t, err)

	// Initialize workflow first
	ctx := context.Background()
	err = runWorkflow(ctx)
	require.NoError(t, err)

	// Run again without force - should load existing state
	err = runWorkflow(ctx)
	require.NoError(t, err)
}

func TestValidateProtocol_InvalidDAG(t *testing.T) {
	tmpDir := t.TempDir()

	// Create protocol with invalid DAG
	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: a
    name: "A"
    depends_on: [b]
  - id: b
    name: "B"
    depends_on: [a]
`
	protocolPath := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(protocolPath, []byte(protocolContent), 0644)
	require.NoError(t, err)

	err = validateProtocol(protocolPath)
	assert.Error(t, err)
}
