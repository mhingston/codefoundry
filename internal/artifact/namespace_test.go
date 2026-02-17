package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNamespace(t *testing.T) {
	ns := NewNamespace("/base/path", "run-123")
	
	assert.NotNil(t, ns)
	assert.Equal(t, "/base/path", ns.GetBasePath())
	assert.Equal(t, "run-123", ns.GetRunID())
}

func TestNamespace_StagePath(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	path := ns.StagePath("plan")
	assert.Equal(t, "/base/artifacts/run-123/plan", path)
}

func TestNamespace_ArtifactPath(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	path := ns.ArtifactPath("plan", "plan.md")
	assert.Equal(t, "/base/artifacts/run-123/plan/plan.md", path)
}

func TestNamespace_StatusPath(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	path := ns.StatusPath("plan")
	assert.Equal(t, "/base/artifacts/run-123/plan/status.json", path)
}

func TestNamespace_GateReportPath(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	path := ns.GateReportPath("verify", "test")
	assert.Equal(t, "/base/artifacts/run-123/verify/test.json", path)
}

func TestNamespace_StatePath(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	path := ns.StatePath()
	assert.Equal(t, "/base/state/state.json", path)
}

func TestNamespace_RunPath(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	path := ns.RunPath()
	assert.Equal(t, "/base/artifacts/run-123", path)
}

func TestNamespace_ResolveInputPath_Absolute(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	// Absolute path should be returned as-is
	path := ns.ResolveInputPath("plan", "/absolute/path/file.txt")
	assert.Equal(t, "/absolute/path/file.txt", path)
}

func TestNamespace_ResolveInputPath_RelativeWithDot(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	// Paths starting with ./ should be returned as-is
	path := ns.ResolveInputPath("plan", "./file.txt")
	assert.Equal(t, "./file.txt", path)
	
	path = ns.ResolveInputPath("plan", "../other/file.txt")
	assert.Equal(t, "../other/file.txt", path)
}

func TestNamespace_ResolveInputPath_Relative(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	// Relative paths should be resolved to stage artifact directory
	path := ns.ResolveInputPath("plan", "input.md")
	assert.Equal(t, "/base/artifacts/run-123/plan/input.md", path)
}

func TestValidateArtifactName_Valid(t *testing.T) {
	validNames := []string{
		"file.txt",
		"path/to/file.md",
		"file-with-dashes.txt",
		"file_with_underscores.txt",
	}
	
	for _, name := range validNames {
		err := ValidateArtifactName(name)
		assert.NoError(t, err, "Expected %s to be valid", name)
	}
}

func TestValidateArtifactName_Invalid(t *testing.T) {
	tests := []struct {
		name        string
		expectedErr string
	}{
		{"", "cannot be empty"},
		{"../outside/file.txt", "path traversal"},
		{"/absolute/path.txt", "absolute path"},
	}
	
	for _, tt := range tests {
		err := ValidateArtifactName(tt.name)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), tt.expectedErr)
	}
}

func TestNamespace_GetRelativePath(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	fullPath := "/base/artifacts/run-123/plan/status.json"
	rel, err := ns.GetRelativePath("plan", fullPath)
	
	require.NoError(t, err)
	assert.Equal(t, "status.json", rel)
}

func TestNamespace_GetRelativePath_DifferentStage(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	fullPath := "/base/artifacts/run-123/spec/output.md"
	rel, err := ns.GetRelativePath("plan", fullPath)
	
	require.NoError(t, err)
	assert.Equal(t, "../spec/output.md", rel)
}

func TestNamespace_PathTraversal(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	// Try path traversal in stage ID
	path := ns.StagePath("../../../etc/passwd")
	// Path is constructed - caller should validate with ValidateArtifactName
	assert.NotEmpty(t, path)
}

func TestNamespace_ListStages(t *testing.T) {
	tmpDir := t.TempDir()
	ns := NewNamespace(tmpDir, "run-123")
	
	// Empty list when no stages exist
	stages, err := ns.ListStages()
	require.NoError(t, err)
	assert.Empty(t, stages)
	
	// Create some stage directories
	err = os.MkdirAll(ns.StagePath("stage1"), 0755)
	require.NoError(t, err)
	err = os.MkdirAll(ns.StagePath("stage2"), 0755)
	require.NoError(t, err)
	
	// List should return stages
	stages, err = ns.ListStages()
	require.NoError(t, err)
	assert.Len(t, stages, 2)
	assert.Contains(t, stages, "stage1")
	assert.Contains(t, stages, "stage2")
}

func TestNamespace_CreateStageDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	ns := NewNamespace(tmpDir, "run-123")
	
	err := ns.CreateStageDirectory("new-stage")
	require.NoError(t, err)
	
	// Verify directory exists
	stagePath := ns.StagePath("new-stage")
	info, err := os.Stat(stagePath)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNamespace_CreateStageDirectory_AlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	ns := NewNamespace(tmpDir, "run-123")
	
	// Create first time
	err := ns.CreateStageDirectory("existing-stage")
	require.NoError(t, err)
	
	// Create again - should not error
	err = ns.CreateStageDirectory("existing-stage")
	require.NoError(t, err)
}

func TestNamespace_ListStages_WithFiles(t *testing.T) {
	tmpDir := t.TempDir()
	ns := NewNamespace(tmpDir, "run-123")
	
	// Create stage directory with files
	stagePath := ns.StagePath("stage1")
	err := os.MkdirAll(stagePath, 0755)
	require.NoError(t, err)
	
	// Create a file in the run directory
	err = os.WriteFile(filepath.Join(ns.RunPath(), "file.txt"), []byte("content"), 0644)
	require.NoError(t, err)
	
	// List should only return directories
	stages, err := ns.ListStages()
	require.NoError(t, err)
	assert.Len(t, stages, 1)
	assert.Contains(t, stages, "stage1")
}

func TestNamespace_GetRelativePath_SameDir(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	// Path in same directory
	fullPath := ns.ArtifactPath("plan", "file.txt")
	rel, err := ns.GetRelativePath("plan", fullPath)
	require.NoError(t, err)
	assert.Equal(t, "file.txt", rel)
}

func TestNamespace_GetRelativePath_ParentDir(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	
	// Path in parent directory
	fullPath := ns.ArtifactPath("..", "file.txt")
	rel, err := ns.GetRelativePath("plan", fullPath)
	require.NoError(t, err)
	// Should give relative path from plan to parent
	assert.NotEmpty(t, rel)
}
