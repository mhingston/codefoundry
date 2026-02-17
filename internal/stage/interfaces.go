package stage

import (
	"context"

	"github.com/mhingston/codefoundry/internal/subagent"
	"github.com/mhingston/codefoundry/internal/worktree"
)

// WorktreeManager defines the interface for worktree operations
type WorktreeManager interface {
	Create(taskID string, config worktree.WorktreeConfig) (*worktree.Worktree, error)
	Merge(worktreeID string, strategy worktree.MergeStrategy) (*worktree.MergeResult, error)
	Delete(worktreeID string) error
	GetDiff(worktreeID string) (string, error)
}

// SubagentRunner defines the interface for subagent operations
type SubagentRunner interface {
	Spawn(req subagent.SpawnRequest) (*subagent.Subagent, error)
	Wait(ctx context.Context, subagentID string) (*subagent.Result, error)
}
