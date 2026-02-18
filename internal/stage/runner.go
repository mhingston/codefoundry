package stage

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/gate"
	"github.com/mhingston/codefoundry/internal/protocol"
)

// Runner executes stages in dependency order
type Runner struct {
	protocolDef      *protocol.Protocol
	resolver         *protocol.Resolver
	stateManager     *StateManager
	checkpointMgr    *CheckpointManager
	artifactStore    *artifact.Store
	gateExecutor     *gate.Executor
	namespace        *artifact.Namespace
	handlers         map[string]StageHandler
	artifactBasePath string
	basePath         string
}

// StageHandler is a function that executes a stage
type StageHandler func(ctx context.Context, stage *protocol.Stage, input *StageInput) (*StageResult, error)

// StageInput contains inputs for stage execution
type StageInput struct {
	StageID           string
	RunID             string
	Inputs            []string
	Dependencies      []string
	Artifacts         *artifact.Store
	ExplorationPolicy *protocol.ExplorationPolicy
	RequiredGateByID  map[string]bool
}

// StageResult contains the result of stage execution
type StageResult struct {
	Status   string
	Summary  string
	Outputs  []string
	Evidence []string
	Error    error
	Metadata map[string]interface{}
}

// NewRunner creates a new stage runner
func NewRunner(p *protocol.Protocol, basePath string) *Runner {
	runner := &Runner{
		protocolDef:      p,
		resolver:         protocol.NewResolver(p),
		stateManager:     NewStateManager(basePath),
		basePath:         basePath,
		artifactBasePath: filepath.Join(basePath, "artifacts"),
		handlers:         make(map[string]StageHandler),
	}

	runner.checkpointMgr = NewCheckpointManager(runner.stateManager, basePath)

	return runner
}

// WithStateManager sets a custom state manager
func (r *Runner) WithStateManager(sm *StateManager) *Runner {
	r.stateManager = sm
	r.checkpointMgr = NewCheckpointManager(sm, r.basePath)
	return r
}

// WithArtifactStore sets an artifact store
func (r *Runner) WithArtifactStore(store *artifact.Store) *Runner {
	r.artifactStore = store
	if store != nil {
		r.namespace = store.Namespace()
	}
	return r
}

// WithGateExecutor sets a gate executor
func (r *Runner) WithGateExecutor(executor *gate.Executor) *Runner {
	r.gateExecutor = executor
	return r
}

// RegisterHandler registers a handler for a stage type
func (r *Runner) RegisterHandler(stageType string, handler StageHandler) {
	r.handlers[stageType] = handler
}

// Initialize sets up the runner for a new run
func (r *Runner) Initialize() error {
	// Initialize state
	if err := r.stateManager.Initialize(r.protocolDef.Version); err != nil {
		return fmt.Errorf("failed to initialize state: %w", err)
	}

	// Create namespace
	runID := r.stateManager.GetRunID()
	r.namespace = artifact.NewNamespace(r.basePath, runID)

	// Create artifact store
	r.artifactStore = artifact.NewStore(r.namespace)

	// Initialize all stages as pending
	for _, stage := range r.protocolDef.Stages {
		if err := r.stateManager.InitializeStage(stage.ID); err != nil {
			return fmt.Errorf("failed to initialize stage %s: %w", stage.ID, err)
		}
	}

	return nil
}

// Load loads existing state for resume
func (r *Runner) Load() error {
	if err := r.stateManager.Load(); err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Recreate namespace from state
	runID := r.stateManager.GetRunID()
	r.namespace = artifact.NewNamespace(r.basePath, runID)
	r.artifactStore = artifact.NewStore(r.namespace)

	return nil
}

// Run executes the full workflow
func (r *Runner) Run(ctx context.Context) error {
	// Get stages in dependency order
	stageOrder, err := r.resolver.TopologicalSort()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Execute each stage
	for _, stageID := range stageOrder {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		stage, err := r.protocolDef.GetStage(stageID)
		if err != nil {
			return fmt.Errorf("stage %s not found: %w", stageID, err)
		}

		if err := r.RunStage(ctx, stage); err != nil {
			return fmt.Errorf("stage %s failed: %w", stageID, err)
		}

		// Check if stage failed
		stageState, _ := r.stateManager.GetStageState(stageID)
		if stageState != nil && stageState.Status == string(StatusFail) {
			return fmt.Errorf("stage %s failed, stopping workflow", stageID)
		}
	}

	return nil
}

// RunStage executes a single stage
func (r *Runner) RunStage(ctx context.Context, stage *protocol.Stage) error {
	// Check if stage is already complete
	if r.stateManager.IsStageComplete(stage.ID) {
		return nil
	}

	// Check dependencies
	for _, depID := range stage.DependsOn {
		if !r.stateManager.IsStageComplete(depID) {
			return fmt.Errorf("stage %s has incomplete dependency: %s", stage.ID, depID)
		}
	}

	// Set up artifact path for stage
	artifactPath := r.namespace.StagePath(stage.ID)

	// Start stage
	if err := r.stateManager.StartStage(stage.ID, artifactPath); err != nil {
		return fmt.Errorf("failed to start stage: %w", err)
	}

	// Create status file
	status := &StageStatus{
		SchemaVersion: "codefoundry_stage_status.v1",
		StageID:       stage.ID,
		Status:        string(StatusRunning),
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
	}

	if err := r.artifactStore.WriteJSON(stage.ID, "status.json", status); err != nil {
		return fmt.Errorf("failed to write initial status: %w", err)
	}

	// Build stage input
	requiredGateByID := make(map[string]bool, len(r.protocolDef.Gates))
	for _, gateDef := range r.protocolDef.Gates {
		if gateDef.Required {
			requiredGateByID[gateDef.ID] = true
		}
	}

	input := &StageInput{
		StageID:           stage.ID,
		RunID:             r.stateManager.GetRunID(),
		Inputs:            stage.Inputs,
		Dependencies:      stage.DependsOn,
		Artifacts:         r.artifactStore,
		ExplorationPolicy: r.protocolDef.ExplorationPolicy,
		RequiredGateByID:  requiredGateByID,
	}

	// Execute based on stage type
	var result *StageResult
	var execErr error

	handler, hasHandler := r.handlers[stage.Type]
	if hasHandler {
		result, execErr = handler(ctx, stage, input)
	} else {
		// Default execution
		result, execErr = r.executeDefault(ctx, stage, input)
	}

	if execErr != nil {
		// Fail stage
		r.stateManager.FailStage(stage.ID, execErr)
		status.Status = string(StatusFail)
		status.Error = execErr.Error()
		status.CompletedAt = time.Now().UTC().Format(time.RFC3339)
		r.artifactStore.WriteJSON(stage.ID, "status.json", status)
		return execErr
	}

	// Complete stage
	status.Status = result.Status
	status.Summary = result.Summary
	status.Evidence = result.Evidence
	status.CompletedAt = time.Now().UTC().Format(time.RFC3339)

	if err := r.artifactStore.WriteJSON(stage.ID, "status.json", status); err != nil {
		return fmt.Errorf("failed to write final status: %w", err)
	}

	// Update state
	if result.Error != nil {
		r.stateManager.FailStage(stage.ID, result.Error)
	} else {
		statusVal := Status(result.Status)
		if statusVal == "" {
			statusVal = StatusPass
		}
		r.stateManager.CompleteStage(stage.ID, statusVal, result.Summary)
	}

	return nil
}

// executeDefault handles default stage execution
func (r *Runner) executeDefault(ctx context.Context, stage *protocol.Stage, input *StageInput) (*StageResult, error) {
	// Default execution just marks stage as completed
	// Actual implementation would depend on the stage type
	result := &StageResult{
		Status:  string(StatusPass),
		Summary: fmt.Sprintf("Stage %s completed", stage.ID),
		Outputs: stage.Outputs,
	}

	return result, nil
}

// RunSingleStage runs a single stage by ID
func (r *Runner) RunSingleStage(ctx context.Context, stageID string) error {
	stage, err := r.protocolDef.GetStage(stageID)
	if err != nil {
		return err
	}
	return r.RunStage(ctx, stage)
}

// RunFrom runs stages starting from a specific stage
func (r *Runner) RunFrom(ctx context.Context, startStageID string) error {
	stageOrder, err := r.resolver.TopologicalSort()
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// Find starting position
	startIdx := -1
	for i, stageID := range stageOrder {
		if stageID == startStageID {
			startIdx = i
			break
		}
	}

	if startIdx == -1 {
		return fmt.Errorf("start stage not found: %s", startStageID)
	}

	// Run from start position
	for i := startIdx; i < len(stageOrder); i++ {
		stageID := stageOrder[i]
		stage, err := r.protocolDef.GetStage(stageID)
		if err != nil {
			return err
		}

		if err := r.RunStage(ctx, stage); err != nil {
			return err
		}
	}

	return nil
}

// GetCurrentStage returns the currently running stage
func (r *Runner) GetCurrentStage() (string, error) {
	state := r.stateManager.GetState()
	if state == nil {
		return "", fmt.Errorf("state not initialized")
	}
	return state.CurrentStage, nil
}

// GetStageStatuses returns status of all stages
func (r *Runner) GetStageStatuses() (map[string]string, error) {
	state := r.stateManager.GetState()
	if state == nil {
		return nil, fmt.Errorf("state not initialized")
	}

	statuses := make(map[string]string)
	for id, stageState := range state.Stages {
		statuses[id] = stageState.Status
	}

	return statuses, nil
}

// IsComplete returns true if all stages are complete
func (r *Runner) IsComplete() bool {
	state := r.stateManager.GetState()
	if state == nil {
		return false
	}

	for _, stageState := range state.Stages {
		if stageState.Status == string(StatusPending) || stageState.Status == string(StatusRunning) {
			return false
		}
	}

	return true
}

// GetFailedStages returns IDs of all failed stages
func (r *Runner) GetFailedStages() []string {
	return r.stateManager.GetFailedStages()
}

// CompleteStage manually marks a stage as complete with artifacts
func (r *Runner) CompleteStage(stageID string, artifacts map[string][]byte) error {
	// Write artifacts
	for name, content := range artifacts {
		if err := r.artifactStore.Write(stageID, name, content); err != nil {
			return fmt.Errorf("failed to write artifact %s: %w", name, err)
		}
	}

	// Complete stage
	return r.stateManager.CompleteStage(stageID, StatusPass, "Manually completed")
}

// StageStatus represents a stage status
type StageStatus struct {
	SchemaVersion string                 `json:"schema_version"`
	StageID       string                 `json:"stage_id"`
	Status        string                 `json:"status"`
	Summary       string                 `json:"summary,omitempty"`
	Evidence      []string               `json:"evidence,omitempty"`
	StartedAt     string                 `json:"started_at,omitempty"`
	CompletedAt   string                 `json:"completed_at,omitempty"`
	Error         string                 `json:"error,omitempty"`
	DurationMs    int64                  `json:"duration_ms,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}
