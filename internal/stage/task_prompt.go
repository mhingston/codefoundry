package stage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/mhingston/codefoundry/internal/protocol"
	"github.com/mhingston/codefoundry/internal/subagent"
	"github.com/mhingston/codefoundry/internal/worktree"
)

// TaskPromptHandler handles task_prompt stage execution
type TaskPromptHandler struct {
	worktreeManager WorktreeManager
	subagentRunner  SubagentRunner
	repoRoot        string
	basePath        string
	hookExecutor    HookExecutor
}

// HookExecutor defines the interface for executing hooks
type HookExecutor interface {
	Call(hook protocol.Hook, ctx HookContext) (*HookResult, error)
}

// HookContext contains context for hook execution
type HookContext struct {
	RunID         string
	StageID       string
	StageType     string
	Task          *protocol.Task
	Worktree      *worktree.Worktree
	SubagentResult *subagent.Result
	Limits        subagent.Limits
}

// HookResult contains the result of hook execution
type HookResult struct {
	Status        string
	Continue      bool
	Reason        string
	MergeApproved bool
	Overrides     struct {
		Limits       subagent.Limits
		TemplateVars map[string]string
	}
}

// NewTaskPromptHandler creates a new task_prompt handler with real implementations
func NewTaskPromptHandler(repoRoot, basePath string) *TaskPromptHandler {
	worktreeBase := filepath.Join(basePath, "worktrees")

	return &TaskPromptHandler{
		worktreeManager: worktree.NewManager(repoRoot, worktreeBase),
		subagentRunner:  subagent.NewRunner(basePath),
		repoRoot:        repoRoot,
		basePath:        basePath,
	}
}

// NewTaskPromptHandlerWithDeps creates a new task_prompt handler with injected dependencies
func NewTaskPromptHandlerWithDeps(
	repoRoot,
	basePath string,
	worktreeMgr WorktreeManager,
	subagentRunner SubagentRunner,
) *TaskPromptHandler {
	return &TaskPromptHandler{
		worktreeManager: worktreeMgr,
		subagentRunner:  subagentRunner,
		repoRoot:        repoRoot,
		basePath:        basePath,
	}
}

// WithHookExecutor sets a custom hook executor
func (h *TaskPromptHandler) WithHookExecutor(executor HookExecutor) *TaskPromptHandler {
	h.hookExecutor = executor
	return h
}

// Execute runs the task_prompt stage
func (h *TaskPromptHandler) Execute(ctx context.Context, stage *protocol.Stage, input *StageInput) (*StageResult, error) {
	// Load tasks from source stage
	tasksPath := filepath.Join(h.basePath, "artifacts", input.RunID, stage.Source, "tasks.yaml")
	
	// Check if tasks file exists
	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		return &StageResult{
			Status: string(StatusFail),
			Error:  fmt.Errorf("tasks file not found: %s", tasksPath),
		}, nil
	}
	
	tasksFile, err := protocol.LoadTasks(tasksPath)
	if err != nil {
		return &StageResult{
			Status: string(StatusFail),
			Error:  fmt.Errorf("failed to load tasks: %w", err),
		}, nil
	}
	
	// Validate tasks
	if err := tasksFile.Validate(); err != nil {
		return &StageResult{
			Status: string(StatusFail),
			Error:  fmt.Errorf("task validation failed: %w", err),
		}, nil
	}
	
	// Build DAG and topological sort
	dag := tasksFile.GetDAG()
	waves, err := dag.TopologicalSort()
	if err != nil {
		return &StageResult{
			Status: string(StatusFail),
			Error:  fmt.Errorf("failed to sort tasks: %w", err),
		}, nil
	}
	
	// Determine merge strategy
	strategy, err := worktree.ValidateMergeStrategy(stage.WorktreeStrategy)
	if err != nil {
		strategy = worktree.MergeStrategyFail // Default to fail-closed
	}
	
	// Execute tasks in waves
	results := make(map[string]*protocol.TaskResult)
	completed := make(map[string]bool)
	var resultMu sync.Mutex
	
	for waveIdx, wave := range waves {
		// Execute tasks in this wave in parallel
		err := h.executeWave(ctx, stage, input, dag, wave, strategy, results, completed, &resultMu)
		if err != nil {
			return &StageResult{
				Status: string(StatusFail),
				Error:  fmt.Errorf("wave %d failed: %w", waveIdx+1, err),
			}, nil
		}
	}
	
	// Build result
	summary := fmt.Sprintf("Completed %d tasks in %d waves", len(tasksFile.Tasks), len(waves))
	
	return &StageResult{
		Status:   string(StatusPass),
		Summary:  summary,
		Outputs:  stage.Outputs,
		Metadata: map[string]interface{}{
			"tasks_completed": len(results),
			"waves":           len(waves),
		},
	}, nil
}

// executeWave executes tasks in a single wave
func (h *TaskPromptHandler) executeWave(
	ctx context.Context,
	stage *protocol.Stage,
	input *StageInput,
	dag *protocol.TaskDAG,
	wave []string,
	strategy worktree.MergeStrategy,
	results map[string]*protocol.TaskResult,
	completed map[string]bool,
	resultMu *sync.Mutex,
) error {
	// Limit concurrency
	maxConcurrent := stage.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = 5
	}
	
	// Create semaphore for limiting concurrency
	semaphore := make(chan struct{}, maxConcurrent)
	
	var wg sync.WaitGroup
	errors := make(chan error, len(wave))
	
	for _, taskID := range wave {
		task, err := dag.GetTask(taskID)
		if err != nil {
			return err
		}
		
		wg.Add(1)
		go func(t *protocol.Task) {
			defer wg.Done()
			
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			if err := h.executeTask(ctx, stage, input, t, strategy, results, completed, resultMu); err != nil {
				errors <- err
			}
		}(task)
	}
	
	wg.Wait()
	close(errors)
	
	// Check for errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	
	if len(errs) > 0 {
		if len(errs) == 1 {
			return errs[0]
		}
		return fmt.Errorf("%d task(s) failed in wave: %v", len(errs), errs[0])
	}
	
	return nil
}

// executeTask executes a single task
func (h *TaskPromptHandler) executeTask(
	ctx context.Context,
	stage *protocol.Stage,
	input *StageInput,
	task *protocol.Task,
	strategy worktree.MergeStrategy,
	results map[string]*protocol.TaskResult,
	completed map[string]bool,
	resultMu *sync.Mutex,
) error {
	// Create worktree for task
	config := worktree.WorktreeConfig{
		TaskID:     task.ID,
		BaseBranch: "main",
	}
	
	wt, err := h.worktreeManager.Create(task.ID, config)
	if err != nil {
		return fmt.Errorf("failed to create worktree for task %s: %w", task.ID, err)
	}
	
	// Ensure cleanup
	defer h.worktreeManager.Delete(wt.ID)
	
	// Call pre_subagent hook if configured
	if stage.Hooks != nil && len(stage.Hooks["pre_subagent"]) > 0 {
		for _, hook := range stage.Hooks["pre_subagent"] {
			if h.hookExecutor != nil {
				hookCtx := HookContext{
					RunID:    input.RunID,
					StageID:  stage.ID,
					StageType: stage.Type,
					Task:     task,
					Worktree: wt,
					Limits:   subagent.DefaultLimits(),
				}
				
				result, err := h.hookExecutor.Call(hook, hookCtx)
				if err != nil {
					return fmt.Errorf("pre_subagent hook failed for task %s: %w", task.ID, err)
				}
				
				if !result.Continue {
					return fmt.Errorf("pre_subagent hook blocked task %s: %s", task.ID, result.Reason)
				}
			}
		}
	}
	
	// Spawn subagent
	limits := subagent.DefaultLimits()
	
	req := subagent.SpawnRequest{
		TaskID:       task.ID,
		WorktreePath: wt.Path,
		Limits:       limits,
		Prompt:       task.Description,
		TemplateVars: task.TemplateVars,
	}
	
	subagent, err := h.subagentRunner.Spawn(req)
	if err != nil {
		return fmt.Errorf("failed to spawn subagent for task %s: %w", task.ID, err)
	}
	
	// Wait for subagent completion
	subagentResult, err := h.subagentRunner.Wait(ctx, subagent.ID)
	if err != nil {
		return fmt.Errorf("subagent failed for task %s: %w", task.ID, err)
	}
	
	// Call post_subagent hook if configured
	if stage.Hooks != nil && len(stage.Hooks["post_subagent"]) > 0 {
		for _, hook := range stage.Hooks["post_subagent"] {
			if h.hookExecutor != nil {
				hookCtx := HookContext{
					RunID:          input.RunID,
					StageID:        stage.ID,
					StageType:      stage.Type,
					Task:           task,
					Worktree:       wt,
					SubagentResult: subagentResult,
				}
				
				result, err := h.hookExecutor.Call(hook, hookCtx)
				if err != nil {
					return fmt.Errorf("post_subagent hook failed for task %s: %w", task.ID, err)
				}
				
				if !result.Continue {
					return fmt.Errorf("post_subagent hook blocked task %s: %s", task.ID, result.Reason)
				}
			}
		}
	}
	
	// Call pre_merge hook if configured
	if stage.Hooks != nil && len(stage.Hooks["pre_merge"]) > 0 {
		for _, hook := range stage.Hooks["pre_merge"] {
			if h.hookExecutor != nil {
				hookCtx := HookContext{
					RunID:          input.RunID,
					StageID:        stage.ID,
					StageType:      stage.Type,
					Task:           task,
					Worktree:       wt,
					SubagentResult: subagentResult,
				}
				
				result, err := h.hookExecutor.Call(hook, hookCtx)
				if err != nil {
					return fmt.Errorf("pre_merge hook failed for task %s: %w", task.ID, err)
				}
				
				if !result.Continue {
					return fmt.Errorf("pre_merge hook blocked merge for task %s: %s", task.ID, result.Reason)
				}
				
				if !result.MergeApproved {
					return fmt.Errorf("merge blocked by hook for task %s: %s", task.ID, result.Reason)
				}
			}
		}
	}
	
	// Merge worktree
	mergeResult, err := h.worktreeManager.Merge(wt.ID, strategy)
	if err != nil {
		return fmt.Errorf("merge failed for task %s: %w", task.ID, err)
	}
	
	if !mergeResult.Success {
		return fmt.Errorf("merge failed for task %s: %v", task.ID, mergeResult.Conflicts)
	}
	
	// Mark task complete
	resultMu.Lock()
	completed[task.ID] = true
	results[task.ID] = &protocol.TaskResult{
		TaskID:       task.ID,
		Success:      subagentResult.Success,
		Output:       subagentResult.Output,
		FilesChanged: subagentResult.FilesChanged,
		TurnsUsed:    subagentResult.TurnsUsed,
		TokensUsed:   subagentResult.TokensUsed,
		DurationMs:   subagentResult.Duration.Milliseconds(),
		Metadata:     subagentResult.Metadata,
	}
	resultMu.Unlock()
	
	return nil
}

// GetWorktreeManager returns the worktree manager
func (h *TaskPromptHandler) GetWorktreeManager() WorktreeManager {
	return h.worktreeManager
}

// GetSubagentRunner returns the subagent runner
func (h *TaskPromptHandler) GetSubagentRunner() SubagentRunner {
	return h.subagentRunner
}

// SetWorktreeManager sets the worktree manager (for testing)
func (h *TaskPromptHandler) SetWorktreeManager(mgr WorktreeManager) {
	h.worktreeManager = mgr
}

// SetSubagentRunner sets the subagent runner (for testing)
func (h *TaskPromptHandler) SetSubagentRunner(runner SubagentRunner) {
	h.subagentRunner = runner
}
