package artifact

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Store provides artifact storage and retrieval
type Store struct {
	namespace *Namespace
	hasher    *Hasher
}

// NewStore creates a new artifact store
func NewStore(namespace *Namespace) *Store {
	return &Store{
		namespace: namespace,
		hasher:    NewHasher(),
	}
}

// Write writes content to an artifact file
func (s *Store) Write(stageID, name string, content []byte) error {
	if err := ValidateArtifactName(name); err != nil {
		return err
	}

	artifactPath := s.namespace.ArtifactPath(stageID, name)
	artifactDir := filepath.Dir(artifactPath)

	// Ensure directory exists
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
	}

	// Write file
	if err := os.WriteFile(artifactPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write artifact: %w", err)
	}

	return nil
}

// WriteString writes a string to an artifact file
func (s *Store) WriteString(stageID, name, content string) error {
	return s.Write(stageID, name, []byte(content))
}

// WriteJSON writes a JSON object to an artifact file
func (s *Store) WriteJSON(stageID, name string, data interface{}) error {
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return s.Write(stageID, name, content)
}

// Read reads content from an artifact file
func (s *Store) Read(stageID, name string) ([]byte, error) {
	artifactPath := s.namespace.ArtifactPath(stageID, name)
	content, err := os.ReadFile(artifactPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact: %w", err)
	}
	return content, nil
}

// ReadString reads a string from an artifact file
func (s *Store) ReadString(stageID, name string) (string, error) {
	content, err := s.Read(stageID, name)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// ReadJSON reads and unmarshals a JSON artifact
func (s *Store) ReadJSON(stageID, name string, target interface{}) error {
	content, err := s.Read(stageID, name)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(content, target); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return nil
}

// Exists checks if an artifact exists
func (s *Store) Exists(stageID, name string) bool {
	artifactPath := s.namespace.ArtifactPath(stageID, name)
	_, err := os.Stat(artifactPath)
	return err == nil
}

// Delete removes an artifact
func (s *Store) Delete(stageID, name string) error {
	artifactPath := s.namespace.ArtifactPath(stageID, name)
	if err := os.Remove(artifactPath); err != nil {
		return fmt.Errorf("failed to delete artifact: %w", err)
	}
	return nil
}

// List returns all artifacts for a stage
func (s *Store) List(stageID string) ([]string, error) {
	stagePath := s.namespace.StagePath(stageID)
	entries, err := os.ReadDir(stagePath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	artifacts := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			artifacts = append(artifacts, entry.Name())
		}
	}

	return artifacts, nil
}

// Copy copies an artifact from one stage to another
func (s *Store) Copy(srcStage, srcName, dstStage, dstName string) error {
	content, err := s.Read(srcStage, srcName)
	if err != nil {
		return fmt.Errorf("failed to read source artifact: %w", err)
	}

	if err := s.Write(dstStage, dstName, content); err != nil {
		return fmt.Errorf("failed to write destination artifact: %w", err)
	}

	return nil
}

// Move moves an artifact from one location to another
func (s *Store) Move(srcStage, srcName, dstStage, dstName string) error {
	if err := s.Copy(srcStage, srcName, dstStage, dstName); err != nil {
		return err
	}
	return s.Delete(srcStage, srcName)
}

// GetHash returns the hash of an artifact
func (s *Store) GetHash(stageID, name string) (string, error) {
	artifactPath := s.namespace.ArtifactPath(stageID, name)
	return s.hasher.HashFile(artifactPath)
}

// GetSize returns the size of an artifact in bytes
func (s *Store) GetSize(stageID, name string) (int64, error) {
	artifactPath := s.namespace.ArtifactPath(stageID, name)
	info, err := os.Stat(artifactPath)
	if err != nil {
		return 0, fmt.Errorf("failed to stat artifact: %w", err)
	}
	return info.Size(), nil
}

// Open returns a file handle for reading
func (s *Store) Open(stageID, name string) (*os.File, error) {
	artifactPath := s.namespace.ArtifactPath(stageID, name)
	return os.Open(artifactPath)
}

// Create returns a file handle for writing
func (s *Store) Create(stageID, name string) (*os.File, error) {
	if err := ValidateArtifactName(name); err != nil {
		return nil, err
	}

	artifactPath := s.namespace.ArtifactPath(stageID, name)
	artifactDir := filepath.Dir(artifactPath)

	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create artifact directory: %w", err)
	}

	return os.Create(artifactPath)
}

// WriteFromReader writes content from a reader
func (s *Store) WriteFromReader(stageID, name string, r io.Reader) error {
	if err := ValidateArtifactName(name); err != nil {
		return err
	}

	artifactPath := s.namespace.ArtifactPath(stageID, name)
	artifactDir := filepath.Dir(artifactPath)

	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return fmt.Errorf("failed to create artifact directory: %w", err)
	}

	file, err := os.Create(artifactPath)
	if err != nil {
		return fmt.Errorf("failed to create artifact file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, r); err != nil {
		return fmt.Errorf("failed to write artifact: %w", err)
	}

	return nil
}

// CreateSymlink creates a symbolic link to another artifact
func (s *Store) CreateSymlink(stageID, name, targetStage, targetName string) error {
	targetPath := s.namespace.ArtifactPath(targetStage, targetName)
	linkPath := s.namespace.ArtifactPath(stageID, name)
	
	// Ensure link directory exists
	linkDir := filepath.Dir(linkPath)
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		return fmt.Errorf("failed to create link directory: %w", err)
	}

	// Remove existing link if it exists
	if _, err := os.Lstat(linkPath); err == nil {
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("failed to remove existing link: %w", err)
		}
	}

	// Create the symlink
	if err := os.Symlink(targetPath, linkPath); err != nil {
		return fmt.Errorf("failed to create symlink: %w", err)
	}

	return nil
}

// CleanupStage removes all artifacts for a stage
func (s *Store) CleanupStage(stageID string) error {
	stagePath := s.namespace.StagePath(stageID)
	return os.RemoveAll(stagePath)
}

// CleanupRun removes all artifacts for the current run
func (s *Store) CleanupRun() error {
	runPath := s.namespace.RunPath()
	return os.RemoveAll(runPath)
}

// Namespace returns the store's namespace
func (s *Store) Namespace() *Namespace {
	return s.namespace
}
