package stage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	RunID          string
	StageID        string
	StageType      string
	Task           *protocol.Task
	Worktree       *worktree.Worktree
	SubagentResult *subagent.Result
	Limits         subagent.Limits
}

// HookResult contains the result of hook execution
type HookResult struct {
	Status        string
	Continue      bool
	Reason        string
	MergeApproved bool
	Overrides     struct {
		Limits          subagent.Limits
		TemplateVars    map[string]string
		GateRelaxations map[string]bool
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

	policyByTask := map[string]interface{}{}
	taskIDs := make([]string, 0, len(results))
	for taskID := range results {
		taskIDs = append(taskIDs, taskID)
	}
	sort.Strings(taskIDs)
	for _, taskID := range taskIDs {
		if decisions, ok := results[taskID].Metadata["exploration_policy_decisions"]; ok {
			policyByTask[taskID] = decisions
		}
	}

	return &StageResult{
		Status:  string(StatusPass),
		Summary: summary,
		Outputs: stage.Outputs,
		Metadata: map[string]interface{}{
			"tasks_completed":              len(results),
			"waves":                        len(waves),
			"exploration_policy_snapshot":  input.ExplorationPolicy,
			"exploration_policy_decisions": policyByTask,
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

type policyDecision struct {
	StageAllowed        bool                   `json:"stage_allowed"`
	Reason              string                 `json:"reason"`
	ExplorationEnabled  bool                   `json:"exploration_enabled"`
	AllowedParameters   []string               `json:"allowed_parameters,omitempty"`
	AppliedParameters   []string               `json:"applied_parameters,omitempty"`
	DeniedParameters    []string               `json:"denied_parameters,omitempty"`
	GateRelaxationBlock []string               `json:"gate_relaxation_block,omitempty"`
	Ambiguous           bool                   `json:"ambiguous"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
}

func evaluatePolicy(policy *protocol.ExplorationPolicy, stageID string) policyDecision {
	decision := policyDecision{
		StageAllowed:       false,
		ExplorationEnabled: false,
		Reason:             "no exploration policy configured; fail-closed",
	}
	if policy == nil {
		return decision
	}

	allowed := false
	for _, id := range policy.AllowedStages {
		if id == stageID {
			allowed = true
			break
		}
	}

	if !allowed {
		decision.Reason = "stage not in exploration_policy.allowed_stages; fail-closed"
		return decision
	}

	if policy.MaxVariantAttempts <= 0 {
		decision.Reason = "max_variant_attempts <= 0; exploration disabled"
		return decision
	}

	decision.StageAllowed = true
	decision.ExplorationEnabled = true
	decision.Reason = "exploration enabled by policy"
	decision.AllowedParameters = append(decision.AllowedParameters, policy.AllowedParameters...)
	return decision
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func hasOverrides(result HookResult) bool {
	return result.Overrides.Limits != (subagent.Limits{}) ||
		len(result.Overrides.TemplateVars) > 0 ||
		len(result.Overrides.GateRelaxations) > 0
}

func applyExplorationOverrides(baseLimits subagent.Limits, baseTemplateVars map[string]string, override HookResult, policy *protocol.ExplorationPolicy, stageID string, requiredGateByID, allGateByID map[string]bool) (subagent.Limits, map[string]string, policyDecision, error) {
	limits := baseLimits
	templateVars := map[string]string{}
	for k, v := range baseTemplateVars {
		templateVars[k] = v
	}

	decision := evaluatePolicy(policy, stageID)
	if decision.Metadata == nil {
		decision.Metadata = map[string]interface{}{}
	}
	decision.Metadata["max_variant_attempts"] = 0
	if policy != nil {
		decision.Metadata["max_variant_attempts"] = policy.MaxVariantAttempts
	}

	if len(override.Overrides.GateRelaxations) > 0 {
		if policy == nil {
			decision.Ambiguous = true
			decision.Reason = "gate relaxations requested without policy; fail-closed"
			return limits, templateVars, decision, fmt.Errorf(decision.Reason)
		}
		for gateID, relax := range override.Overrides.GateRelaxations {
			if !relax {
				continue
			}
			if !allGateByID[gateID] {
				decision.Ambiguous = true
				decision.Reason = "unknown gate in relaxation request; fail-closed"
				return limits, templateVars, decision, fmt.Errorf("unknown gate in relaxation request: %s", gateID)
			}
			if requiredGateByID[gateID] || containsString(policy.ForbiddenGateRelaxation, gateID) {
				decision.GateRelaxationBlock = append(decision.GateRelaxationBlock, gateID)
				decision.Reason = "required/forbidden gate relaxation blocked"
				return limits, templateVars, decision, fmt.Errorf("gate relaxation blocked for %s", gateID)
			}
		}
	}

	if !decision.ExplorationEnabled {
		if len(override.Overrides.TemplateVars) > 0 || override.Overrides.Limits != (subagent.Limits{}) {
			decision.DeniedParameters = append(decision.DeniedParameters, "overrides")
		}
		return limits, templateVars, decision, nil
	}

	allowed := map[string]bool{}
	for _, key := range policy.AllowedParameters {
		allowed[key] = true
	}

	if override.Overrides.Limits.MaxTurns > 0 {
		param := "limits.max_turns"
		if !allowed[param] {
			decision.DeniedParameters = append(decision.DeniedParameters, param)
		} else {
			limits.MaxTurns = override.Overrides.Limits.MaxTurns
			decision.AppliedParameters = append(decision.AppliedParameters, param)
		}
	}
	if override.Overrides.Limits.MaxTokens > 0 {
		param := "limits.max_tokens"
		if !allowed[param] {
			decision.DeniedParameters = append(decision.DeniedParameters, param)
		} else {
			limits.MaxTokens = override.Overrides.Limits.MaxTokens
			decision.AppliedParameters = append(decision.AppliedParameters, param)
		}
	}
	if override.Overrides.Limits.Timeout > 0 {
		param := "limits.timeout"
		if !allowed[param] {
			decision.DeniedParameters = append(decision.DeniedParameters, param)
		} else {
			limits.Timeout = override.Overrides.Limits.Timeout
			decision.AppliedParameters = append(decision.AppliedParameters, param)
		}
	}
	if override.Overrides.Limits.MemoryMB > 0 {
		param := "limits.memory_mb"
		if !allowed[param] {
			decision.DeniedParameters = append(decision.DeniedParameters, param)
		} else {
			limits.MemoryMB = override.Overrides.Limits.MemoryMB
			decision.AppliedParameters = append(decision.AppliedParameters, param)
		}
	}

	for key, value := range override.Overrides.TemplateVars {
		param := "template_vars." + key
		if !allowed[param] {
			decision.DeniedParameters = append(decision.DeniedParameters, param)
			continue
		}
		templateVars[key] = value
		decision.AppliedParameters = append(decision.AppliedParameters, param)
	}

	return limits, templateVars, decision, nil
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

	// Initialize subagent request context
	limits := subagent.DefaultLimits()
	templateVars := map[string]string{}
	for k, v := range task.TemplateVars {
		templateVars[k] = v
	}
	policyDecisions := []policyDecision{evaluatePolicy(input.ExplorationPolicy, stage.ID)}
	variantAttempts := 0

	// Call pre_subagent hook if configured
	if stage.Hooks != nil && len(stage.Hooks["pre_subagent"]) > 0 {
		for _, hook := range stage.Hooks["pre_subagent"] {
			if h.hookExecutor != nil {
				hookCtx := HookContext{
					RunID:     input.RunID,
					StageID:   stage.ID,
					StageType: stage.Type,
					Task:      task,
					Worktree:  wt,
					Limits:    limits,
				}

				result, err := h.hookExecutor.Call(hook, hookCtx)
				if err != nil {
					return fmt.Errorf("pre_subagent hook failed for task %s: %w", task.ID, err)
				}

				if !result.Continue {
					return fmt.Errorf("pre_subagent hook blocked task %s: %s", task.ID, result.Reason)
				}

				if hasOverrides(*result) {
					variantAttempts++
					if input.ExplorationPolicy == nil || variantAttempts > input.ExplorationPolicy.MaxVariantAttempts {
						return fmt.Errorf("pre_subagent exploration policy blocked task %s: max variant attempts exceeded or policy missing", task.ID)
					}
				}

				updatedLimits, updatedTemplateVars, decision, applyErr := applyExplorationOverrides(limits, templateVars, *result, input.ExplorationPolicy, stage.ID, input.RequiredGateByID, input.AllGateByID)
				policyDecisions = append(policyDecisions, decision)
				if applyErr != nil {
					return fmt.Errorf("pre_subagent exploration policy blocked task %s: %w", task.ID, applyErr)
				}

				limits = updatedLimits
				templateVars = updatedTemplateVars
			}
		}
	}

	// Spawn subagent
	req := subagent.SpawnRequest{
		TaskID:       task.ID,
		WorktreePath: wt.Path,
		Limits:       limits,
		Prompt:       task.Description,
		TemplateVars: templateVars,
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
		Metadata: func() map[string]interface{} {
			metadata := map[string]interface{}{}
			for k, v := range subagentResult.Metadata {
				metadata[k] = v
			}
			metadata["exploration_policy_decisions"] = policyDecisions
			metadata["effective_limits"] = map[string]interface{}{
				"max_turns":  limits.MaxTurns,
				"max_tokens": limits.MaxTokens,
				"timeout":    limits.Timeout.String(),
				"memory_mb":  limits.MemoryMB,
			}
			metadata["effective_template_vars"] = templateVars
			return metadata
		}(),
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
