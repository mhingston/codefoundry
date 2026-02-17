package worktree

import (
	"fmt"
	"os/exec"
	"strings"
)

// MergeOptions contains options for merge operations
type MergeOptions struct {
	Strategy      MergeStrategy
	CommitMessage string
	Squash        bool
}

// ValidateMergeStrategy validates a merge strategy string
func ValidateMergeStrategy(strategy string) (MergeStrategy, error) {
	switch strings.ToLower(strategy) {
	case "fail":
		return MergeStrategyFail, nil
	case "ours":
		return MergeStrategyOurs, nil
	case "theirs":
		return MergeStrategyTheirs, nil
	default:
		return "", fmt.Errorf("invalid merge strategy: %s (valid: fail, ours, theirs)", strategy)
	}
}

// DefaultMergeStrategy returns the default merge strategy
func DefaultMergeStrategy() MergeStrategy {
	return MergeStrategyFail
}

// MergeWithOptions performs a merge with additional options
func (m *Manager) MergeWithOptions(worktreeID string, opts MergeOptions) (*MergeResult, error) {
	worktree, err := m.Get(worktreeID)
	if err != nil {
		return nil, err
	}

	result := &MergeResult{
		Strategy: opts.Strategy,
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

	// Build merge command
	mergeArgs := []string{"merge", "--no-commit", "--no-ff"}
	if opts.Squash {
		mergeArgs = append(mergeArgs, "--squash")
	}
	mergeArgs = append(mergeArgs, worktree.Branch)

	// Attempt merge
	cmd = exec.Command("git", mergeArgs...)
	cmd.Dir = m.repoRoot
	mergeOutput, err := cmd.CombinedOutput()

	if err == nil {
		// Clean merge
		result.Success = true
		
		// Add commit message if provided
		if opts.CommitMessage != "" {
			cmd = exec.Command("git", "commit", "-m", opts.CommitMessage)
			cmd.Dir = m.repoRoot
			cmd.Run()
		}
		
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
	switch opts.Strategy {
	case MergeStrategyFail:
		exec.Command("git", "merge", "--abort").Run()
		result.Error = fmt.Errorf("merge conflicts detected: %v", conflicts)
		return result, result.Error

	case MergeStrategyOurs:
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
		
		if opts.CommitMessage != "" {
			cmd = exec.Command("git", "commit", "-m", opts.CommitMessage)
			cmd.Dir = m.repoRoot
			cmd.Run()
		}
		
		return result, nil

	case MergeStrategyTheirs:
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
		
		if opts.CommitMessage != "" {
			cmd = exec.Command("git", "commit", "-m", opts.CommitMessage)
			cmd.Dir = m.repoRoot
			cmd.Run()
		}
		
		return result, nil

	default:
		exec.Command("git", "merge", "--abort").Run()
		result.Error = fmt.Errorf("unknown merge strategy: %s", opts.Strategy)
		return result, result.Error
	}
}

// CanAutoMerge checks if a worktree can be merged without conflicts
func (m *Manager) CanAutoMerge(worktreeID string) (bool, error) {
	worktree, err := m.Get(worktreeID)
	if err != nil {
		return false, err
	}

	// Try a dry-run merge
	cmd := exec.Command("git", "merge-tree", "HEAD", worktree.Branch)
	cmd.Dir = m.repoRoot
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check merge status: %w", err)
	}

	// If output contains "<<" or ">>", there are conflicts
	content := string(output)
	if strings.Contains(content, "<<<<<<<") || strings.Contains(content, ">>>>>>>") {
		return false, nil
	}

	return true, nil
}

// GetConflictDetails returns detailed information about conflicts
func (m *Manager) GetConflictDetails(worktreeID string) ([]ConflictDetail, error) {
	worktree, err := m.Get(worktreeID)
	if err != nil {
		return nil, err
	}

	// Start a test merge
	cmd := exec.Command("git", "merge", "--no-commit", "--no-ff", worktree.Branch)
	cmd.Dir = m.repoRoot
	cmd.Run()

	// Get conflicted files
	cmd = exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = m.repoRoot
	output, err := cmd.Output()
	if err != nil {
		exec.Command("git", "merge", "--abort").Run()
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var details []ConflictDetail

	for _, line := range lines {
		if line = strings.TrimSpace(line); line == "" {
			continue
		}

		// Get conflict markers in the file
		cmd = exec.Command("git", "show", ":1:"+line)
		cmd.Dir = m.repoRoot
		base, _ := cmd.Output()

		cmd = exec.Command("git", "show", ":2:"+line)
		cmd.Dir = m.repoRoot
		ours, _ := cmd.Output()

		cmd = exec.Command("git", "show", ":3:"+line)
		cmd.Dir = m.repoRoot
		theirs, _ := cmd.Output()

		details = append(details, ConflictDetail{
			File:   line,
			Base:   string(base),
			Ours:   string(ours),
			Theirs: string(theirs),
		})
	}

	// Abort the test merge
	exec.Command("git", "merge", "--abort").Run()

	return details, nil
}

// ConflictDetail contains detailed conflict information
type ConflictDetail struct {
	File   string
	Base   string
	Ours   string
	Theirs string
}
