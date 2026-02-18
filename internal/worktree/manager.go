package worktree

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MergeStrategy defines how to handle merge conflicts
type MergeStrategy string

const (
	// MergeStrategyFail fails on any conflict (deterministic, default)
	MergeStrategyFail MergeStrategy = "fail"
	// MergeStrategyOurs prefers base branch changes
	MergeStrategyOurs MergeStrategy = "ours"
	// MergeStrategyTheirs prefers worktree changes
	MergeStrategyTheirs MergeStrategy = "theirs"
)

// WorktreeConfig contains configuration for creating a worktree
type WorktreeConfig struct {
	TaskID         string
	BaseBranch     string
	WorkingDir     string
	IsolationLevel string
}

// Worktree represents a git worktree for task isolation
type Worktree struct {
	ID         string
	TaskID     string
	Path       string
	Branch     string
	BaseCommit string
	CreatedAt  time.Time
}

// MergeResult contains the result of a merge operation
type MergeResult struct {
	Success   bool
	Strategy  MergeStrategy
	Conflicts []string
	Error     error
}

// Manager handles git worktree operations
type Manager struct {
	repoRoot  string
	basePath  string
	worktrees map[string]*Worktree
	mu        sync.RWMutex
}

// NewManager creates a new worktree manager
func NewManager(repoRoot, basePath string) *Manager {
	return &Manager{
		repoRoot:  repoRoot,
		basePath:  basePath,
		worktrees: make(map[string]*Worktree),
	}
}

// IsGitRepository checks if the current directory is a git repository
func (m *Manager) IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = m.repoRoot
	err := cmd.Run()
	return err == nil
}

// HasUncommittedChanges checks if the working tree has uncommitted changes
func (m *Manager) HasUncommittedChanges() bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = m.repoRoot
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// GetCurrentHead returns the current git HEAD commit SHA
func (m *Manager) GetCurrentHead() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = m.repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// Create creates a new worktree for a task
func (m *Manager) Create(taskID string, config WorktreeConfig) (*Worktree, error) {
	if !m.IsGitRepository() {
		return nil, fmt.Errorf("not a git repository")
	}

	// Get current HEAD
	head, err := m.GetCurrentHead()
	if err != nil {
		return nil, fmt.Errorf("failed to get current HEAD: %w", err)
	}

	// Create worktree directory
	worktreeID := fmt.Sprintf("wt-%s", taskID)
	worktreePath := filepath.Join(m.basePath, worktreeID)

	if err := os.MkdirAll(worktreePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Create branch name
	branchName := fmt.Sprintf("codefoundry/%s", taskID)

	// Create worktree: git worktree add -b <branch> <path> <commit>
	cmd := exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, head)
	cmd.Dir = m.repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.RemoveAll(worktreePath)
		return nil, fmt.Errorf("failed to create worktree: %s: %w", string(output), err)
	}

	worktree := &Worktree{
		ID:         worktreeID,
		TaskID:     taskID,
		Path:       worktreePath,
		Branch:     branchName,
		BaseCommit: head,
		CreatedAt:  time.Now().UTC(),
	}

	m.mu.Lock()
	m.worktrees[worktreeID] = worktree
	m.mu.Unlock()

	return worktree, nil
}

// Get retrieves a worktree by ID
func (m *Manager) Get(worktreeID string) (*Worktree, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	worktree, ok := m.worktrees[worktreeID]
	if !ok {
		return nil, fmt.Errorf("worktree not found: %s", worktreeID)
	}

	return worktree, nil
}

// List returns all active worktrees
func (m *Manager) List() []*Worktree {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Worktree, 0, len(m.worktrees))
	for _, wt := range m.worktrees {
		result = append(result, wt)
	}

	return result
}

// GetDiff returns the diff between a worktree and the base commit
func (m *Manager) GetDiff(worktreeID string) (string, error) {
	worktree, err := m.Get(worktreeID)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("git", "diff", worktree.BaseCommit, worktree.Branch)
	cmd.Dir = m.repoRoot
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get diff: %w", err)
	}

	return string(output), nil
}

// Delete removes a worktree and cleans up
func (m *Manager) Delete(worktreeID string) error {
	worktree, err := m.Get(worktreeID)
	if err != nil {
		return err
	}

	// Remove worktree
	cmd := exec.Command("git", "worktree", "remove", "--force", worktree.Path)
	cmd.Dir = m.repoRoot
	if err := cmd.Run(); err != nil {
		// If git worktree remove fails, try manual cleanup
		os.RemoveAll(worktree.Path)
	}

	// Delete the branch
	cmd = exec.Command("git", "branch", "-D", worktree.Branch)
	cmd.Dir = m.repoRoot
	cmd.Run() // Ignore error if branch doesn't exist

	m.mu.Lock()
	delete(m.worktrees, worktreeID)
	m.mu.Unlock()

	return nil
}

// Merge merges a worktree back to the main branch
func (m *Manager) Merge(worktreeID string, strategy MergeStrategy) (*MergeResult, error) {
	worktree, err := m.Get(worktreeID)
	if err != nil {
		return nil, err
	}

	result := &MergeResult{
		Strategy: strategy,
	}

	// Check if there are any changes to merge
	cmd := exec.Command("git", "diff", "--stat", "HEAD", worktree.Branch)
	cmd.Dir = m.repoRoot
	diffOutput, err := cmd.Output()
	if err != nil {
		result.Error = fmt.Errorf("failed to check diff: %w", err)
		return result, result.Error
	}

	if len(strings.TrimSpace(string(diffOutput))) == 0 {
		// No changes to merge
		result.Success = true
		return result, nil
	}

	// Attempt merge
	cmd = exec.Command("git", "merge", "--no-commit", "--no-ff", worktree.Branch)
	cmd.Dir = m.repoRoot
	mergeOutput, err := cmd.CombinedOutput()

	if err == nil {
		// Clean merge
		result.Success = true
		return result, nil
	}

	// Check for conflicts
	cmd = exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = m.repoRoot
	conflictOutput, _ := cmd.Output()
	conflicts := parseConflicts(string(conflictOutput))

	if len(conflicts) == 0 {
		// No conflicts, but merge failed for other reason
		exec.Command("git", "merge", "--abort").Run()
		result.Error = fmt.Errorf("merge failed: %s", string(mergeOutput))
		return result, result.Error
	}

	result.Conflicts = conflicts

	// Handle conflicts based on strategy
	switch strategy {
	case MergeStrategyFail:
		exec.Command("git", "merge", "--abort").Run()
		result.Error = fmt.Errorf("merge conflicts detected: %v", conflicts)
		return result, result.Error

	case MergeStrategyOurs:
		// Accept current (main) version for all conflicts
		for _, file := range conflicts {
			cmd = exec.Command("git", "checkout", "--ours", file)
			cmd.Dir = m.repoRoot
			cmd.Run()
			cmd = exec.Command("git", "add", file)
			cmd.Dir = m.repoRoot
			cmd.Run()
		}
		result.Success = true
		result.Conflicts = markResolvedConflicts(conflicts, "ours")
		return result, nil

	case MergeStrategyTheirs:
		// Accept worktree (branch) version for all conflicts
		for _, file := range conflicts {
			cmd = exec.Command("git", "checkout", "--theirs", file)
			cmd.Dir = m.repoRoot
			cmd.Run()
			cmd = exec.Command("git", "add", file)
			cmd.Dir = m.repoRoot
			cmd.Run()
		}
		result.Success = true
		result.Conflicts = markResolvedConflicts(conflicts, "theirs")
		return result, nil

	default:
		exec.Command("git", "merge", "--abort").Run()
		result.Error = fmt.Errorf("unknown merge strategy: %s", strategy)
		return result, result.Error
	}
}

// Cleanup removes all worktrees
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	worktrees := make([]*Worktree, 0, len(m.worktrees))
	for _, wt := range m.worktrees {
		worktrees = append(worktrees, wt)
	}
	m.mu.Unlock()

	var lastErr error
	for _, wt := range worktrees {
		if err := m.Delete(wt.ID); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// parseConflicts extracts conflicted file names from git output
func parseConflicts(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var conflicts []string
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			conflicts = append(conflicts, line)
		}
	}
	return conflicts
}

// markResolvedConflicts marks conflicts as resolved with the strategy used
func markResolvedConflicts(conflicts []string, strategy string) []string {
	result := make([]string, len(conflicts))
	for i, c := range conflicts {
		result[i] = fmt.Sprintf("%s (resolved: %s)", c, strategy)
	}
	return result
}
