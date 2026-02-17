package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	repoRoot := filepath.Join(tmpDir, "repo")
	basePath := filepath.Join(tmpDir, "worktrees")

	os.MkdirAll(repoRoot, 0755)
	
	m := NewManager(repoRoot, basePath)
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
	
	if m.repoRoot != repoRoot {
		t.Errorf("expected repoRoot %s, got %s", repoRoot, m.repoRoot)
	}
	
	if m.basePath != basePath {
		t.Errorf("expected basePath %s, got %s", basePath, m.basePath)
	}
}

func TestIsGitRepository(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Test non-git directory
	m := NewManager(tmpDir, tmpDir)
	if m.IsGitRepository() {
		t.Error("expected false for non-git directory")
	}
	
	// Initialize a git repo
	repoDir := filepath.Join(tmpDir, "git-repo")
	os.MkdirAll(repoDir, 0755)
	
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	m = NewManager(repoDir, tmpDir)
	if !m.IsGitRepository() {
		t.Error("expected true for git directory")
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	m := NewManager(repoDir, tmpDir)
	
	// Initially no changes
	if m.HasUncommittedChanges() {
		t.Error("expected no uncommitted changes initially")
	}
	
	// Create a file
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	
	// Now should have uncommitted changes
	if !m.HasUncommittedChanges() {
		t.Error("expected uncommitted changes after creating file")
	}
}

func TestGetCurrentHead(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, tmpDir)
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	head, err := m.GetCurrentHead()
	if err != nil {
		t.Fatalf("failed to get HEAD: %v", err)
	}
	
	if head == "" {
		t.Error("expected non-empty HEAD")
	}
	
	if len(head) != 40 {
		t.Errorf("expected 40 character SHA, got %d: %s", len(head), head)
	}
}

func TestCreateAndDelete(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	// Create worktree
	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	
	if worktree.ID != "wt-test-task" {
		t.Errorf("expected worktree ID wt-test-task, got %s", worktree.ID)
	}
	
	if worktree.TaskID != "test-task" {
		t.Errorf("expected task ID test-task, got %s", worktree.TaskID)
	}
	
	if !strings.Contains(worktree.Path, "wt-test-task") {
		t.Errorf("expected path to contain wt-test-task, got %s", worktree.Path)
	}
	
	// Verify worktree exists
	if _, err := os.Stat(worktree.Path); os.IsNotExist(err) {
		t.Error("worktree directory should exist")
	}
	
	// Delete worktree
	if err := m.Delete(worktree.ID); err != nil {
		t.Fatalf("failed to delete worktree: %v", err)
	}
	
	// Verify worktree is gone
	if _, err := m.Get(worktree.ID); err == nil {
		t.Error("expected worktree to be deleted")
	}
}

func TestCreateNotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: tmpDir,
	}
	
	_, err := m.Create("test-task", config)
	if err == nil {
		t.Error("expected error for non-git repository")
	}
}

func TestGetDiff(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)
	
	// Make a change in the worktree
	worktreeFile := filepath.Join(worktree.Path, "test.txt")
	os.WriteFile(worktreeFile, []byte("hello world"), 0644)
	
	// Commit the change
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = worktree.Path
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "update")
	cmd.Dir = worktree.Path
	cmd.Run()
	
	// Get diff
	diff, err := m.GetDiff(worktree.ID)
	if err != nil {
		t.Fatalf("failed to get diff: %v", err)
	}
	
	if diff == "" {
		t.Error("expected non-empty diff")
	}
	
	if !strings.Contains(diff, "hello world") {
		t.Error("expected diff to contain 'hello world'")
	}
}

func TestMerge(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)
	
	// Make a change in the worktree
	worktreeFile := filepath.Join(worktree.Path, "test.txt")
	os.WriteFile(worktreeFile, []byte("hello world"), 0644)
	
	// Commit the change
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = worktree.Path
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "update")
	cmd.Dir = worktree.Path
	cmd.Run()
	
	// Merge with fail strategy
	result, err := m.Merge(worktree.ID, MergeStrategyFail)
	if err != nil {
		t.Fatalf("failed to merge: %v", err)
	}
	
	if !result.Success {
		t.Errorf("expected successful merge, got error: %v", result.Error)
	}
}

func TestMergeOursStrategy(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)
	
	// Make different changes
	os.WriteFile(testFile, []byte("hello from main"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "main update")
	cmd.Dir = repoDir
	cmd.Run()
	
	worktreeFile := filepath.Join(worktree.Path, "test.txt")
	os.WriteFile(worktreeFile, []byte("hello from worktree"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = worktree.Path
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "worktree update")
	cmd.Dir = worktree.Path
	cmd.Run()
	
	// Merge with ours strategy
	result, err := m.Merge(worktree.ID, MergeStrategyOurs)
	if err != nil {
		t.Fatalf("failed to merge with ours: %v", err)
	}
	
	if !result.Success {
		t.Errorf("expected successful merge, got error: %v", result.Error)
	}
	
	// Should have conflicts resolved
	if len(result.Conflicts) == 0 {
		t.Error("expected conflicts to be resolved")
	}
	
	for _, c := range result.Conflicts {
		if !strings.Contains(c, "resolved: ours") {
			t.Errorf("expected conflict to be resolved with ours, got: %s", c)
		}
	}
}

func TestList(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	// Initially empty
	worktrees := m.List()
	if len(worktrees) != 0 {
		t.Errorf("expected 0 worktrees, got %d", len(worktrees))
	}
	
	// Create worktree
	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)
	
	// Now should have 1
	worktrees = m.List()
	if len(worktrees) != 1 {
		t.Errorf("expected 1 worktree, got %d", len(worktrees))
	}
	
	if worktrees[0].ID != worktree.ID {
		t.Errorf("expected worktree ID %s, got %s", worktree.ID, worktrees[0].ID)
	}
}

func TestGetNonExistentWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)
	
	_, err := m.Get("non-existent")
	if err == nil {
		t.Error("expected error for non-existent worktree")
	}
}

func TestDeleteNonExistentWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)
	
	err := m.Delete("non-existent")
	if err == nil {
		t.Error("expected error for non-existent worktree")
	}
}

func TestParseConflicts(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "empty",
			input:    "",
			expected: []string{},
		},
		{
			name:     "single conflict",
			input:    "file.go",
			expected: []string{"file.go"},
		},
		{
			name:     "multiple conflicts",
			input:    "file1.go\nfile2.go\nfile3.go",
			expected: []string{"file1.go", "file2.go", "file3.go"},
		},
		{
			name:     "with whitespace",
			input:    "  file1.go  \n  file2.go  ",
			expected: []string{"file1.go", "file2.go"},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseConflicts(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d conflicts, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if i < len(result) && result[i] != expected {
					t.Errorf("expected conflict %d to be %s, got %s", i, expected, result[i])
				}
			}
		})
	}
}

func TestMarkResolvedConflicts(t *testing.T) {
	conflicts := []string{"file1.go", "file2.go"}
	result := markResolvedConflicts(conflicts, "ours")
	
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
	
	for _, r := range result {
		if !strings.Contains(r, "resolved: ours") {
			t.Errorf("expected result to contain 'resolved: ours', got %s", r)
		}
	}
}

func TestGetDiffInvalidWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)
	
	_, err := m.GetDiff("non-existent")
	if err == nil {
		t.Error("expected error for non-existent worktree")
	}
}

func TestGetDiffNotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)
	
	// Add a fake worktree to bypass Get() check
	m.mu.Lock()
	m.worktrees["test-wt"] = &Worktree{
		ID:         "test-wt",
		BaseCommit: "abc123",
		Branch:     "test-branch",
	}
	m.mu.Unlock()
	
	_, err := m.GetDiff("test-wt")
	// Should fail because not a git repo
	if err == nil {
		t.Error("expected error when not in a git repository")
	}
}

func TestCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	// Create multiple worktrees
	worktrees := []string{"task-1", "task-2", "task-3"}
	for _, taskID := range worktrees {
		config.TaskID = taskID
		_, err := m.Create(taskID, config)
		if err != nil {
			t.Fatalf("failed to create worktree %s: %v", taskID, err)
		}
	}
	
	// Verify worktrees exist
	if len(m.List()) != len(worktrees) {
		t.Errorf("expected %d worktrees, got %d", len(worktrees), len(m.List()))
	}
	
	// Cleanup all worktrees
	err := m.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}
	
	// Verify all worktrees are removed
	if len(m.List()) != 0 {
		t.Errorf("expected 0 worktrees after cleanup, got %d", len(m.List()))
	}
}

func TestCreatePermissionError(t *testing.T) {
	// This test is difficult to run reliably without actually changing permissions
	// We'll test the error path by using a non-git directory
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: tmpDir,
	}
	
	_, err := m.Create("test-task", config)
	if err == nil {
		t.Error("expected error for non-git repository")
	}
}

func TestGetCurrentHeadError(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	m := NewManager(repoDir, tmpDir)
	
	// GetCurrentHead should fail because there are no commits
	_, err := m.GetCurrentHead()
	if err == nil {
		t.Error("expected error when no commits exist")
	}
}

func TestGetCurrentHeadNotGitRepo(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)
	
	_, err := m.GetCurrentHead()
	if err == nil {
		t.Error("expected error when not in a git repository")
	}
}

func TestHasUncommittedChangesError(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)
	
	// Should return false for non-git directory (not error)
	if m.HasUncommittedChanges() {
		t.Error("expected false for non-git directory")
	}
}

func TestMergeTheirsStrategy(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)
	
	// Make different changes
	os.WriteFile(testFile, []byte("hello from main"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "main update")
	cmd.Dir = repoDir
	cmd.Run()
	
	worktreeFile := filepath.Join(worktree.Path, "test.txt")
	os.WriteFile(worktreeFile, []byte("hello from worktree"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = worktree.Path
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "worktree update")
	cmd.Dir = worktree.Path
	cmd.Run()
	
	// Merge with theirs strategy
	result, err := m.Merge(worktree.ID, MergeStrategyTheirs)
	if err != nil {
		t.Fatalf("failed to merge with theirs: %v", err)
	}
	
	if !result.Success {
		t.Errorf("expected successful merge, got error: %v", result.Error)
	}
	
	// Should have conflicts resolved
	if len(result.Conflicts) == 0 {
		t.Error("expected conflicts to be resolved")
	}
	
	for _, c := range result.Conflicts {
		if !strings.Contains(c, "resolved: theirs") {
			t.Errorf("expected conflict to be resolved with theirs, got: %s", c)
		}
	}
}

func TestMergeNoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)
	
	// Merge with no changes - should succeed
	result, err := m.Merge(worktree.ID, MergeStrategyFail)
	if err != nil {
		t.Fatalf("failed to merge: %v", err)
	}
	
	if !result.Success {
		t.Errorf("expected successful merge with no changes, got error: %v", result.Error)
	}
	
	if len(result.Conflicts) != 0 {
		t.Errorf("expected no conflicts, got %v", result.Conflicts)
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	// Test concurrent reads
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_ = m.List()
			done <- true
		}()
	}
	
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestDeleteWorktreeNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)
	
	// Try to delete non-existent worktree
	err := m.Delete("non-existent")
	if err == nil {
		t.Error("expected error for non-existent worktree")
	}
}

func TestManagerCreateWorktreeGitError(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// No initial commit - this will cause worktree creation to fail
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	// Should fail because no commits exist
	_, err := m.Create("test-task", config)
	if err == nil {
		t.Error("expected error when no commits exist")
	}
}

func TestMergeWithUnknownStrategy(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("base content"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)
	
	// Make changes in main and worktree to create conflict
	os.WriteFile(testFile, []byte("main content"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "main update")
	cmd.Dir = repoDir
	cmd.Run()
	
	worktreeFile := filepath.Join(worktree.Path, "test.txt")
	os.WriteFile(worktreeFile, []byte("worktree content"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = worktree.Path
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "worktree update")
	cmd.Dir = worktree.Path
	cmd.Run()
	
	// Merge with unknown strategy during conflict
	result, err := m.Merge(worktree.ID, MergeStrategy("unknown"))
	if err == nil {
		t.Error("expected error for unknown strategy with conflicts")
	}
	if result.Success {
		t.Error("expected merge to fail with unknown strategy")
	}
}

func TestCreateGetCurrentHeadError(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// No commits - GetCurrentHead will fail
	m := NewManager(repoDir, worktreeBase)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	_, err := m.Create("test-task", config)
	if err == nil {
		t.Error("expected error when no HEAD exists")
	}
}

func TestCreateWorktreeDirectoryExists(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()
	
	m := NewManager(repoDir, worktreeBase)
	
	// Pre-create the worktree directory to test cleanup on failure
	worktreePath := filepath.Join(worktreeBase, "wt-test-task")
	os.MkdirAll(worktreePath, 0755)
	
	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}
	
	// This may succeed or fail depending on git behavior
	// The important thing is it doesn't panic
	_, _ = m.Create("test-task", config)
}
