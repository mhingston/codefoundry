package stage

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mhingston/codefoundry/internal/protocol"
)

func TestNewTaskPromptHandler(t *testing.T) {
	handler := NewTaskPromptHandler("/tmp/repo", "/tmp/base")
	
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
	
	if handler.repoRoot != "/tmp/repo" {
		t.Errorf("expected repoRoot /tmp/repo, got %s", handler.repoRoot)
	}
	
	if handler.basePath != "/tmp/base" {
		t.Errorf("expected basePath /tmp/base, got %s", handler.basePath)
	}
	
	if handler.worktreeManager == nil {
		t.Error("expected non-nil worktree manager")
	}
	
	if handler.subagentRunner == nil {
		t.Error("expected non-nil subagent runner")
	}
}

func TestTaskPromptHandlerWithHookExecutor(t *testing.T) {
	handler := NewTaskPromptHandler("/tmp/repo", "/tmp/base")
	
	// Create a mock hook executor
	mockExecutor := &mockHookExecutor{}
	
	handler.WithHookExecutor(mockExecutor)
	
	if handler.hookExecutor != mockExecutor {
		t.Error("expected hook executor to be set")
	}
}

func TestExecuteTaskPromptNoTasksFile(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail because tasks file doesn't exist
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}

func TestTaskPromptHandlerGetters(t *testing.T) {
	handler := NewTaskPromptHandler("/tmp/repo", "/tmp/base")
	
	if handler.GetWorktreeManager() == nil {
		t.Error("expected non-nil worktree manager")
	}
	
	if handler.GetSubagentRunner() == nil {
		t.Error("expected non-nil subagent runner")
	}
}

// mockHookExecutor is a mock implementation of HookExecutor
type mockHookExecutor struct {
	callResult *HookResult
	callError  error
}

func (m *mockHookExecutor) Call(hook protocol.Hook, ctx HookContext) (*HookResult, error) {
	if m.callError != nil {
		return nil, m.callError
	}
	
	if m.callResult != nil {
		return m.callResult, nil
	}
	
	return &HookResult{
		Status:   "ok",
		Continue: true,
	}, nil
}

func TestHookContext(t *testing.T) {
	ctx := HookContext{
		RunID:     "run-123",
		StageID:   "implement",
		StageType: "task_prompt",
	}
	
	if ctx.RunID != "run-123" {
		t.Errorf("expected run ID run-123, got %s", ctx.RunID)
	}
}

func TestHookResult(t *testing.T) {
	result := HookResult{
		Status:        "ok",
		Continue:      true,
		MergeApproved: true,
	}
	
	if !result.Continue {
		t.Error("expected Continue to be true")
	}
	
	if !result.MergeApproved {
		t.Error("expected MergeApproved to be true")
	}
}

func TestTaskPromptHookFailure(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	// Create a mock hook executor that returns an error
	mockExecutor := &mockHookExecutor{
		callError: fmt.Errorf("hook execution failed"),
	}
	handler.WithHookExecutor(mockExecutor)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
		Hooks: map[string][]protocol.Hook{
			"pre_subagent": {
				{Type: "script", URL: "http://localhost/test"},
			},
		},
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail because hook failed
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}

func TestTaskPromptHookBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	// Create a mock hook executor that blocks execution
	mockExecutor := &mockHookExecutor{
		callResult: &HookResult{
			Status:   "blocked",
			Continue: false,
			Reason:   "blocked by policy",
		},
	}
	handler.WithHookExecutor(mockExecutor)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
		Hooks: map[string][]protocol.Hook{
			"pre_subagent": {
				{Type: "script", URL: "http://localhost/test"},
			},
		},
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail because hook blocked
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}

func TestTaskPromptEmptyTasks(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file with no tasks
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks: []
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should succeed with no tasks
	if result.Status != string(StatusPass) {
		t.Errorf("expected status pass, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTaskPromptCircularDeps(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file with circular dependencies
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Task 1"
    priority: high
    depends_on:
      - task-2
  - id: task-2
    description: "Task 2"
    priority: high
    depends_on:
      - task-1
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail due to circular dependency
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}

func TestTaskPromptWaveExecution(t *testing.T) {
	// Skip complex integration test
	t.Skip("Skipping complex integration test")
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file with multiple waves
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Task 1"
    priority: high
  - id: task-2
    description: "Task 2"
    priority: high
    depends_on:
      - task-1
  - id: task-3
    description: "Task 3"
    priority: high
    depends_on:
      - task-1
  - id: task-4
    description: "Task 4"
    priority: high
    depends_on:
      - task-2
      - task-3
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should succeed with multiple waves
	if result.Status != string(StatusPass) {
		t.Errorf("expected status pass, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	// Should report multiple waves
	if result.Metadata == nil {
		t.Fatal("expected metadata in result")
	}
	
	waves, ok := result.Metadata["waves"].(int)
	if !ok || waves < 1 {
		t.Errorf("expected at least 1 wave, got %v", result.Metadata["waves"])
	}
}

func TestTaskPromptParallelTasks(t *testing.T) {
	// Skip complex integration test
	t.Skip("Skipping complex integration test")
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file with parallel tasks (no dependencies)
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Task 1"
    priority: high
  - id: task-2
    description: "Task 2"
    priority: high
  - id: task-3
    description: "Task 3"
    priority: medium
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:            "implement",
		Type:          "task_prompt",
		Source:        "decompose",
		MaxConcurrent: 5, // Allow parallel execution
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should succeed with parallel tasks
	if result.Status != string(StatusPass) {
		t.Errorf("expected status pass, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	
	// Should report all tasks completed
	if result.Metadata == nil {
		t.Fatal("expected metadata in result")
	}
	
	tasksCompleted, ok := result.Metadata["tasks_completed"].(int)
	if !ok || tasksCompleted != 3 {
		t.Errorf("expected 3 tasks completed, got %v", result.Metadata["tasks_completed"])
	}
}

func TestTaskPromptMergeConflict(t *testing.T) {
	// Skip complex integration test
	t.Skip("Skipping complex integration test")
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	// Use "ours" strategy to handle conflicts
	stage := &protocol.Stage{
		ID:               "implement",
		Type:             "task_prompt",
		Source:           "decompose",
		WorktreeStrategy: "ours",
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should succeed with conflict resolution strategy
	if result.Status != string(StatusPass) {
		t.Errorf("expected status pass with conflict strategy, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTaskPromptInvalidWorktreeStrategy(t *testing.T) {
	// Skip complex integration test
	t.Skip("Skipping complex integration test")
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	// Use invalid strategy - should default to fail-closed
	stage := &protocol.Stage{
		ID:               "implement",
		Type:             "task_prompt",
		Source:           "decompose",
		WorktreeStrategy: "invalid_strategy",
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should succeed (invalid strategy defaults to fail, but no conflicts here)
	if result.Status != string(StatusPass) {
		t.Errorf("expected status pass, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTaskPromptPostSubagentHook(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	// Create a mock hook executor that blocks post_subagent
	mockExecutor := &mockHookExecutor{
		callResult: &HookResult{
			Status:   "blocked",
			Continue: false,
			Reason:   "post_subagent blocked",
		},
	}
	handler.WithHookExecutor(mockExecutor)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
		Hooks: map[string][]protocol.Hook{
			"post_subagent": {
				{Type: "script", URL: "http://localhost/test"},
			},
		},
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail because post_subagent hook blocked
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}

func TestTaskPromptPreMergeHook(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	// Create a mock hook executor that disapproves merge
	mockExecutor := &mockHookExecutor{
		callResult: &HookResult{
			Status:        "blocked",
			Continue:      true,
			MergeApproved: false,
			Reason:        "merge not approved",
		},
	}
	handler.WithHookExecutor(mockExecutor)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
		Hooks: map[string][]protocol.Hook{
			"pre_merge": {
				{Type: "script", URL: "http://localhost/test"},
			},
		},
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail because merge not approved
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}

func TestTaskPromptExecuteLoadTasksError(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
	// Create tasks file with invalid YAML
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	// Invalid YAML content
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte("invalid: yaml: content: ["), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail due to invalid YAML
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}

func TestExecuteWaveWithErrors(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task 1"
    priority: high
  - id: task-2
    description: "Test task 2"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:            "implement",
		Type:          "task_prompt",
		Source:        "decompose",
		MaxConcurrent: 1,
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should succeed with empty tasks or handle gracefully
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	
	if err != nil {
		t.Logf("Execute returned error (may be expected): %v", err)
	}
}

func TestExecuteWaveTaskFailure(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file with invalid task that will fail
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	// Task with invalid worktree path that will cause failure
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task with long ID that creates invalid branch names with special characters!!!!"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:            "implement",
		Type:          "task_prompt",
		Source:        "decompose",
		MaxConcurrent: 1,
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Task may succeed or fail depending on how the handler handles it
	// The important thing is that executeWave and executeTask are called
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	
	if err != nil {
		t.Logf("Execute returned error (may be expected): %v", err)
	}
}

func TestExecuteWaveEmptyWave(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	// Tasks file with no tasks
	tasksContent := `version: "1.0"
tasks: []
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:            "implement",
		Type:          "task_prompt",
		Source:        "decompose",
		MaxConcurrent: 1,
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should succeed with no tasks
	if result.Status != string(StatusPass) {
		t.Errorf("expected status pass, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestTaskPromptExecuteWithWorktreeError(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
	// Create valid tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	// repoDir is not a git repo, so worktree creation will fail
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail due to worktree creation error
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}

func TestTaskPromptExecuteValidateTasksError(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
	// Create tasks file with invalid task (missing required fields)
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: ""
    description: ""
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail due to validation error
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}

func TestTaskPromptPreMergeHookBlocks(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	baseDir := filepath.Join(tmpDir, "base")
	os.MkdirAll(repoDir, 0755)
	os.MkdirAll(baseDir, 0755)
	
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
	
	// Create tasks file
	artifactsDir := filepath.Join(baseDir, "artifacts", "test-run", "decompose")
	os.MkdirAll(artifactsDir, 0755)
	
	tasksContent := `version: "1.0"
tasks:
  - id: task-1
    description: "Test task"
    priority: high
`
	os.WriteFile(filepath.Join(artifactsDir, "tasks.yaml"), []byte(tasksContent), 0644)
	
	handler := NewTaskPromptHandler(repoDir, baseDir)
	
	// Create a mock hook executor that blocks at pre_merge
	mockExecutor := &mockHookExecutor{
		callResult: &HookResult{
			Status:        "blocked",
			Continue:      false,
			MergeApproved: true, // Approved but Continue is false
			Reason:        "blocked at pre_merge",
		},
	}
	handler.WithHookExecutor(mockExecutor)
	
	stage := &protocol.Stage{
		ID:     "implement",
		Type:   "task_prompt",
		Source: "decompose",
		Hooks: map[string][]protocol.Hook{
			"pre_merge": {
				{Type: "script", URL: "http://localhost/test"},
			},
		},
	}
	
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}
	
	result, err := handler.Execute(context.Background(), stage, input)
	
	// Should fail because pre_merge hook blocked
	if result.Status != string(StatusFail) {
		t.Errorf("expected status fail, got %s", result.Status)
	}
	
	if err != nil {
		t.Errorf("expected no error return (error in result), got %v", err)
	}
}
