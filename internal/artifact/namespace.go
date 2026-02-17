package artifact

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Namespace manages stage-scoped artifact paths
type Namespace struct {
	basePath string
	runID    string
}

// NewNamespace creates a new artifact namespace
func NewNamespace(basePath, runID string) *Namespace {
	return &Namespace{
		basePath: basePath,
		runID:    runID,
	}
}

// GetBasePath returns the base path
func (n *Namespace) GetBasePath() string {
	return n.basePath
}

// GetRunID returns the run ID
func (n *Namespace) GetRunID() string {
	return n.runID
}

// StagePath returns the path for a stage's artifacts
func (n *Namespace) StagePath(stageID string) string {
	return filepath.Join(n.basePath, "artifacts", n.runID, stageID)
}

// ArtifactPath returns the full path for an artifact
func (n *Namespace) ArtifactPath(stageID, artifactName string) string {
	return filepath.Join(n.StagePath(stageID), artifactName)
}

// StatusPath returns the path for a stage's status.json
func (n *Namespace) StatusPath(stageID string) string {
	return filepath.Join(n.StagePath(stageID), "status.json")
}

// GateReportPath returns the path for a gate report
func (n *Namespace) GateReportPath(stageID, gateID string) string {
	return filepath.Join(n.StagePath(stageID), fmt.Sprintf("%s.json", gateID))
}

// StatePath returns the path for the state.json
func (n *Namespace) StatePath() string {
	return filepath.Join(n.basePath, "state", "state.json")
}

// RunPath returns the path for the run directory
func (n *Namespace) RunPath() string {
	return filepath.Join(n.basePath, "artifacts", n.runID)
}

// ResolveInputPath resolves an input path relative to the run
func (n *Namespace) ResolveInputPath(stageID, inputPath string) string {
	// If it's an absolute path, return as-is
	if filepath.IsAbs(inputPath) {
		return inputPath
	}
	
	// If path starts with ./ or ../, resolve relative to current directory
	if strings.HasPrefix(inputPath, "./") || strings.HasPrefix(inputPath, "../") {
		return inputPath
	}
	
	// Otherwise, resolve relative to the stage's artifact directory
	return filepath.Join(n.StagePath(stageID), inputPath)
}

// ListStages returns all stage IDs in the namespace
func (n *Namespace) ListStages() ([]string, error) {
	runPath := n.RunPath()
	entries, err := filepath.Glob(filepath.Join(runPath, "*"))
	if err != nil {
		return nil, err
	}
	
	stages := make([]string, 0, len(entries))
	for _, entry := range entries {
		info, err := filepath.Abs(entry)
		if err != nil {
			continue
		}
		// Skip if it's a file
		if filepath.Ext(info) != "" {
			continue
		}
		stages = append(stages, filepath.Base(entry))
	}
	
	return stages, nil
}

// ValidateArtifactName checks if an artifact name is valid
func ValidateArtifactName(name string) error {
	if name == "" {
		return fmt.Errorf("artifact name cannot be empty")
	}
	
	// Check for path traversal
	if strings.Contains(name, "..") {
		return fmt.Errorf("artifact name contains path traversal: %s", name)
	}
	
	// Check for absolute paths
	if filepath.IsAbs(name) {
		return fmt.Errorf("artifact name cannot be absolute path: %s", name)
	}
	
	return nil
}

// GetRelativePath returns the path relative to the stage directory
func (n *Namespace) GetRelativePath(stageID, fullPath string) (string, error) {
	stageDir := n.StagePath(stageID)
	rel, err := filepath.Rel(stageDir, fullPath)
	if err != nil {
		return "", err
	}
	return rel, nil
}

// CreateStageDirectory creates the directory for a stage
func (n *Namespace) CreateStageDirectory(stageID string) error {
	stageDir := n.StagePath(stageID)
	return os.MkdirAll(stageDir, 0755)
}
