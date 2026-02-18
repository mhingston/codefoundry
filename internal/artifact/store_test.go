package artifact

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStore(t *testing.T) (*Store, string) {
	tmpDir := t.TempDir()
	ns := NewNamespace(tmpDir, "test-run")
	store := NewStore(ns)
	return store, tmpDir
}

func TestNewStore(t *testing.T) {
	ns := NewNamespace("/base", "run-123")
	store := NewStore(ns)

	assert.NotNil(t, store)
	assert.NotNil(t, store.namespace)
	assert.NotNil(t, store.hasher)
	assert.Equal(t, ns, store.Namespace())
}

func TestStore_Write(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	content := []byte("test content")
	err := store.Write("plan", "test.txt", content)
	require.NoError(t, err)

	// Verify file exists
	expectedPath := filepath.Join(tmpDir, "artifacts", "test-run", "plan", "test.txt")
	_, err = os.Stat(expectedPath)
	assert.NoError(t, err)

	// Verify content
	readContent, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, content, readContent)
}

func TestStore_Write_InvalidName(t *testing.T) {
	store, _ := setupTestStore(t)

	err := store.Write("plan", "../outside.txt", []byte("content"))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestStore_WriteString(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	content := "string content"
	err := store.WriteString("plan", "string.txt", content)
	require.NoError(t, err)

	expectedPath := filepath.Join(tmpDir, "artifacts", "test-run", "plan", "string.txt")
	readContent, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, content, string(readContent))
}

func TestStore_WriteJSON(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	data := map[string]string{"key": "value", "foo": "bar"}
	err := store.WriteJSON("plan", "data.json", data)
	require.NoError(t, err)

	expectedPath := filepath.Join(tmpDir, "artifacts", "test-run", "plan", "data.json")
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)

	// Verify it's valid JSON
	assert.Contains(t, string(content), `"key": "value"`)
	assert.Contains(t, string(content), `"foo": "bar"`)
}

func TestStore_Read(t *testing.T) {
	store, _ := setupTestStore(t)

	content := []byte("test content")
	err := store.Write("plan", "read.txt", content)
	require.NoError(t, err)

	readContent, err := store.Read("plan", "read.txt")
	require.NoError(t, err)
	assert.Equal(t, content, readContent)
}

func TestStore_Read_NotFound(t *testing.T) {
	store, _ := setupTestStore(t)

	_, err := store.Read("plan", "nonexistent.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read artifact")
}

func TestStore_ReadString(t *testing.T) {
	store, _ := setupTestStore(t)

	content := "string content"
	err := store.WriteString("plan", "string.txt", content)
	require.NoError(t, err)

	readContent, err := store.ReadString("plan", "string.txt")
	require.NoError(t, err)
	assert.Equal(t, content, readContent)
}

func TestStore_ReadJSON(t *testing.T) {
	store, _ := setupTestStore(t)

	originalData := map[string]string{"key": "value"}
	err := store.WriteJSON("plan", "data.json", originalData)
	require.NoError(t, err)

	var readData map[string]string
	err = store.ReadJSON("plan", "data.json", &readData)
	require.NoError(t, err)
	assert.Equal(t, originalData["key"], readData["key"])
}

func TestStore_ReadJSON_Invalid(t *testing.T) {
	store, _ := setupTestStore(t)

	// Write invalid JSON
	err := store.WriteString("plan", "invalid.json", "not json")
	require.NoError(t, err)

	var data map[string]string
	err = store.ReadJSON("plan", "invalid.json", &data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestStore_Exists(t *testing.T) {
	store, _ := setupTestStore(t)

	// Non-existent file
	assert.False(t, store.Exists("plan", "missing.txt"))

	// Create file
	err := store.WriteString("plan", "exists.txt", "content")
	require.NoError(t, err)

	// Now it should exist
	assert.True(t, store.Exists("plan", "exists.txt"))
}

func TestStore_Delete(t *testing.T) {
	store, _ := setupTestStore(t)

	// Create file
	err := store.WriteString("plan", "delete.txt", "content")
	require.NoError(t, err)
	assert.True(t, store.Exists("plan", "delete.txt"))

	// Delete file
	err = store.Delete("plan", "delete.txt")
	require.NoError(t, err)
	assert.False(t, store.Exists("plan", "delete.txt"))
}

func TestStore_List(t *testing.T) {
	store, _ := setupTestStore(t)

	// Empty list
	files, err := store.List("plan")
	require.NoError(t, err)
	assert.Empty(t, files)

	// Add files
	store.WriteString("plan", "file1.txt", "content1")
	store.WriteString("plan", "file2.txt", "content2")

	// List should return both files
	files, err = store.List("plan")
	require.NoError(t, err)
	assert.Len(t, files, 2)
	assert.Contains(t, files, "file1.txt")
	assert.Contains(t, files, "file2.txt")
}

func TestStore_Copy(t *testing.T) {
	store, _ := setupTestStore(t)

	// Create source file
	err := store.WriteString("plan", "source.txt", "content")
	require.NoError(t, err)

	// Copy to destination
	err = store.Copy("plan", "source.txt", "spec", "dest.txt")
	require.NoError(t, err)

	// Verify destination
	content, err := store.ReadString("spec", "dest.txt")
	require.NoError(t, err)
	assert.Equal(t, "content", content)

	// Source should still exist
	assert.True(t, store.Exists("plan", "source.txt"))
}

func TestStore_Move(t *testing.T) {
	store, _ := setupTestStore(t)

	// Create source file
	err := store.WriteString("plan", "move.txt", "content")
	require.NoError(t, err)

	// Move to destination
	err = store.Move("plan", "move.txt", "spec", "moved.txt")
	require.NoError(t, err)

	// Verify destination
	content, err := store.ReadString("spec", "moved.txt")
	require.NoError(t, err)
	assert.Equal(t, "content", content)

	// Source should be gone
	assert.False(t, store.Exists("plan", "move.txt"))
}

func TestStore_GetHash(t *testing.T) {
	store, _ := setupTestStore(t)

	content := []byte("hash content")
	err := store.Write("plan", "hash.txt", content)
	require.NoError(t, err)

	hash, err := store.GetHash("plan", "hash.txt")
	require.NoError(t, err)
	assert.Len(t, hash, 64)

	// Should be deterministic
	hash2, err := store.GetHash("plan", "hash.txt")
	require.NoError(t, err)
	assert.Equal(t, hash, hash2)
}

func TestStore_GetSize(t *testing.T) {
	store, _ := setupTestStore(t)

	content := []byte("size content")
	err := store.Write("plan", "size.txt", content)
	require.NoError(t, err)

	size, err := store.GetSize("plan", "size.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), size)
}

func TestStore_CleanupStage(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	// Create files in stage
	store.WriteString("plan", "file1.txt", "content")
	store.WriteString("plan", "file2.txt", "content")

	// Cleanup stage
	err := store.CleanupStage("plan")
	require.NoError(t, err)

	// Verify directory is gone
	stagePath := filepath.Join(tmpDir, "artifacts", "test-run", "plan")
	_, err = os.Stat(stagePath)
	assert.True(t, os.IsNotExist(err))
}

func TestStore_CleanupRun(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	// Create files in multiple stages
	store.WriteString("plan", "file1.txt", "content")
	store.WriteString("spec", "file2.txt", "content")

	// Cleanup run
	err := store.CleanupRun()
	require.NoError(t, err)

	// Verify run directory is gone
	runPath := filepath.Join(tmpDir, "artifacts", "test-run")
	_, err = os.Stat(runPath)
	assert.True(t, os.IsNotExist(err))
}

func TestStore_Open(t *testing.T) {
	store, _ := setupTestStore(t)

	// Create a file
	err := store.WriteString("plan", "test.txt", "content")
	require.NoError(t, err)

	// Open it
	file, err := store.Open("plan", "test.txt")
	require.NoError(t, err)
	defer file.Close()

	content, err := io.ReadAll(file)
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}

func TestStore_Open_NotFound(t *testing.T) {
	store, _ := setupTestStore(t)

	_, err := store.Open("plan", "nonexistent.txt")
	assert.Error(t, err)
}

func TestStore_Create(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	file, err := store.Create("plan", "created.txt")
	require.NoError(t, err)

	// Write to file
	_, err = file.WriteString("created content")
	require.NoError(t, err)
	file.Close()

	// Verify file exists
	expectedPath := filepath.Join(tmpDir, "artifacts", "test-run", "plan", "created.txt")
	content, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, "created content", string(content))
}

func TestStore_Create_InvalidName(t *testing.T) {
	store, _ := setupTestStore(t)

	_, err := store.Create("plan", "../outside.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestStore_WriteFromReader(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	content := strings.NewReader("reader content")
	err := store.WriteFromReader("plan", "from-reader.txt", content)
	require.NoError(t, err)

	// Verify file
	expectedPath := filepath.Join(tmpDir, "artifacts", "test-run", "plan", "from-reader.txt")
	readContent, err := os.ReadFile(expectedPath)
	require.NoError(t, err)
	assert.Equal(t, "reader content", string(readContent))
}

func TestStore_WriteFromReader_InvalidName(t *testing.T) {
	store, _ := setupTestStore(t)

	content := strings.NewReader("content")
	err := store.WriteFromReader("plan", "../outside.txt", content)
	assert.Error(t, err)
}

func TestStore_CreateSymlink(t *testing.T) {
	store, _ := setupTestStore(t)

	// Create target file
	err := store.WriteString("plan", "target.txt", "target content")
	require.NoError(t, err)

	// Create symlink
	err = store.CreateSymlink("spec", "link.txt", "plan", "target.txt")
	require.NoError(t, err)

	// Read through symlink
	content, err := store.ReadString("spec", "link.txt")
	require.NoError(t, err)
	assert.Equal(t, "target content", content)
}

func TestStore_CreateSymlink_ReplaceExisting(t *testing.T) {
	store, _ := setupTestStore(t)

	// Create target files
	err := store.WriteString("plan", "target1.txt", "content1")
	require.NoError(t, err)
	err = store.WriteString("plan", "target2.txt", "content2")
	require.NoError(t, err)

	// Create initial symlink
	err = store.CreateSymlink("spec", "link.txt", "plan", "target1.txt")
	require.NoError(t, err)

	// Replace symlink
	err = store.CreateSymlink("spec", "link.txt", "plan", "target2.txt")
	require.NoError(t, err)

	// Verify it points to new target
	content, err := store.ReadString("spec", "link.txt")
	require.NoError(t, err)
	assert.Equal(t, "content2", content)
}

func TestStore_CreateSymlink_TargetNotExist(t *testing.T) {
	store, _ := setupTestStore(t)

	// Can create symlink to non-existent target (symlink doesn't validate target)
	err := store.CreateSymlink("spec", "link.txt", "plan", "nonexistent.txt")
	require.NoError(t, err)

	// But reading will fail
	_, err = store.ReadString("spec", "link.txt")
	assert.Error(t, err)
}

func TestStore_Namespace(t *testing.T) {
	store, _ := setupTestStore(t)

	ns := store.Namespace()
	assert.NotNil(t, ns)
}

func TestStore_Copy_SourceNotFound(t *testing.T) {
	store, _ := setupTestStore(t)

	err := store.Copy("plan", "nonexistent.txt", "spec", "dest.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read source")
}

func TestStore_Move_SourceNotFound(t *testing.T) {
	store, _ := setupTestStore(t)

	err := store.Move("plan", "nonexistent.txt", "spec", "dest.txt")
	assert.Error(t, err)
}

func TestStore_GetSize_NotFound(t *testing.T) {
	store, _ := setupTestStore(t)

	_, err := store.GetSize("plan", "nonexistent.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat")
}

func TestStore_Delete_NotFound(t *testing.T) {
	store, _ := setupTestStore(t)

	err := store.Delete("plan", "nonexistent.txt")
	assert.Error(t, err)
}

func TestStore_Write_MkdirError(t *testing.T) {
	// This is hard to test as root can write anywhere
	// Skip this test
	t.Skip("Cannot reliably test Mkdir error as root")
}

func TestStore_Write_WriteFileError(t *testing.T) {
	// This is hard to test as root can write anywhere
	// Skip this test
	t.Skip("Cannot reliably test WriteFile error as root")
}

func TestStore_CleanupStage_NotExist(t *testing.T) {
	store, _ := setupTestStore(t)

	// Cleaning up a non-existent stage should not error
	err := store.CleanupStage("nonexistent")
	require.NoError(t, err)
}

func TestStore_CleanupRun_NotExist(t *testing.T) {
	store, _ := setupTestStore(t)

	// Cleaning up a non-existent run should not error
	err := store.CleanupRun()
	require.NoError(t, err)
}
