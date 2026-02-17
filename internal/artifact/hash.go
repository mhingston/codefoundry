package artifact

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
)

// Hasher provides content-addressed hashing
type Hasher struct {
	h hash.Hash
}

// NewHasher creates a new SHA256 hasher
func NewHasher() *Hasher {
	return &Hasher{
		h: sha256.New(),
	}
}

// HashContent hashes byte content and returns the hash string
func (h *Hasher) HashContent(content []byte) string {
	h.h.Reset()
	h.h.Write(content)
	return hex.EncodeToString(h.h.Sum(nil))
}

// HashString hashes a string and returns the hash
func (h *Hasher) HashString(content string) string {
	return h.HashContent([]byte(content))
}

// HashFile reads a file and returns its hash
func (h *Hasher) HashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	h.h.Reset()
	if _, err := io.Copy(h.h, file); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.h.Sum(nil)), nil
}

// HashReader reads from an io.Reader and returns the hash
func (h *Hasher) HashReader(r io.Reader) (string, error) {
	h.h.Reset()
	if _, err := io.Copy(h.h, r); err != nil {
		return "", fmt.Errorf("failed to hash reader: %w", err)
	}
	return hex.EncodeToString(h.h.Sum(nil)), nil
}

// HashContentWithPrefix hashes content with a prefix for namespacing
func (h *Hasher) HashContentWithPrefix(content []byte, prefix string) string {
	h.h.Reset()
	h.h.Write([]byte(prefix))
	h.h.Write([]byte(":"))
	h.h.Write(content)
	return hex.EncodeToString(h.h.Sum(nil))
}

// ValidateHash checks if content matches the expected hash
func (h *Hasher) ValidateHash(content []byte, expectedHash string) bool {
	actualHash := h.HashContent(content)
	return actualHash == expectedHash
}

// QuickHash is a convenience function for one-off hashing
func QuickHash(content []byte) string {
	h := sha256.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// QuickHashFile is a convenience function for one-off file hashing
func QuickHashFile(path string) (string, error) {
	h := NewHasher()
	return h.HashFile(path)
}

// ContentAddress generates a content address for storage
// Format: <first-2-chars>/<next-2-chars>/<full-hash>
func ContentAddress(hash string) string {
	if len(hash) < 64 {
		return hash
	}
	return fmt.Sprintf("%s/%s/%s", hash[:2], hash[2:4], hash)
}
