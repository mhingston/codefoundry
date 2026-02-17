# Architecture

## Overview

CodeFoundry is built around three principles:

1. **Determinism** - Same inputs produce same outputs
2. **Fail-Closed** - When uncertain, require human approval
3. **Composability** - External harnesses can integrate via tools/events

## System Components

### Control Plane

The control plane manages state, gates, and protocol execution.

#### Protocol Loader
- Parses YAML protocol definitions
- Validates against JSON schema
- Resolves dependencies (DAG validation)
- Caches loaded protocols

#### Stage Runner
- State machine for stage transitions
- Dependency checking
- Checkpoint/resume capability
- Parallel execution coordination

#### Gate Executor
- Runs command-based gates
- Captures stdout/stderr
- Structured output parsing
- Fail-closed enforcement

#### Artifact Store
- Content-addressable storage
- Stage-scoped namespaces
- Versioning and history
- Garbage collection

#### Lock Decision Evaluator
- Evaluates gate results
- Applies confidence thresholds
- P1/P2/P3 severity classification
- Fail-closed logic

### Execution Plane

The execution plane interfaces with external systems.

#### Harness Adapter
- HTTP/gRPC API for tools
- Event streaming
- Status polling endpoints

#### Subagent Runner
- Resource limit enforcement
- Token budget tracking
- Turn limits
- Abort signal handling

#### Worktree Manager
- Git worktree operations
- Isolation guarantees
- Merge strategies
- Cleanup handling

## Data Flow

```
User Request
    │
    ▼
┌─────────────┐
│   Plan      │
└──────┬──────┘
       │ plan.md
       ▼
┌─────────────┐
│    Spec     │
└──────┬──────┘
       │ spec.md
       ▼
┌─────────────┐
│  Decompose  │
└──────┬──────┘
       │ tasks.yaml
       ▼
┌─────────────┐
│ Implement   │────┬── Parallel subagents
└──────┬──────┘    │   in isolated worktrees
       │           │
       │ code      │
       ▼           │
┌─────────────┐    │
│   Verify    │◀───┘
└──────┬──────┘
       │ gate reports
       ▼
┌─────────────┐
│   Review    │
└──────┬──────┘
       │ rubric score
       ▼
┌─────────────┐
│    Lock     │
└──────┬──────┘
       │ decision
       ▼
┌─────────────┐
│  Compound   │
└─────────────┘
```

## State Management

### State Persistence

State is persisted to `.codefoundry/state/state.json`:

```json
{
  "run_id": "run-20260216-abc123",
  "stages": {
    "plan": { "status": "pass", ... },
    "spec": { "status": "running", ... }
  },
  "current_stage": "spec"
}
```

### Checkpoint/Resume

Stages can checkpoint their progress:

```go
type Checkpoint struct {
    StageID string                 `json:"stage_id"`
    Data    map[string]interface{} `json:"data"`
}
```

On resume, the stage receives its checkpoint data and continues.

## Artifact Storage

### Layout

```
.codefoundry/
└── artifacts/
    └── <run-id>/
        ├── plan/
        │   ├── status.json
        │   └── plan.md
        ├── spec/
        │   ├── status.json
        │   └── spec.md
        └── lock/
            ├── status.json
            └── lock-decision.json
```

### Content Addressing

Artifacts are stored by content hash for deduplication:

```
.codefoundry/artifacts/<run-id>/<stage-id>/<filename>-<hash>.<ext>
```

## Event System

### Event Types

- `stage_start` - Stage execution started
- `stage_complete` - Stage execution completed
- `gate_start` - Gate execution started
- `gate_complete` - Gate execution completed
- `lock_decision` - Lock decision made
- `checkpoint_saved` - Checkpoint persisted

### Event Schema

```json
{
  "type": "stage_complete",
  "timestamp": "2026-02-16T10:00:00Z",
  "run_id": "run-abc123",
  "stage_id": "verify",
  "payload": {
    "status": "pass",
    "duration_ms": 5000
  }
}
```

## Error Handling

### Stage Failures

Stages can fail in three ways:

1. **Execution Error** - Command failed (non-zero exit)
2. **Validation Error** - Output doesn't match expected format
3. **Timeout** - Stage exceeded time limit

All failures result in `status: "fail"` and require human intervention.

### Gate Failures

Gates fail closed:

- **Required gate fails** → Stage fails → Workflow pauses
- **Optional gate fails** → Warning logged → Workflow continues

### Lock Decision

The lock decision evaluator is fail-closed:

```go
func EvaluateLock(gates []GateResult, review ReviewResult) Decision {
    if !allRequiredGatesPass(gates) {
        return DecisionReopen
    }
    
    if review.Confidence < ConfidenceThreshold {
        return DecisionReopen
    }
    
    if review.P1Findings > 0 {
        return DecisionReopen
    }
    
    return DecisionResolved
}
```

## Security Considerations

### Command Execution

Gates execute shell commands. Security measures:

1. Commands are defined in protocol (not user input)
2. Environment variables are explicitly declared
3. Working directory is restricted
4. Timeout prevents infinite loops

### Worktree Isolation

Each subagent gets its own worktree:

1. Separate filesystem namespace
2. No access to parent repo changes
3. Merge requires explicit decision
4. Cleanup on completion

### Secret Handling

Secrets are never logged:

1. Environment variables filtered from logs
2. Sensitive patterns redacted from output
3. No secrets in state files

## Performance

### Caching

- Protocol definitions cached after first load
- Artifacts content-addressed for deduplication
- Gate results cached for replay

### Parallelism

- Independent stages run in parallel
- Subagents run in parallel
- Worktrees allow concurrent mutation

### Resource Limits

- Subagent token budgets
- Turn limits for LLM calls
- Memory limits for gate commands
- Disk quotas for artifacts
