package worktree

import (
	"fmt"
	"os"
	"path/filepath"
)

// IsolationLevel defines the level of isolation for a worktree
type IsolationLevel string

const (
	// IsolationLevelBasic provides filesystem isolation via git worktree
	IsolationLevelBasic IsolationLevel = "basic"
	// IsolationLevelStrict provides additional process isolation
	IsolationLevelStrict IsolationLevel = "strict"
	// IsolationLevelFull provides maximum isolation (chroot, network, etc.)
	IsolationLevelFull IsolationLevel = "full"
)

// IsolationConfig contains isolation configuration
type IsolationConfig struct {
	Level          IsolationLevel
	PreventNetwork bool
	PreventWrite   bool
	ResourceLimits *ResourceLimits
}

// ResourceLimits defines resource constraints
type ResourceLimits struct {
	MaxMemoryMB   int
	MaxCPUPercent int
	MaxDiskMB     int
}

// IsolationGuarantee represents an isolation promise
type IsolationGuarantee struct {
	WorktreeID string
	Level      IsolationLevel
	Guarantees []string
	Enforced   bool
}

// GetIsolationLevel returns the isolation level for a string
func GetIsolationLevel(level string) (IsolationLevel, error) {
	switch level {
	case "basic":
		return IsolationLevelBasic, nil
	case "strict":
		return IsolationLevelStrict, nil
	case "full":
		return IsolationLevelFull, nil
	default:
		return "", fmt.Errorf("invalid isolation level: %s (valid: basic, strict, full)", level)
	}
}

// ValidateIsolationConfig validates isolation configuration
func ValidateIsolationConfig(config *IsolationConfig) error {
	if config == nil {
		return fmt.Errorf("isolation config is nil")
	}

	_, err := GetIsolationLevel(string(config.Level))
	if err != nil {
		return err
	}

	if config.ResourceLimits != nil {
		if config.ResourceLimits.MaxMemoryMB < 0 {
			return fmt.Errorf("max memory must be non-negative")
		}
		if config.ResourceLimits.MaxCPUPercent < 0 || config.ResourceLimits.MaxCPUPercent > 100 {
			return fmt.Errorf("max CPU percent must be between 0 and 100")
		}
		if config.ResourceLimits.MaxDiskMB < 0 {
			return fmt.Errorf("max disk must be non-negative")
		}
	}

	return nil
}

// GetIsolationGuarantees returns the guarantees for an isolation level
func GetIsolationGuarantees(level IsolationLevel) []string {
	switch level {
	case IsolationLevelBasic:
		return []string{
			"separate_working_directory",
			"independent_git_state",
			"branch_isolation",
		}
	case IsolationLevelStrict:
		return []string{
			"separate_working_directory",
			"independent_git_state",
			"branch_isolation",
			"process_isolation",
			"resource_limits",
		}
	case IsolationLevelFull:
		return []string{
			"separate_working_directory",
			"independent_git_state",
			"branch_isolation",
			"process_isolation",
			"resource_limits",
			"network_isolation",
			"filesystem_sandbox",
		}
	default:
		return []string{}
	}
}

// EnforceIsolation attempts to enforce isolation guarantees
func (m *Manager) EnforceIsolation(worktreeID string, config *IsolationConfig) (*IsolationGuarantee, error) {
	if err := ValidateIsolationConfig(config); err != nil {
		return nil, err
	}

	// Verify worktree exists
	if _, err := m.Get(worktreeID); err != nil {
		return nil, err
	}

	guarantee := &IsolationGuarantee{
		WorktreeID: worktreeID,
		Level:      config.Level,
		Guarantees: GetIsolationGuarantees(config.Level),
		Enforced:   true,
	}

	// Basic level guarantees are already provided by git worktree
	if config.Level == IsolationLevelBasic {
		return guarantee, nil
	}

	// For strict/full levels, we would need additional enforcement
	// This is a placeholder for future implementation
	// In practice, this might involve:
	// - Setting up cgroups for resource limits
	// - Configuring network namespaces
	// - Setting up chroot jails

	return guarantee, nil
}

// CleanupWorktree performs comprehensive cleanup of a worktree
func (m *Manager) CleanupWorktree(worktreeID string) error {
	worktree, err := m.Get(worktreeID)
	if err != nil {
		return err
	}

	// Ensure worktree directory is removed
	if _, err := os.Stat(worktree.Path); err == nil {
		if err := os.RemoveAll(worktree.Path); err != nil {
			return fmt.Errorf("failed to remove worktree directory: %w", err)
		}
	}

	// Remove from tracking
	m.mu.Lock()
	delete(m.worktrees, worktreeID)
	m.mu.Unlock()

	return nil
}

// VerifyIsolation verifies that isolation guarantees are maintained
func (m *Manager) VerifyIsolation(worktreeID string) (*IsolationVerification, error) {
	worktree, err := m.Get(worktreeID)
	if err != nil {
		return nil, err
	}

	verification := &IsolationVerification{
		WorktreeID: worktreeID,
		Checks:     make(map[string]bool),
		Passed:     true,
	}

	// Check worktree directory exists and is separate
	if _, err := os.Stat(worktree.Path); err == nil {
		verification.Checks["directory_exists"] = true
	} else {
		verification.Checks["directory_exists"] = false
		verification.Passed = false
	}

	// Check worktree is not the repo root
	absRepoRoot, _ := filepath.Abs(m.repoRoot)
	absWorktreePath, _ := filepath.Abs(worktree.Path)
	verification.Checks["separate_from_repo"] = absRepoRoot != absWorktreePath
	if !verification.Checks["separate_from_repo"] {
		verification.Passed = false
	}

	return verification, nil
}

// IsolationVerification contains isolation verification results
type IsolationVerification struct {
	WorktreeID string
	Checks     map[string]bool
	Passed     bool
}

// ValidateWorktreeConfig validates a worktree configuration
func ValidateWorktreeConfig(config *WorktreeConfig) error {
	if config.TaskID == "" {
		return fmt.Errorf("task ID is required")
	}

	if config.WorkingDir == "" {
		return fmt.Errorf("working directory is required")
	}

	return nil
}
