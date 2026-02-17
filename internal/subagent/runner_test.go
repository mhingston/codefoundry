package subagent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestNewRunner(t *testing.T) {
	runner := NewRunner("/tmp/test")
	if runner == nil {
		t.Fatal("expected non-nil runner")
	}
	
	if runner.basePath != "/tmp/test" {
		t.Errorf("expected basePath /tmp/test, got %s", runner.basePath)
	}
	
	if runner.emitter == nil {
		t.Error("expected non-nil emitter")
	}
}

func TestSpawn(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   5 * time.Second,
	}
	
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "/tmp/worktree",
		Limits:       limits,
		Prompt:       "test prompt",
	}
	
	subagent, err := runner.Spawn(req)
	if err != nil {
		t.Fatalf("failed to spawn subagent: %v", err)
	}
	
	if subagent == nil {
		t.Fatal("expected non-nil subagent")
	}
	
	if subagent.TaskID != "test-task" {
		t.Errorf("expected task ID test-task, got %s", subagent.TaskID)
	}
	
	if subagent.Status != StatusPending {
		t.Errorf("expected status pending, got %s", subagent.Status)
	}
	
	if subagent.Limits.MaxTurns != 10 {
		t.Errorf("expected max turns 10, got %d", subagent.Limits.MaxTurns)
	}
	
	if !contains(subagent.ID, "subagent-test-task-") {
		t.Errorf("expected ID to contain 'subagent-test-task-', got %s", subagent.ID)
	}
}

func TestSpawnInvalidLimits(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	tests := []struct {
		name   string
		limits Limits
	}{
		{
			name:   "zero turns",
			limits: Limits{MaxTurns: 0, MaxTokens: 100, Timeout: time.Second},
		},
		{
			name:   "zero tokens",
			limits: Limits{MaxTurns: 10, MaxTokens: 0, Timeout: time.Second},
		},
		{
			name:   "zero timeout",
			limits: Limits{MaxTurns: 10, MaxTokens: 100, Timeout: 0},
		},
		{
			name:   "negative memory",
			limits: Limits{MaxTurns: 10, MaxTokens: 100, Timeout: time.Second, MemoryMB: -1},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := SpawnRequest{
				TaskID:       "test-task",
				WorktreePath: "/tmp/worktree",
				Limits:       tt.limits,
			}
			
			_, err := runner.Spawn(req)
			if err == nil {
				t.Error("expected error for invalid limits")
			}
		})
	}
}

func TestStatus(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   5 * time.Second,
	}
	
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "/tmp/worktree",
		Limits:       limits,
	}
	
	subagent, _ := runner.Spawn(req)
	
	status, err := runner.Status(subagent.ID)
	if err != nil {
		t.Fatalf("failed to get status: %v", err)
	}
	
	if status.ID != subagent.ID {
		t.Errorf("expected ID %s, got %s", subagent.ID, status.ID)
	}
	
	if status.TaskID != "test-task" {
		t.Errorf("expected task ID test-task, got %s", status.TaskID)
	}
	
	if status.Status != StatusPending {
		t.Errorf("expected status pending, got %s", status.Status)
	}
}

func TestStatusNonExistent(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	_, err := runner.Status("non-existent")
	if err == nil {
		t.Error("expected error for non-existent subagent")
	}
}

func TestAbort(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   5 * time.Second,
	}
	
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "/tmp/worktree",
		Limits:       limits,
	}
	
	subagent, _ := runner.Spawn(req)
	
	// Start the subagent
	ctx := context.Background()
	runner.startSubagent(ctx, subagent)
	
	// Abort it
	err := runner.Abort(subagent.ID)
	if err != nil {
		t.Fatalf("failed to abort: %v", err)
	}
	
	// Wait a bit for abort to complete
	time.Sleep(100 * time.Millisecond)
	
	status, _ := runner.Status(subagent.ID)
	// Status could be aborted or completed (due to race with simulated execution)
	if status.Status != StatusAborted && status.Status != StatusCompleted {
		t.Errorf("expected status aborted or completed, got %s", status.Status)
	}
}

func TestList(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   5 * time.Second,
	}
	
	// Initially empty
	list := runner.List()
	if len(list) != 0 {
		t.Errorf("expected 0 subagents, got %d", len(list))
	}
	
	// Spawn a subagent
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "/tmp/worktree",
		Limits:       limits,
	}
	
	runner.Spawn(req)
	
	// Now should have 1
	list = runner.List()
	if len(list) != 1 {
		t.Errorf("expected 1 subagent, got %d", len(list))
	}
}

func TestCleanup(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   5 * time.Second,
	}
	
	// Spawn subagents
	for i := 0; i < 3; i++ {
		req := SpawnRequest{
			TaskID:       "test-task",
			WorktreePath: "/tmp/worktree",
			Limits:       limits,
		}
		runner.Spawn(req)
	}
	
	if len(runner.List()) != 3 {
		t.Errorf("expected 3 subagents, got %d", len(runner.List()))
	}
	
	// Cleanup - since none are completed, should still have 3
	runner.Cleanup()
	
	if len(runner.List()) != 3 {
		t.Errorf("expected 3 subagents after cleanup (none completed), got %d", len(runner.List()))
	}
}

func TestGetResult(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   5 * time.Second,
	}
	
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "/tmp/worktree",
		Limits:       limits,
	}
	
	subagent, _ := runner.Spawn(req)
	
	// Result not available yet
	_, err := runner.GetResult(subagent.ID)
	if err == nil {
		t.Error("expected error for incomplete subagent")
	}
}

func TestGetResultNonExistent(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	_, err := runner.GetResult("non-existent")
	if err == nil {
		t.Error("expected error for non-existent subagent")
	}
}

func TestAbortNonExistent(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	err := runner.Abort("non-existent")
	if err == nil {
		t.Error("expected error for non-existent subagent")
	}
}

func TestRunnerTimeout(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   100 * time.Millisecond, // Very short timeout
	}
	
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "/tmp/worktree",
		Limits:       limits,
		Prompt:       "test prompt",
	}
	
	subagent, err := runner.Spawn(req)
	if err != nil {
		t.Fatalf("failed to spawn subagent: %v", err)
	}
	
	// Manually set subagent to running but don't complete it
	subagent.mu.Lock()
	subagent.Status = StatusRunning
	now := time.Now().UTC()
	subagent.StartedAt = &now
	subagent.mu.Unlock()
	
	// Wait for the subagent to timeout
	ctx := context.Background()
	_, err = runner.Wait(ctx, subagent.ID)
	
	// Should get timeout error
	if err == nil {
		t.Error("expected timeout error")
	}
	
	// Check status - should be aborted or timeout
	status, _ := runner.Status(subagent.ID)
	if status.Status != StatusAborted && status.Status != StatusTimeout {
		t.Errorf("expected status aborted or timeout, got %s", status.Status)
	}
}

func TestRunnerAbort(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  100,
		MaxTokens: 10000,
		Timeout:   30 * time.Second,
	}
	
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "/tmp/worktree",
		Limits:       limits,
		Prompt:       "test prompt",
	}
	
	subagent, err := runner.Spawn(req)
	if err != nil {
		t.Fatalf("failed to spawn subagent: %v", err)
	}
	
	// Start the subagent
	ctx := context.Background()
	runner.startSubagent(ctx, subagent)
	
	// Give it a moment to start
	time.Sleep(50 * time.Millisecond)
	
	// Abort the subagent
	err = runner.Abort(subagent.ID)
	if err != nil {
		t.Fatalf("failed to abort subagent: %v", err)
	}
	
	// Give it a moment to abort
	time.Sleep(50 * time.Millisecond)
	
	// Check status - should be aborted
	status, _ := runner.Status(subagent.ID)
	if status.Status != StatusAborted && status.Status != StatusCompleted {
		t.Errorf("expected status aborted or completed, got %s", status.Status)
	}
}

func TestRunnerAbortNotFound(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	err := runner.Abort("non-existent-id")
	if err == nil {
		t.Error("expected error for non-existent subagent")
	}
}

func TestRunnerConcurrentSpawns(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   5 * time.Second,
	}
	
	// Spawn multiple subagents concurrently
	numSubagents := 10
	subagents := make([]*Subagent, numSubagents)
	
	for i := 0; i < numSubagents; i++ {
		req := SpawnRequest{
			TaskID:       fmt.Sprintf("task-%d", i),
			WorktreePath: fmt.Sprintf("/tmp/worktree-%d", i),
			Limits:       limits,
			Prompt:       fmt.Sprintf("prompt %d", i),
		}
		
		subagent, err := runner.Spawn(req)
		if err != nil {
			t.Fatalf("failed to spawn subagent %d: %v", i, err)
		}
		subagents[i] = subagent
	}
	
	// Verify all subagents exist
	list := runner.List()
	if len(list) != numSubagents {
		t.Errorf("expected %d subagents, got %d", numSubagents, len(list))
	}
	
	// Verify each subagent has unique ID
	idMap := make(map[string]bool)
	for _, s := range subagents {
		if idMap[s.ID] {
			t.Errorf("duplicate subagent ID: %s", s.ID)
		}
		idMap[s.ID] = true
	}
}

func TestRunnerInvalidWorktree(t *testing.T) {
	// Testing error paths for invalid worktree paths
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   5 * time.Second,
	}
	
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "", // Invalid empty path
		Limits:       limits,
		Prompt:       "test prompt",
	}
	
	// Spawn should still succeed (path validation is not in Spawn)
	subagent, err := runner.Spawn(req)
	if err != nil {
		t.Fatalf("failed to spawn subagent: %v", err)
	}
	
	if subagent.Worktree != "" {
		t.Errorf("expected empty worktree path, got %s", subagent.Worktree)
	}
}

func TestRunnerWithEmitter(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	// Create a custom emitter
	customEmitter := NewEmitter()
	
	// Set custom emitter
	result := runner.WithEmitter(customEmitter)
	
	// Should return the same runner for chaining
	if result != runner {
		t.Error("expected WithEmitter to return the same runner")
	}
	
	// Verify emitter was set
	if runner.emitter != customEmitter {
		t.Error("expected emitter to be set to custom emitter")
	}
}

func TestRunnerWaitNotFound(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	ctx := context.Background()
	_, err := runner.Wait(ctx, "non-existent-id")
	if err == nil {
		t.Error("expected error for non-existent subagent")
	}
}

func TestRunnerStatusNotFound(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	_, err := runner.Status("non-existent-id")
	if err == nil {
		t.Error("expected error for non-existent subagent")
	}
}

func TestRunnerGetResultCompleted(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   5 * time.Second,
	}
	
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "/tmp/worktree",
		Limits:       limits,
	}
	
	subagent, _ := runner.Spawn(req)
	
	// Manually set result
	subagent.mu.Lock()
	subagent.Status = StatusCompleted
	subagent.Result = &Result{
		Success: true,
		Output:  "test output",
	}
	subagent.mu.Unlock()
	
	result, err := runner.GetResult(subagent.ID)
	if err != nil {
		t.Fatalf("failed to get result: %v", err)
	}
	
	if !result.Success {
		t.Error("expected result to be successful")
	}
	
	if result.Output != "test output" {
		t.Errorf("expected output 'test output', got %s", result.Output)
	}
}

func TestRunnerWaitResourceLimits(t *testing.T) {
	runner := NewRunner("/tmp/test")
	
	limits := Limits{
		MaxTurns:  1,
		MaxTokens: 1000,
		Timeout:   30 * time.Second,
	}
	
	req := SpawnRequest{
		TaskID:       "test-task",
		WorktreePath: "/tmp/worktree",
		Limits:       limits,
		Prompt:       "test prompt",
	}
	
	subagent, err := runner.Spawn(req)
	if err != nil {
		t.Fatalf("failed to spawn subagent: %v", err)
	}
	
	// Manually set to running and set usage to exceed limits
	subagent.mu.Lock()
	subagent.Status = StatusRunning
	now := time.Now().UTC()
	subagent.StartedAt = &now
	subagent.Usage = Usage{
		TurnsUsed:  100, // Exceeds MaxTurns of 1
		TokensUsed: 0,
	}
	subagent.mu.Unlock()
	
	// Wait should fail due to resource limit
	ctx := context.Background()
	_, err = runner.Wait(ctx, subagent.ID)
	
	if err == nil {
		t.Error("expected error due to resource limit exceeded")
	}
}

func TestGetFilesChanged(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = tmpDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()
	
	// Make a change
	os.WriteFile(testFile, []byte("hello world"), 0644)
	
	// Get changed files
	files, err := GetFilesChanged(tmpDir)
	if err != nil {
		t.Fatalf("GetFilesChanged() error = %v", err)
	}
	
	if len(files) != 1 {
		t.Errorf("expected 1 changed file, got %d", len(files))
	}
}

func TestGetFilesChangedNoChanges(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}
	
	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = tmpDir
	cmd.Run()
	
	// Create initial commit
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	cmd.Run()
	
	// Get changed files (no changes)
	files, err := GetFilesChanged(tmpDir)
	if err != nil {
		t.Fatalf("GetFilesChanged() error = %v", err)
	}
	
	if len(files) != 0 {
		t.Errorf("expected 0 changed files, got %d", len(files))
	}
}

func TestGetFilesChangedError(t *testing.T) {
	// Test with non-existent directory
	_, err := GetFilesChanged("/non-existent-path")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "line1\nline2\nline3",
			expected: []string{"line1", "line2", "line3"},
		},
		{
			input:    "single",
			expected: []string{"single"},
		},
		{
			input:    "",
			expected: []string{},
		},
		{
			input:    "line1\n",
			expected: []string{"line1"},
		},
		{
			input:    "\nline2",
			expected: []string{"", "line2"},
		},
	}
	
	for _, tt := range tests {
		result := splitLines(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("splitLines(%q) returned %d lines, expected %d", tt.input, len(result), len(tt.expected))
			continue
		}
		for i, exp := range tt.expected {
			if result[i] != exp {
				t.Errorf("splitLines(%q)[%d] = %q, expected %q", tt.input, i, result[i], exp)
			}
		}
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "  hello  ",
			expected: "hello",
		},
		{
			input:    "\t\thello\t\t",
			expected: "hello",
		},
		{
			input:    "\n\rhello\r\n",
			expected: "hello",
		},
		{
			input:    "  \t \n hello world \r \t ",
			expected: "hello world",
		},
		{
			input:    "no-spaces",
			expected: "no-spaces",
		},
		{
			input:    "",
			expected: "",
		},
		{
			input:    "   ",
			expected: "",
		},
	}
	
	for _, tt := range tests {
		result := trimSpace(tt.input)
		if result != tt.expected {
			t.Errorf("trimSpace(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}
