package subagent

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// Status represents the current state of a subagent
type Status string

const (
	StatusPending    Status = "pending"
	StatusRunning    Status = "running"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusAborted    Status = "aborted"
	StatusTimeout    Status = "timeout"
)

// Subagent represents a running subagent
type Subagent struct {
	ID         string
	TaskID     string
	Worktree   string
	Status     Status
	Limits     Limits
	Usage      Usage
	Result     *Result
	Error      error
	StartedAt  *time.Time
	CompletedAt *time.Time
	mu         sync.RWMutex
}

// SubagentStatus provides a snapshot of subagent status
type SubagentStatus struct {
	ID          string
	TaskID      string
	Status      Status
	Usage       Usage
	StartedAt   *time.Time
	CompletedAt *time.Time
	Duration    time.Duration
}

// Result contains the result of subagent execution
type Result struct {
	Success      bool
	Output       string
	FilesChanged []string
	TurnsUsed    int
	TokensUsed   int
	Duration     time.Duration
	Metadata     map[string]interface{}
}

// SpawnRequest contains parameters for spawning a subagent
type SpawnRequest struct {
	TaskID      string
	WorktreePath string
	Limits      Limits
	Prompt      string
	TemplateVars map[string]string
	Environment map[string]string
}

// Runner manages subagent execution
type Runner struct {
	subagents   map[string]*Subagent
	emitter     *Emitter
	mu          sync.RWMutex
	basePath    string
}

// NewRunner creates a new subagent runner
func NewRunner(basePath string) *Runner {
	runner := &Runner{
		subagents: make(map[string]*Subagent),
		emitter:   NewEmitter(),
		basePath:  basePath,
	}
	
	// Register console emitter by default
	runner.emitter.RegisterHandler(ConsoleEmitter())
	
	return runner
}

// WithEmitter sets a custom event emitter
func (r *Runner) WithEmitter(emitter *Emitter) *Runner {
	r.emitter = emitter
	return r
}

// Spawn creates a new subagent in a worktree
func (r *Runner) Spawn(req SpawnRequest) (*Subagent, error) {
	// Validate limits
	if err := req.Limits.Validate(); err != nil {
		return nil, fmt.Errorf("invalid limits: %w", err)
	}

	subagentID := fmt.Sprintf("subagent-%s-%d", req.TaskID, time.Now().UnixNano())
	
	subagent := &Subagent{
		ID:       subagentID,
		TaskID:   req.TaskID,
		Worktree: req.WorktreePath,
		Status:   StatusPending,
		Limits:   req.Limits,
		Usage:    Usage{},
	}

	r.mu.Lock()
	r.subagents[subagentID] = subagent
	r.mu.Unlock()

	// Emit spawned event
	ctx := context.Background()
	r.emitter.Emit(ctx, NewEvent(EventSpawned, subagentID, req.TaskID, map[string]interface{}{
		"worktree": req.WorktreePath,
		"limits":   req.Limits,
	}))

	return subagent, nil
}

// Wait blocks until a subagent completes or times out
func (r *Runner) Wait(ctx context.Context, subagentID string) (*Result, error) {
	subagent, err := r.getSubagent(subagentID)
	if err != nil {
		return nil, err
	}

	// Start the subagent if not already started
	if subagent.Status == StatusPending {
		r.startSubagent(ctx, subagent)
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, subagent.Limits.Timeout)
	defer cancel()

	// Wait for completion
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeoutCtx.Done():
			// Timeout exceeded
			r.abortSubagent(subagentID, "timeout")
			r.emitter.Emit(ctx, NewEvent(EventTimeout, subagentID, subagent.TaskID, map[string]interface{}{
				"timeout": subagent.Limits.Timeout,
			}))
			return &Result{
				Success: false,
				Output:  "Subagent timed out",
			}, fmt.Errorf("subagent %s timed out after %v", subagentID, subagent.Limits.Timeout)
			
		case <-ticker.C:
			subagent.mu.RLock()
			status := subagent.Status
			result := subagent.Result
			subagent.mu.RUnlock()
			
			if status == StatusCompleted || status == StatusFailed || status == StatusAborted {
				if result != nil {
					return result, nil
				}
				return nil, subagent.Error
			}
			
			// Check resource limits
			enforcer := NewEnforcer(subagent.Limits)
			enforcer.usage = subagent.Usage
			exceeded, reason := enforcer.limits.IsExceeded(subagent.Usage)
			if exceeded {
				r.abortSubagent(subagentID, reason)
				r.emitter.Emit(ctx, NewEvent(EventLimitExceeded, subagentID, subagent.TaskID, map[string]interface{}{
					"reason": reason,
					"usage":  subagent.Usage,
				}))
				return &Result{
					Success: false,
					Output:  fmt.Sprintf("Resource limit exceeded: %s", reason),
				}, fmt.Errorf("resource limit exceeded: %s", reason)
			}
		}
	}
}

// Abort terminates a running subagent
func (r *Runner) Abort(subagentID string) error {
	subagent, err := r.getSubagent(subagentID)
	if err != nil {
		return err
	}

	r.abortSubagent(subagentID, "manual abort")
	
	ctx := context.Background()
	r.emitter.Emit(ctx, NewEvent(EventAborted, subagentID, subagent.TaskID, map[string]interface{}{
		"reason": "manual abort",
	}))
	
	return nil
}

// Status returns the current status of a subagent
func (r *Runner) Status(subagentID string) (*SubagentStatus, error) {
	subagent, err := r.getSubagent(subagentID)
	if err != nil {
		return nil, err
	}

	subagent.mu.RLock()
	defer subagent.mu.RUnlock()

	status := &SubagentStatus{
		ID:       subagent.ID,
		TaskID:   subagent.TaskID,
		Status:   subagent.Status,
		Usage:    subagent.Usage,
		StartedAt: subagent.StartedAt,
		CompletedAt: subagent.CompletedAt,
	}

	if subagent.StartedAt != nil && subagent.CompletedAt != nil {
		status.Duration = subagent.CompletedAt.Sub(*subagent.StartedAt)
	} else if subagent.StartedAt != nil {
		status.Duration = time.Since(*subagent.StartedAt)
	}

	return status, nil
}

// List returns all subagents
func (r *Runner) List() []*SubagentStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*SubagentStatus, 0, len(r.subagents))
	for _, subagent := range r.subagents {
		status, _ := r.Status(subagent.ID)
		if status != nil {
			result = append(result, status)
		}
	}

	return result
}

// GetResult returns the result for a completed subagent
func (r *Runner) GetResult(subagentID string) (*Result, error) {
	subagent, err := r.getSubagent(subagentID)
	if err != nil {
		return nil, err
	}

	subagent.mu.RLock()
	defer subagent.mu.RUnlock()

	if subagent.Result == nil {
		return nil, fmt.Errorf("subagent %s has no result yet", subagentID)
	}

	return subagent.Result, nil
}

// Cleanup removes completed subagents from memory
func (r *Runner) Cleanup() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, subagent := range r.subagents {
		subagent.mu.RLock()
		status := subagent.Status
		subagent.mu.RUnlock()
		
		if status == StatusCompleted || status == StatusFailed || status == StatusAborted {
			delete(r.subagents, id)
		}
	}
}

// getSubagent retrieves a subagent by ID
func (r *Runner) getSubagent(subagentID string) (*Subagent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	subagent, ok := r.subagents[subagentID]
	if !ok {
		return nil, fmt.Errorf("subagent not found: %s", subagentID)
	}

	return subagent, nil
}

// startSubagent starts a subagent execution
func (r *Runner) startSubagent(ctx context.Context, subagent *Subagent) {
	subagent.mu.Lock()
	if subagent.Status != StatusPending {
		subagent.mu.Unlock()
		return
	}
	
	now := time.Now().UTC()
	subagent.Status = StatusRunning
	subagent.StartedAt = &now
	subagent.mu.Unlock()

	// Emit started event
	r.emitter.Emit(ctx, NewEvent(EventStarted, subagent.ID, subagent.TaskID, map[string]interface{}{
		"worktree": subagent.Worktree,
	}))

	// Start execution in a goroutine
	go r.executeSubagent(subagent)
}

// executeSubagent executes the subagent task
func (r *Runner) executeSubagent(subagent *Subagent) {
	// This is a placeholder for actual subagent execution
	// In Phase 3, this would call the LLM harness
	
	ctx := context.Background()
	startTime := time.Now()

	// Simulate execution
	// In real implementation, this would:
	// 1. Read task prompt
	// 2. Call LLM harness
	// 3. Track turns and tokens
	// 4. Monitor files changed
	
	// For now, simulate a simple execution
	result := &Result{
		Success:      true,
		Output:       "Simulated subagent execution",
		FilesChanged: []string{},
		TurnsUsed:    1,
		TokensUsed:   100,
		Duration:     time.Since(startTime),
		Metadata: map[string]interface{}{
			"simulated": true,
		},
	}

	// Update subagent
	subagent.mu.Lock()
	now := time.Now().UTC()
	subagent.Status = StatusCompleted
	subagent.Result = result
	subagent.CompletedAt = &now
	subagent.Usage = Usage{
		TurnsUsed:  result.TurnsUsed,
		TokensUsed: result.TokensUsed,
		Duration:   result.Duration,
	}
	subagent.mu.Unlock()

	// Emit completed event
	r.emitter.Emit(ctx, NewEvent(EventCompleted, subagent.ID, subagent.TaskID, map[string]interface{}{
		"result": result,
	}))
}

// abortSubagent aborts a subagent
func (r *Runner) abortSubagent(subagentID string, reason string) {
	subagent, err := r.getSubagent(subagentID)
	if err != nil {
		return
	}

	subagent.mu.Lock()
	now := time.Now().UTC()
	subagent.Status = StatusAborted
	subagent.Error = fmt.Errorf("aborted: %s", reason)
	subagent.CompletedAt = &now
	subagent.mu.Unlock()
}

// GetFilesChanged returns the list of files changed in a worktree
func GetFilesChanged(worktreePath string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = worktreePath
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	if len(output) == 0 {
		return []string{}, nil
	}

	files := []string{}
	for _, line := range splitLines(string(output)) {
		if line = trimSpace(line); line != "" {
			files = append(files, line)
		}
	}

	return files, nil
}

func splitLines(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	
	return s[start:end]
}
