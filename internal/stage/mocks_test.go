package stage

import (
	"context"
	"time"

	"github.com/mhingston/codefoundry/internal/protocol"
	"github.com/mhingston/codefoundry/internal/subagent"
	"github.com/mhingston/codefoundry/internal/worktree"
)

// MockWorktreeManager implements WorktreeManager for testing
type MockWorktreeManager struct {
	CreateFunc  func(taskID string, config worktree.WorktreeConfig) (*worktree.Worktree, error)
	MergeFunc   func(worktreeID string, strategy worktree.MergeStrategy) (*worktree.MergeResult, error)
	DeleteFunc  func(worktreeID string) error
	GetDiffFunc func(worktreeID string) (string, error)
	CreateCalls []string
	MergeCalls  []string
	DeleteCalls []string
}

func (m *MockWorktreeManager) Create(taskID string, config worktree.WorktreeConfig) (*worktree.Worktree, error) {
	m.CreateCalls = append(m.CreateCalls, taskID)
	if m.CreateFunc != nil {
		return m.CreateFunc(taskID, config)
	}
	return &worktree.Worktree{
		ID:         "wt-" + taskID,
		TaskID:     taskID,
		Path:       "/tmp/worktrees/" + taskID,
		Branch:     "cf-" + taskID,
		BaseCommit: "abc123",
		CreatedAt:  time.Now().UTC(),
	}, nil
}

func (m *MockWorktreeManager) Merge(worktreeID string, strategy worktree.MergeStrategy) (*worktree.MergeResult, error) {
	m.MergeCalls = append(m.MergeCalls, worktreeID)
	if m.MergeFunc != nil {
		return m.MergeFunc(worktreeID, strategy)
	}
	return &worktree.MergeResult{
		Success:   true,
		Strategy:  strategy,
		Conflicts: []string{},
	}, nil
}

func (m *MockWorktreeManager) Delete(worktreeID string) error {
	m.DeleteCalls = append(m.DeleteCalls, worktreeID)
	if m.DeleteFunc != nil {
		return m.DeleteFunc(worktreeID)
	}
	return nil
}

func (m *MockWorktreeManager) GetDiff(worktreeID string) (string, error) {
	if m.GetDiffFunc != nil {
		return m.GetDiffFunc(worktreeID)
	}
	return "diff --git a/file.go b/file.go\n...", nil
}

// MockSubagentRunner implements SubagentRunner for testing
type MockSubagentRunner struct {
	SpawnFunc  func(req subagent.SpawnRequest) (*subagent.Subagent, error)
	WaitFunc   func(ctx context.Context, subagentID string) (*subagent.Result, error)
	SpawnCalls []subagent.SpawnRequest
	WaitCalls  []string
}

func (m *MockSubagentRunner) Spawn(req subagent.SpawnRequest) (*subagent.Subagent, error) {
	m.SpawnCalls = append(m.SpawnCalls, req)
	if m.SpawnFunc != nil {
		return m.SpawnFunc(req)
	}
	return &subagent.Subagent{
		ID:       "sub-" + req.TaskID,
		TaskID:   req.TaskID,
		Worktree: req.WorktreePath,
		Status:   subagent.StatusRunning,
	}, nil
}

func (m *MockSubagentRunner) Wait(ctx context.Context, subagentID string) (*subagent.Result, error) {
	m.WaitCalls = append(m.WaitCalls, subagentID)
	if m.WaitFunc != nil {
		return m.WaitFunc(ctx, subagentID)
	}
	return &subagent.Result{
		Success:      true,
		Output:       "Task completed successfully",
		FilesChanged: []string{"file.go"},
		TurnsUsed:    10,
		TokensUsed:   5000,
		Duration:     time.Second,
		Metadata: map[string]interface{}{
			"task_id": subagentID,
		},
	}, nil
}

// MockHookExecutor implements HookExecutor for testing
type MockHookExecutor struct {
	CallFunc    func(hook protocol.Hook, ctx HookContext) (*HookResult, error)
	CallCount   int
	CallHistory []HookCall
}

type HookCall struct {
	Hook protocol.Hook
	Ctx  HookContext
}

func (m *MockHookExecutor) Call(hook protocol.Hook, ctx HookContext) (*HookResult, error) {
	m.CallCount++
	m.CallHistory = append(m.CallHistory, HookCall{Hook: hook, Ctx: ctx})
	if m.CallFunc != nil {
		return m.CallFunc(hook, ctx)
	}
	return &HookResult{
		Status:        "ok",
		Continue:      true,
		MergeApproved: true,
	}, nil
}

// Reset clears all call tracking
func (m *MockWorktreeManager) Reset() {
	m.CreateCalls = nil
	m.MergeCalls = nil
	m.DeleteCalls = nil
}

func (m *MockSubagentRunner) Reset() {
	m.SpawnCalls = nil
	m.WaitCalls = nil
}

func (m *MockHookExecutor) Reset() {
	m.CallCount = 0
	m.CallHistory = nil
}
