package artifact

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHasher(t *testing.T) {
	hasher := NewHasher()
	assert.NotNil(t, hasher)
	assert.NotNil(t, hasher.h)
}

func TestHasher_HashContent(t *testing.T) {
	hasher := NewHasher()

	// Hash same content twice, should get same result
	content := []byte("test content")
	hash1 := hasher.HashContent(content)
	hash2 := hasher.HashContent(content)

	assert.Equal(t, hash1, hash2)
	assert.Len(t, hash1, 64) // SHA256 produces 64 hex chars

	// Different content should produce different hash
	differentContent := []byte("different content")
	hash3 := hasher.HashContent(differentContent)
	assert.NotEqual(t, hash1, hash3)
}

func TestHasher_HashString(t *testing.T) {
	hasher := NewHasher()

	// Hash same string twice
	str := "test string"
	hash1 := hasher.HashString(str)
	hash2 := hasher.HashString(str)

	assert.Equal(t, hash1, hash2)
}

func TestHasher_HashFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	content := []byte("test file content")
	require.NoError(t, os.WriteFile(filePath, content, 0644))

	hasher := NewHasher()

	// Hash file
	hash1, err := hasher.HashFile(filePath)
	require.NoError(t, err)

	// Hash same content directly
	hash2 := hasher.HashContent(content)

	assert.Equal(t, hash1, hash2)

	// Hash different content
	require.NoError(t, os.WriteFile(filePath, []byte("different content"), 0644))
	hash3, err := hasher.HashFile(filePath)
	require.NoError(t, err)
	assert.NotEqual(t, hash1, hash3)
}

func TestHasher_HashFile_NotFound(t *testing.T) {
	hasher := NewHasher()
	_, err := hasher.HashFile("/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestHasher_HashReader(t *testing.T) {
	hasher := NewHasher()

	content := []byte("reader content")
	reader := bytes.NewReader(content)

	hash1, err := hasher.HashReader(reader)
	require.NoError(t, err)

	hash2 := hasher.HashContent(content)
	assert.Equal(t, hash1, hash2)
}

func TestHasher_HashContentWithPrefix(t *testing.T) {
	hasher := NewHasher()

	content := []byte("test content")

	hash1 := hasher.HashContentWithPrefix(content, "prefix1")
	hash2 := hasher.HashContentWithPrefix(content, "prefix2")
	hash3 := hasher.HashContent(content)

	// Different prefixes should produce different hashes
	assert.NotEqual(t, hash1, hash2)
	// Prefix should produce different hash than no prefix
	assert.NotEqual(t, hash1, hash3)
	assert.NotEqual(t, hash2, hash3)

	// Same prefix should produce same hash
	hash4 := hasher.HashContentWithPrefix(content, "prefix1")
	assert.Equal(t, hash1, hash4)
}

func TestHasher_ValidateHash(t *testing.T) {
	hasher := NewHasher()

	content := []byte("test content")
	hash := hasher.HashContent(content)

	// Valid hash
	assert.True(t, hasher.ValidateHash(content, hash))

	// Invalid hash
	assert.False(t, hasher.ValidateHash(content, "invalid"))

	// Different content
	assert.False(t, hasher.ValidateHash([]byte("different"), hash))
}

func TestQuickHash(t *testing.T) {
	content := []byte("quick test")

	hash1 := QuickHash(content)
	hash2 := QuickHash(content)

	assert.Equal(t, hash1, hash2)
	assert.Len(t, hash1, 64)
}

func TestQuickHashFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content")
	require.NoError(t, os.WriteFile(filePath, content, 0644))

	hash, err := QuickHashFile(filePath)
	require.NoError(t, err)

	expectedHash := QuickHash(content)
	assert.Equal(t, expectedHash, hash)
}

func TestQuickHashFile_NotFound(t *testing.T) {
	_, err := QuickHashFile("/nonexistent/file.txt")
	assert.Error(t, err)
}

func TestContentAddress(t *testing.T) {
	hash := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"

	address := ContentAddress(hash)
	expected := "ab/cd/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	assert.Equal(t, expected, address)

	// Short hash should be returned as-is
	shortHash := "short"
	assert.Equal(t, shortHash, ContentAddress(shortHash))
}

func TestHasher_Deterministic(t *testing.T) {
	hasher1 := NewHasher()
	hasher2 := NewHasher()

	content := []byte("deterministic test")

	hash1 := hasher1.HashContent(content)
	hash2 := hasher2.HashContent(content)

	// Different instances should produce same hash for same content
	assert.Equal(t, hash1, hash2)
}

func TestHasher_HashEmptyContent(t *testing.T) {
	hasher := NewHasher()

	emptyContent := []byte{}
	hash := hasher.HashContent(emptyContent)

	// Should produce valid hash
	assert.Len(t, hash, 64)
	assert.NotEmpty(t, hash)
}

func TestHasher_HashFile_OpenError(t *testing.T) {
	hasher := NewHasher()

	// Try to hash a directory (can't be opened as file)
	_, err := hasher.HashFile("/nonexistent/directory/file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open file")
}

func TestHasher_HashFile_ReadError(t *testing.T) {
	hasher := NewHasher()

	// Try to hash a directory
	_, err := hasher.HashFile("/dev")
	assert.Error(t, err)
}

func TestHasher_HashReader_ReadError(t *testing.T) {
	hasher := NewHasher()

	// Create a reader that returns an error
	errReader := &errorReader{}
	_, err := hasher.HashReader(errReader)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to hash reader")
}

// errorReader is a test helper that always returns an error
type errorReader struct{}

func (r *errorReader) Read(p []byte) (n int, err error) {
	return 0, assert.AnError
}
