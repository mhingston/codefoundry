# CodeFoundry - Implementation Start Guide

## Quick Start

```bash
# Navigate to project
cd /root/codefoundry

# Download dependencies
go mod download

# Build
go build ./cmd/codefoundry

# Run
./codefoundry --help
```

## What's Been Created

### Core Documents
- **`codefoundry-handoff.md`** - Complete handoff specification (294 lines)
- **`README.md`** - Project overview
- **`IMPLEMENTATION_START.md`** - This file

### Schemas (JSON)
- `schemas/protocol.schema.json` - Protocol YAML validation
- `schemas/status.schema.json` - Stage status contract
- `schemas/gate-report.schema.json` - Gate execution results
- `schemas/lock-decision.schema.json` - Lock decision contract
- `schemas/state.schema.json` - Runtime state

### Templates (Markdown)
- `templates/plan.md` - Plan stage template
- `templates/spec.md` - Spec stage template
- `templates/decompose.md` - Task decomposition template
- `templates/review.md` - Code review template
- `templates/compound.md` - Learning capture template

### Documentation
- `docs/architecture.md` - System architecture
- `docs/contracts.md` - Contract specifications
- `docs/integration.md` - Harness integration guide

### Examples
- `examples/simple/protocol.yaml` - Basic workflow

### Go Code
- `cmd/codefoundry/main.go` - CLI entry point (stubs)
- `go.mod` - Go module definition

### Build
- `Makefile` - Build commands
- `.gitignore` - Git ignore rules

## Implementation Phases

### Phase 1: MVP Foundation (Weeks 1-2)
**Goal:** Core protocol engine

**Priority Files:**
1. `internal/protocol/loader.go` - YAML parsing
2. `internal/protocol/validator.go` - Schema validation
3. `internal/stage/runner.go` - Stage execution
4. `internal/artifact/store.go` - Artifact storage
5. `internal/stage/state.go` - State persistence

**Tests:**
- `internal/protocol/*_test.go`
- `internal/stage/*_test.go`

### Phase 2: Agentic Execution (Weeks 3-4)
**Goal:** Worktrees and subagents

**Priority Files:**
1. `internal/worktree/manager.go` - Git worktrees
2. `internal/subagent/runner.go` - Subagent execution
3. `internal/harness/adapter.go` - Harness API

### Phase 3: Review and Governance (Weeks 5-6)
**Goal:** Gates and lock decisions

**Priority Files:**
1. `internal/gate/executor.go` - Gate execution
2. `internal/lock/evaluator.go` - Lock decisions
3. `internal/templates/engine.go` - Template rendering

### Phase 4: Autonomy Hardening (Weeks 7-8)
**Goal:** Metrics and replay

**Priority Files:**
1. `internal/report/generator.go` - Report generation
2. Replay functionality
3. CI/CD integration

## Reference Projects

### Factorial (TypeScript)
Location: `/root/factorial/`

**Key patterns to adapt:**
- `packages/core/src/handlers/builtin.ts` - Gate execution
- `packages/core/src/worktree/manager.ts` - Worktree patterns
- `packages/core/src/subagents/` - Subagent patterns
- `packages/core/src/satisfaction/judge.ts` - Review scoring
- `docs/self-hosting-maturity-ladder.md` - Maturity levels

### SpecFirst (Go)
Location: `/root/SpecFirst/`

**Key patterns to adapt:**
- `docs/PROTOCOLS.md` - Protocol schema
- `cmd/` - CLI structure
- `.specfirst/artifacts/` - Artifact storage pattern

### Software Factory Handoff
Location: `/root/software-factory-handoff.md`

**Source of truth for requirements**

## Key Design Decisions

1. **Language:** Go - Deterministic, fast CLI, strong typing
2. **Architecture:** External harness integration via tools/events
3. **Philosophy:** Protocol-first, fail-closed, deterministic
4. **Scope:** All 4 phases (complete implementation)

## Testing Strategy

- Unit tests: `*_test.go` files alongside implementation
- Integration tests: `tests/integration/` directory
- Contract tests: Schema validation
- Determinism tests: Replay verification

Target: 90%+ coverage

## Next Steps

1. **Read the handoff doc:** `codefoundry-handoff.md`
2. **Review reference projects:** Factorial and SpecFirst
3. **Start with schemas:** Ensure all contracts validate
4. **Build protocol loader:** Foundation for everything else
5. **Write tests first:** Contract-driven development

## Questions?

See `codefoundry-handoff.md` section "Questions?" for guidance on handling ambiguities.

**Remember:**
- Prefer explicit over implicit
- Fail closed when uncertain
- Follow SpecFirst's artifact-centric design
- Follow Factorial's maturity patterns
- Keep contracts machine-checkable

## File Structure

```
codefoundry/
├── cmd/codefoundry/         # CLI entry point
├── internal/                 # Private packages
│   ├── protocol/            # YAML loading, validation
│   ├── stage/               # Stage runner, state
│   ├── gate/                # Gate execution
│   ├── artifact/            # Artifact storage
│   ├── lock/                # Lock decisions
│   ├── worktree/            # Git worktrees
│   ├── subagent/            # Subagent runner
│   ├── harness/             # Harness API
│   ├── templates/           # Template engine
│   └── report/              # Report generation
├── pkg/api/                  # Public API
├── schemas/                  # JSON schemas
├── templates/                # Stage templates
├── docs/                     # Documentation
├── examples/                 # Example protocols
└── README.md                 # Project overview
```

## Development Commands

```bash
# Build
make build

# Test
make test

# Test with coverage
make test-coverage

# Lint
make lint

# Format
make fmt

# Clean
make clean

# Run
make run

# Initialize dev environment
make dev-init
```

## Success Criteria

The implementation is complete when:

1. ✅ Full workflow executes: plan → spec → decompose → implement → verify → review → lock
2. ✅ Every stage emits valid `status.json`
3. ✅ Lock decision is fail-closed
4. ✅ Parallel execution in isolated worktrees
5. ✅ Single command emits evidence bundle
6. ✅ Runs are replayable with identical results
7. ✅ External harnesses can integrate via tools/events
8. ✅ Test coverage >90%

---

**Ready to start implementing!**

Start with Phase 1: Protocol loader and validator.
