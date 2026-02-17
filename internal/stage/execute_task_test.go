package stage

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mhingston/codefoundry/internal/protocol"
	"github.com/mhingston/codefoundry/internal/subagent"
	"github.com/mhingston/codefoundry/internal/worktree"
)

func TestExecuteTask(t *testing.T) {
	tests := []struct {
		name      string
		task      *protocol.Task
		wantErr   bool
		mockSetup func(*MockWorktreeManager, *MockSubagentRunner, *MockHookExecutor)
	}{
		{
			name: "successful task execution",
			task: &protocol.Task{
				ID:          "task-001",
				Title:       "Implement feature",
				Description: "Add authentication middleware",
			},
			wantErr: false,
		},
		{
			name: "task with dependencies",
			task: &protocol.Task{
				ID:           "task-002",
				Title:        "Build on task-001",
				Dependencies: []string{"task-001"},
			},
			wantErr: false,
		},
		{
			name: "worktree creation fails",
			task: &protocol.Task{
				ID:    "task-001",
				Title: "Task",
			},
			wantErr: true,
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				wm.CreateFunc = func(taskID string, config worktree.WorktreeConfig) (*worktree.Worktree, error) {
					return nil, fmt.Errorf("git error")
				}
			},
		},
		{
			name: "subagent spawn fails",
			task: &protocol.Task{
				ID:    "task-001",
				Title: "Task",
			},
			wantErr: true,
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				sr.SpawnFunc = func(req subagent.SpawnRequest) (*subagent.Subagent, error) {
					return nil, fmt.Errorf("spawn failed")
				}
			},
		},
		{
			name: "subagent wait fails",
			task: &protocol.Task{
				ID:    "task-001",
				Title: "Task",
			},
			wantErr: true,
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				sr.WaitFunc = func(ctx context.Context, subagentID string) (*subagent.Result, error) {
					return nil, fmt.Errorf("execution failed")
				}
			},
		},
		{
			name: "merge fails",
			task: &protocol.Task{
				ID:    "task-001",
				Title: "Task",
			},
			wantErr: true,
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				wm.MergeFunc = func(worktreeID string, strategy worktree.MergeStrategy) (*worktree.MergeResult, error) {
					return nil, fmt.Errorf("merge conflict")
				}
			},
		},
		{
			name: "merge returns unsuccessful result",
			task: &protocol.Task{
				ID:    "task-001",
				Title: "Task",
			},
			wantErr: true,
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				wm.MergeFunc = func(worktreeID string, strategy worktree.MergeStrategy) (*worktree.MergeResult, error) {
					return &worktree.MergeResult{
						Success:   false,
						Conflicts: []string{"file.go"},
					}, nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockWorktree := &MockWorktreeManager{}
			mockSubagent := &MockSubagentRunner{}
			mockHooks := &MockHookExecutor{}

			if tt.mockSetup != nil {
				tt.mockSetup(mockWorktree, mockSubagent, mockHooks)
			}

			handler := NewTaskPromptHandlerWithDeps("/tmp/repo", "/tmp/base", mockWorktree, mockSubagent)
			handler.WithHookExecutor(mockHooks)

			ctx := context.Background()
			stage := &protocol.Stage{
				ID:   "implement",
				Type: "task_prompt",
			}
			input := &StageInput{
				StageID: "implement",
				RunID:   "test-run",
			}

			results := make(map[string]*protocol.TaskResult)
			completed := make(map[string]bool)

			var mu sync.Mutex
			err := handler.executeTask(ctx, stage, input, tt.task, worktree.MergeStrategyFail, results, completed, &mu)

			if (err != nil) != tt.wantErr {
				t.Errorf("executeTask() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				foundCleanup := false
				for _, call := range mockWorktree.DeleteCalls {
					if strings.HasPrefix(call, "wt-") {
						foundCleanup = true
						break
					}
				}
				if !foundCleanup {
					t.Error("expected worktree cleanup to be called")
				}
			}
		})
	}
}

func TestExecuteTaskHookSequence(t *testing.T) {
	mockWorktree := &MockWorktreeManager{}
	mockSubagent := &MockSubagentRunner{}
	mockHooks := &MockHookExecutor{
		CallFunc: func(hook protocol.Hook, ctx HookContext) (*HookResult, error) {
			return &HookResult{
				Status:        "ok",
				Continue:      true,
				MergeApproved: true,
			}, nil
		},
	}

	handler := NewTaskPromptHandlerWithDeps("/tmp/repo", "/tmp/base", mockWorktree, mockSubagent)
	handler.WithHookExecutor(mockHooks)

	task := &protocol.Task{
		ID:    "task-001",
		Title: "Test Task",
	}

	stage := &protocol.Stage{
		ID:   "implement",
		Type: "task_prompt",
		Hooks: map[string][]protocol.Hook{
			"pre_subagent":  {{Type: "script", URL: "http://test/pre"}},
			"post_subagent": {{Type: "script", URL: "http://test/post"}},
			"pre_merge":     {{Type: "script", URL: "http://test/merge"}},
		},
	}
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}

	results := make(map[string]*protocol.TaskResult)
	completed := make(map[string]bool)

	err := handler.executeTask(context.Background(), stage, input, task, worktree.MergeStrategyFail, results, completed, &sync.Mutex{})
	if err != nil {
		t.Errorf("executeTask() unexpected error: %v", err)
	}

	if mockHooks.CallCount != 3 {
		t.Errorf("expected 3 hook calls, got %d", mockHooks.CallCount)
	}

	if !completed["task-001"] {
		t.Error("expected task to be marked complete")
	}
}

func TestExecuteTaskWithSubagentResult(t *testing.T) {
	mockWorktree := &MockWorktreeManager{}
	mockSubagent := &MockSubagentRunner{}
	mockHooks := &MockHookExecutor{}

	expectedResult := &subagent.Result{
		Success:      true,
		Output:       "Test output",
		FilesChanged: []string{"test.go", "test_test.go"},
		TurnsUsed:    15,
		TokensUsed:   7500,
		Duration:     2 * time.Second,
		Metadata: map[string]interface{}{
			"test_key": "test_value",
		},
	}

	mockSubagent.WaitFunc = func(ctx context.Context, subagentID string) (*subagent.Result, error) {
		return expectedResult, nil
	}

	handler := NewTaskPromptHandlerWithDeps("/tmp/repo", "/tmp/base", mockWorktree, mockSubagent)
	handler.WithHookExecutor(mockHooks)

	task := &protocol.Task{
		ID:    "task-001",
		Title: "Test Task",
	}

	stage := &protocol.Stage{
		ID:   "implement",
		Type: "task_prompt",
	}
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}

	results := make(map[string]*protocol.TaskResult)
	completed := make(map[string]bool)

	err := handler.executeTask(context.Background(), stage, input, task, worktree.MergeStrategyFail, results, completed, &sync.Mutex{})
	if err != nil {
		t.Errorf("executeTask() unexpected error: %v", err)
	}

	if results["task-001"] == nil {
		t.Fatal("expected result to be stored")
	}

	result := results["task-001"]
	if result.Success != expectedResult.Success {
		t.Errorf("expected Success=%v, got %v", expectedResult.Success, result.Success)
	}
	if result.Output != expectedResult.Output {
		t.Errorf("expected Output=%q, got %q", expectedResult.Output, result.Output)
	}
	if len(result.FilesChanged) != len(expectedResult.FilesChanged) {
		t.Errorf("expected %d files changed, got %d", len(expectedResult.FilesChanged), len(result.FilesChanged))
	}
	if result.TurnsUsed != expectedResult.TurnsUsed {
		t.Errorf("expected TurnsUsed=%d, got %d", expectedResult.TurnsUsed, result.TurnsUsed)
	}
	if result.TokensUsed != expectedResult.TokensUsed {
		t.Errorf("expected TokensUsed=%d, got %d", expectedResult.TokensUsed, result.TokensUsed)
	}
}

func TestExecuteTaskCleanupOnFailure(t *testing.T) {
	mockWorktree := &MockWorktreeManager{}
	mockSubagent := &MockSubagentRunner{}
	mockHooks := &MockHookExecutor{}

	mockSubagent.WaitFunc = func(ctx context.Context, subagentID string) (*subagent.Result, error) {
		return nil, fmt.Errorf("execution failed")
	}

	handler := NewTaskPromptHandlerWithDeps("/tmp/repo", "/tmp/base", mockWorktree, mockSubagent)
	handler.WithHookExecutor(mockHooks)

	task := &protocol.Task{
		ID:    "task-001",
		Title: "Test Task",
	}

	stage := &protocol.Stage{
		ID:   "implement",
		Type: "task_prompt",
	}
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}

	results := make(map[string]*protocol.TaskResult)
	completed := make(map[string]bool)

	err := handler.executeTask(context.Background(), stage, input, task, worktree.MergeStrategyFail, results, completed, &sync.Mutex{})
	if err == nil {
		t.Error("expected error")
	}

	if len(mockWorktree.DeleteCalls) == 0 {
		t.Error("expected worktree cleanup to be called even on failure")
	}
}

func TestExecuteTaskWithoutHooks(t *testing.T) {
	mockWorktree := &MockWorktreeManager{}
	mockSubagent := &MockSubagentRunner{}

	handler := NewTaskPromptHandlerWithDeps("/tmp/repo", "/tmp/base", mockWorktree, mockSubagent)

	task := &protocol.Task{
		ID:    "task-001",
		Title: "Test Task",
	}

	stage := &protocol.Stage{
		ID:   "implement",
		Type: "task_prompt",
	}
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}

	results := make(map[string]*protocol.TaskResult)
	completed := make(map[string]bool)

	err := handler.executeTask(context.Background(), stage, input, task, worktree.MergeStrategyFail, results, completed, &sync.Mutex{})
	if err != nil {
		t.Errorf("executeTask() unexpected error: %v", err)
	}

	if !completed["task-001"] {
		t.Error("expected task to be marked complete")
	}
}

func TestExecuteTaskResultStorage(t *testing.T) {
	mockWorktree := &MockWorktreeManager{}
	mockSubagent := &MockSubagentRunner{}

	expectedResult := &subagent.Result{
		Success:      true,
		Output:       "Output content",
		FilesChanged: []string{"file.go"},
		TurnsUsed:    20,
		TokensUsed:   10000,
		Duration:     5 * time.Second,
	}

	mockSubagent.WaitFunc = func(ctx context.Context, subagentID string) (*subagent.Result, error) {
		return expectedResult, nil
	}

	handler := NewTaskPromptHandlerWithDeps("/tmp/repo", "/tmp/base", mockWorktree, mockSubagent)

	task := &protocol.Task{
		ID:    "task-001",
		Title: "Test Task",
	}

	stage := &protocol.Stage{
		ID:   "implement",
		Type: "task_prompt",
	}
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}

	results := make(map[string]*protocol.TaskResult)
	completed := make(map[string]bool)

	err := handler.executeTask(context.Background(), stage, input, task, worktree.MergeStrategyFail, results, completed, &sync.Mutex{})
	if err != nil {
		t.Fatalf("executeTask() unexpected error: %v", err)
	}

	result, ok := results["task-001"]
	if !ok {
		t.Fatal("expected result to be stored in map")
	}

	if result.TaskID != "task-001" {
		t.Errorf("expected TaskID=task-001, got %s", result.TaskID)
	}
	if result.Success != expectedResult.Success {
		t.Errorf("expected Success=%v, got %v", expectedResult.Success, result.Success)
	}
	if result.Output != expectedResult.Output {
		t.Errorf("expected Output=%q, got %q", expectedResult.Output, result.Output)
	}
	if result.TurnsUsed != expectedResult.TurnsUsed {
		t.Errorf("expected TurnsUsed=%d, got %d", expectedResult.TurnsUsed, result.TurnsUsed)
	}
	if result.TokensUsed != expectedResult.TokensUsed {
		t.Errorf("expected TokensUsed=%d, got %d", expectedResult.TokensUsed, result.TokensUsed)
	}
	if result.DurationMs != expectedResult.Duration.Milliseconds() {
		t.Errorf("expected DurationMs=%d, got %d", expectedResult.Duration.Milliseconds(), result.DurationMs)
	}

	if !completed["task-001"] {
		t.Error("expected task to be marked as completed")
	}
}
