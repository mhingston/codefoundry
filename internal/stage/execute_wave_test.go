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

func TestExecuteWave(t *testing.T) {
	tests := []struct {
		name        string
		taskIDs     []string
		wantErr     bool
		errContains string
		mockSetup   func(*MockWorktreeManager, *MockSubagentRunner, *MockHookExecutor)
	}{
		{
			name:    "successful single task",
			taskIDs: []string{"task-001"},
			wantErr: false,
		},
		{
			name:    "successful multiple tasks",
			taskIDs: []string{"task-001", "task-002", "task-003"},
			wantErr: false,
		},
		{
			name:        "worktree creation fails",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "failed to create worktree",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				wm.CreateFunc = func(taskID string, config worktree.WorktreeConfig) (*worktree.Worktree, error) {
					return nil, fmt.Errorf("git error: no commits")
				}
			},
		},
		{
			name:        "subagent spawn fails",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "failed to spawn subagent",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				sr.SpawnFunc = func(req subagent.SpawnRequest) (*subagent.Subagent, error) {
					return nil, fmt.Errorf("connection refused")
				}
			},
		},
		{
			name:        "subagent execution fails",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "subagent failed",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				sr.WaitFunc = func(ctx context.Context, subagentID string) (*subagent.Result, error) {
					return nil, fmt.Errorf("execution timeout")
				}
			},
		},
		{
			name:        "merge conflict with fail strategy",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "merge failed",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				wm.MergeFunc = func(worktreeID string, strategy worktree.MergeStrategy) (*worktree.MergeResult, error) {
					return &worktree.MergeResult{
						Success:   false,
						Strategy:  strategy,
						Conflicts: []string{"file.go"},
					}, nil
				}
			},
		},
		{
			name:        "merge returns error",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "merge failed",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				wm.MergeFunc = func(worktreeID string, strategy worktree.MergeStrategy) (*worktree.MergeResult, error) {
					return nil, fmt.Errorf("git merge error")
				}
			},
		},
		{
			name:        "pre_subagent hook blocks execution",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "pre_subagent hook blocked",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				he.CallFunc = func(hook protocol.Hook, ctx HookContext) (*HookResult, error) {
					return &HookResult{
						Status:   "blocked",
						Continue: false,
						Reason:   "validation failed",
					}, nil
				}
			},
		},
		{
			name:        "pre_subagent hook returns error",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "pre_subagent hook failed",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				he.CallFunc = func(hook protocol.Hook, ctx HookContext) (*HookResult, error) {
					return nil, fmt.Errorf("hook execution error")
				}
			},
		},
		{
			name:        "post_subagent hook blocks execution",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "post_subagent hook blocked",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				callCount := 0
				he.CallFunc = func(hook protocol.Hook, ctx HookContext) (*HookResult, error) {
					callCount++
					if callCount == 1 {
						return &HookResult{Status: "ok", Continue: true}, nil
					}
					return &HookResult{
						Status:   "blocked",
						Continue: false,
						Reason:   "post validation failed",
					}, nil
				}
			},
		},
		{
			name:        "pre_merge hook blocks execution",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "pre_merge hook blocked",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				callCount := 0
				he.CallFunc = func(hook protocol.Hook, ctx HookContext) (*HookResult, error) {
					callCount++
					if callCount <= 2 {
						return &HookResult{Status: "ok", Continue: true}, nil
					}
					return &HookResult{
						Status:   "blocked",
						Continue: false,
						Reason:   "merge validation failed",
					}, nil
				}
			},
		},
		{
			name:        "pre_merge hook not approved",
			taskIDs:     []string{"task-001"},
			wantErr:     true,
			errContains: "merge blocked by hook",
			mockSetup: func(wm *MockWorktreeManager, sr *MockSubagentRunner, he *MockHookExecutor) {
				callCount := 0
				he.CallFunc = func(hook protocol.Hook, ctx HookContext) (*HookResult, error) {
					callCount++
					if callCount <= 2 {
						return &HookResult{Status: "ok", Continue: true}, nil
					}
					return &HookResult{
						Status:        "blocked",
						Continue:      true,
						MergeApproved: false,
						Reason:        "manual approval required",
					}, nil
				}
			},
		},

		{
			name:    "empty task list",
			taskIDs: []string{},
			wantErr: false,
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

			var tasks []protocol.Task
			for _, taskID := range tt.taskIDs {
				tasks = append(tasks, protocol.Task{
					ID:    taskID,
					Title: "Task " + taskID,
				})
			}

			dag := protocol.NewTaskDAG(tasks)

			ctx := context.Background()
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

			err := handler.executeWave(ctx, stage, input, dag, tt.taskIDs, worktree.MergeStrategyFail, results, completed, &sync.Mutex{})

			if (err != nil) != tt.wantErr {
				t.Errorf("executeWave() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("executeWave() error = %v, should contain %q", err, tt.errContains)
				}
			}

			if !tt.wantErr && len(tt.taskIDs) > 0 {
				if len(mockWorktree.DeleteCalls) != len(tt.taskIDs) {
					t.Errorf("expected %d Delete calls, got %d", len(tt.taskIDs), len(mockWorktree.DeleteCalls))
				}
			}
		})
	}
}

func TestExecuteWaveConcurrency(t *testing.T) {
	mockWorktree := &MockWorktreeManager{}
	mockSubagent := &MockSubagentRunner{}
	mockHooks := &MockHookExecutor{}

	mockSubagent.WaitFunc = func(ctx context.Context, subagentID string) (*subagent.Result, error) {
		time.Sleep(50 * time.Millisecond)
		return &subagent.Result{Success: true}, nil
	}

	handler := NewTaskPromptHandlerWithDeps("/tmp/repo", "/tmp/base", mockWorktree, mockSubagent)
	handler.WithHookExecutor(mockHooks)

	taskIDs := []string{"task-001", "task-002", "task-003", "task-004", "task-005"}
	var tasks []protocol.Task
	for _, taskID := range taskIDs {
		tasks = append(tasks, protocol.Task{
			ID:    taskID,
			Title: "Task " + taskID,
		})
	}

	dag := protocol.NewTaskDAG(tasks)

	stage := &protocol.Stage{
		ID:            "implement",
		Type:          "task_prompt",
		MaxConcurrent: 5,
	}
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}

	results := make(map[string]*protocol.TaskResult)
	completed := make(map[string]bool)

	start := time.Now()
	err := handler.executeWave(context.Background(), stage, input, dag, taskIDs, worktree.MergeStrategyFail, results, completed, &sync.Mutex{})
	duration := time.Since(start)

	if err != nil {
		t.Errorf("executeWave() unexpected error: %v", err)
	}

	if duration > 200*time.Millisecond {
		t.Logf("Warning: execution took %v, may not be fully concurrent", duration)
	}

	if len(completed) != len(taskIDs) {
		t.Errorf("expected %d completed tasks, got %d", len(taskIDs), len(completed))
	}
}

func TestExecuteWavePartialFailure(t *testing.T) {
	mockWorktree := &MockWorktreeManager{}
	mockSubagent := &MockSubagentRunner{}
	mockHooks := &MockHookExecutor{}

	callCount := 0
	mockSubagent.WaitFunc = func(ctx context.Context, subagentID string) (*subagent.Result, error) {
		callCount++
		if callCount == 2 {
			return nil, fmt.Errorf("task 2 failed")
		}
		return &subagent.Result{Success: true}, nil
	}

	handler := NewTaskPromptHandlerWithDeps("/tmp/repo", "/tmp/base", mockWorktree, mockSubagent)
	handler.WithHookExecutor(mockHooks)

	taskIDs := []string{"task-001", "task-002", "task-003"}
	var tasks []protocol.Task
	for _, taskID := range taskIDs {
		tasks = append(tasks, protocol.Task{
			ID:    taskID,
			Title: "Task " + taskID,
		})
	}

	dag := protocol.NewTaskDAG(tasks)

	stage := &protocol.Stage{
		ID:            "implement",
		Type:          "task_prompt",
		MaxConcurrent: 3,
	}
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}

	results := make(map[string]*protocol.TaskResult)
	completed := make(map[string]bool)

	err := handler.executeWave(context.Background(), stage, input, dag, taskIDs, worktree.MergeStrategyFail, results, completed, &sync.Mutex{})
	if err == nil {
		t.Error("expected error due to task failure")
	}

	if len(mockWorktree.DeleteCalls) != len(taskIDs) {
		t.Errorf("expected %d Delete calls, got %d", len(taskIDs), len(mockWorktree.DeleteCalls))
	}
}

func TestExecuteWaveEmptyTasks(t *testing.T) {
	mockWorktree := &MockWorktreeManager{}
	mockSubagent := &MockSubagentRunner{}

	handler := NewTaskPromptHandlerWithDeps("/tmp/repo", "/tmp/base", mockWorktree, mockSubagent)

	stage := &protocol.Stage{
		ID:   "implement",
		Type: "task_prompt",
	}
	input := &StageInput{
		StageID: "implement",
		RunID:   "test-run",
	}

	dag := protocol.NewTaskDAG([]protocol.Task{})
	results := make(map[string]*protocol.TaskResult)
	completed := make(map[string]bool)

	err := handler.executeWave(context.Background(), stage, input, dag, []string{}, worktree.MergeStrategyFail, results, completed, &sync.Mutex{})
	if err != nil {
		t.Errorf("executeWave() unexpected error: %v", err)
	}

	if len(mockWorktree.CreateCalls) > 0 {
		t.Error("expected no Create calls for empty task list")
	}
}
