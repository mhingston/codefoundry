# Harness Integration Guide

CodeFoundry integrates with external harnesses (opencode, codex, claude, copilot) via tools and events.

## Integration Philosophy

**CodeFoundry is the deterministic backbone.**
- Manages state, gates, and protocol execution
- Provides fail-closed guarantees
- Tracks artifacts and evidence

**Harness handles LLM interaction.**
- Generates prompts from templates
- Executes LLM calls
- Parses responses

## Architecture

```
┌─────────────┐         ┌─────────────┐         ┌─────────────┐
│   Harness   │◀───────▶│   CodeFoundry│◀───────▶│  Protocol   │
│   (opencode)│  Tools   │   (API)      │  State   │  Engine     │
└─────────────┘         └─────────────┘         └─────────────┘
       ▲                       │
       │                       │
       └───────────────────────┘
            Events (SSE)
```

## Tools API

Harnesses call CodeFoundry via HTTP API:

### Start Stage

```bash
POST /api/v1/runs/{run-id}/stages/{stage-id}/start
```

**Request:**
```json
{
  "checkpoint_id": "checkpoint-abc123"
}
```

**Response:**
```json
{
  "stage_id": "implement",
  "status": "running",
  "artifact_path": ".codefoundry/artifacts/run-abc/implement",
  "template": "implement.md",
  "started_at": "2026-02-16T10:00:00Z"
}
```

### Run Gate

```bash
POST /api/v1/runs/{run-id}/gates/{gate-id}/run
```

**Request:**
```json
{
  "working_directory": ".codefoundry/artifacts/run-abc/implement"
}
```

**Response:**
```json
{
  "schema_version": "codefoundry_gate_report.v1",
  "gate_id": "test",
  "status": "pass",
  "exit_code": 0,
  "duration_ms": 12345,
  "timestamp": "2026-02-16T10:00:00Z"
}
```

### Create Worktree

```bash
POST /api/v1/worktrees
```

**Request:**
```json
{
  "task_id": "task-001",
  "base_branch": "main",
  "strategy": "fail"  // fail | ours | theirs
}
```

**Response:**
```json
{
  "worktree_id": "wt-task-001",
  "path": "/path/to/repo/.worktrees/task-001",
  "branch": "cf-task-001",
  "status": "ready"
}
```

### Merge Worktree

```bash
POST /api/v1/worktrees/{worktree-id}/merge
```

**Request:**
```json
{
  "strategy": "fail"
}
```

**Response:**
```json
{
  "worktree_id": "wt-task-001",
  "status": "merged",
  "conflicts": [],
  "merge_commit": "abc123"
}
```

### Spawn Subagent

```bash
POST /api/v1/subagents
```

**Request:**
```json
{
  "task": "Implement user authentication",
  "template": "subagent.md",
  "worktree_id": "wt-task-001",
  "limits": {
    "max_turns": 50,
    "max_tokens": 100000
  }
}
```

**Response:**
```json
{
  "subagent_id": "sub-abc123",
  "status": "running",
  "worktree_id": "wt-task-001",
  "limits": {
    "max_turns": 50,
    "max_tokens": 100000
  }
}
```

### Complete Stage

```bash
POST /api/v1/runs/{run-id}/stages/{stage-id}/complete
```

**Request:**
```json
{
  "artifacts": [
    ".codefoundry/artifacts/run-abc/implement/code.go",
    ".codefoundry/artifacts/run-abc/implement/test.go"
  ],
  "metadata": {
    "files_changed": 2,
    "lines_added": 150,
    "lines_removed": 10
  }
}
```

**Response:**
```json
{
  "stage_id": "implement",
  "status": "pass",
  "artifact_path": ".codefoundry/artifacts/run-abc/implement",
  "completed_at": "2026-02-16T10:30:00Z",
  "duration_ms": 1800000
}
```

## Event Streaming

Harnesses subscribe to events via Server-Sent Events (SSE):

```bash
GET /api/v1/events?run={run-id}

Content-Type: text/event-stream
```

**Event Format:**
```
event: stage_start
data: {"run_id": "run-abc", "stage_id": "implement", "timestamp": "2026-02-16T10:00:00Z"}

event: gate_complete
data: {"run_id": "run-abc", "gate_id": "test", "status": "pass", "duration_ms": 12345}

event: lock_decision
data: {"run_id": "run-abc", "decision": "resolved", "confidence": 0.92}
```

### Event Types

| Event | Description |
|-------|-------------|
| `stage_start` | Stage execution started |
| `stage_complete` | Stage execution completed |
| `gate_start` | Gate execution started |
| `gate_complete` | Gate execution completed |
| `subagent_start` | Subagent spawned |
| `subagent_complete` | Subagent finished |
| `lock_decision` | Lock decision made |
| `worktree_created` | Worktree created |
| `worktree_merged` | Worktree merged |
| `checkpoint_saved` | Checkpoint persisted |

## CLI Integration

Harnesses can also use the CLI:

```bash
# Start a stage
codefoundry stage start <run-id> <stage-id>

# Run a gate
codefoundry gate run <run-id> <gate-id>

# Create worktree
codefoundry worktree create --task <task-id>

# Complete stage
codefoundry complete <stage-id> <artifact-path>
```

## Example: Opencode Integration

```typescript
// opencode tool definition
{
  name: "codefoundry_stage_start",
  description: "Start a CodeFoundry stage",
  parameters: {
    type: "object",
    properties: {
      run_id: { type: "string" },
      stage_id: { type: "string" },
      checkpoint_id: { type: "string" }
    },
    required: ["run_id", "stage_id"]
  },
  handler: async (args) => {
    const response = await fetch(
      `http://localhost:8080/api/v1/runs/${args.run_id}/stages/${args.stage_id}/start`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ checkpoint_id: args.checkpoint_id })
      }
    );
    return response.json();
  }
}
```

## Example: Harness-Side Workflow

```typescript
// Harness workflow using CodeFoundry
async function implementFeature(request: string) {
  // 1. Plan stage
  const planResult = await codefoundry.startStage(runId, "plan");
  const plan = await llm.generate(planResult.template, request);
  await codefoundry.completeStage(runId, "plan", [plan]);
  
  // 2. Spec stage
  const specResult = await codefoundry.startStage(runId, "spec");
  const spec = await llm.generate(specResult.template, plan);
  await codefoundry.completeStage(runId, "spec", [spec]);
  
  // 3. Decompose
  const decompResult = await codefoundry.startStage(runId, "decompose");
  const tasks = await llm.generate(decompResult.template, spec);
  await codefoundry.completeStage(runId, "decompose", [tasks]);
  
  // 4. Parallel implementation
  const tasks = yaml.parse(tasks);
  const implementations = await Promise.all(
    tasks.map(async (task) => {
      const worktree = await codefoundry.createWorktree(task.id);
      const subagent = await codefoundry.spawnSubagent({
        task: task.description,
        worktree_id: worktree.id
      });
      return { task, worktree, subagent };
    })
  );
  
  // Wait for all subagents
  for (const impl of implementations) {
    await codefoundry.waitForSubagent(impl.subagent.id);
    await codefoundry.mergeWorktree(impl.worktree.id);
  }
  
  await codefoundry.completeStage(runId, "implement", implementations);
  
  // 5. Verify
  const verifyResult = await codefoundry.startStage(runId, "verify");
  const gates = await Promise.all(
    verifyResult.gates.map(gate => 
      codefoundry.runGate(runId, gate.id)
    )
  );
  await codefoundry.completeStage(runId, "verify", gates);
  
  // 6. Review
  const reviewResult = await codefoundry.startStage(runId, "review");
  const review = await llm.generate(reviewResult.template, { diff, gates });
  await codefoundry.completeStage(runId, "review", [review]);
  
  // 7. Lock
  const lockResult = await codefoundry.startStage(runId, "lock");
  console.log(`Lock decision: ${lockResult.decision}`);
  
  return lockResult;
}
```

## Error Handling

### API Errors

```json
{
  "error": "stage_not_found",
  "message": "Stage 'unknown' not found in protocol",
  "code": 404
}
```

### Retry Logic

Harnesses should implement exponential backoff for transient errors:

```typescript
async function retryableCall(fn: () => Promise<any>, maxRetries = 3) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      return await fn();
    } catch (err) {
      if (i === maxRetries - 1) throw err;
      await sleep(Math.pow(2, i) * 1000);
    }
  }
}
```

## Security

### Authentication

API requests require authentication:

```bash
Authorization: Bearer <token>
```

### Scope Validation

Harnesses can only access runs they created:

```json
{
  "error": "unauthorized",
  "message": "Harness 'opencode-abc' cannot access run 'run-def'"
}
```

### Rate Limiting

API endpoints are rate-limited:

```json
{
  "error": "rate_limited",
  "message": "Too many requests",
  "retry_after": 60
}
```

## Testing Integration

### Mock Server

For testing harnesses without CodeFoundry:

```typescript
// Mock CodeFoundry server
import { setupServer } from "msw/node";

const server = setupServer(
  http.post("/api/v1/runs/*/stages/*/start", () => {
    return HttpResponse.json({
      stage_id: "implement",
      status: "running"
    });
  })
);

beforeAll(() => server.listen());
afterAll(() => server.close());
```

### Contract Testing

Test harness integration against contract schemas:

```bash
# Validate harness responses
codefoundry validate-harness --harness opencode --responses responses.json
```

## Best Practices

1. **Subscribe to events** - Don't poll, use SSE
2. **Handle failures** - CodeFoundry is fail-closed
3. **Respect timeouts** - Gate timeouts are enforced
4. **Clean up worktrees** - They're not garbage collected automatically
5. **Validate contracts** - Check responses against schemas
6. **Use checkpoinst** - Resume on interruption
7. **Log context** - Include run_id and stage_id in logs

## Troubleshooting

### Connection Refused

Check if CodeFoundry is running:

```bash
curl http://localhost:8080/api/v1/health
```

### Stage Not Found

Verify stage exists in protocol:

```bash
codefoundry validate --protocol .codefoundry/protocols/default.yaml
```

### Gate Timeout

Increase timeout in protocol:

```yaml
gates:
  - id: test
    command: "go test ./..."
    timeout: 600  # 10 minutes
```

### Worktree Conflicts

Check merge strategy:

```bash
codefoundry worktree merge <worktree-id> --strategy fail
```

## Harness SDK Comparison

CodeFoundry is designed to work with multiple AI coding harnesses. This section compares the integration approaches of major harnesses and explains why CodeFoundry uses **external hooks** rather than internal SDK integration.

### Harness SDKs Overview

#### OpenCode

**Extension Model:** In-process JavaScript/TypeScript plugins

**Key Features:**
- Plugins loaded from `.opencode/plugins/` directory
- Event-driven hooks (file.edited, tool.execute.before, session.created)
- Direct state modification
- Custom tools via exported functions
- Language: JavaScript/TypeScript only

**Architecture:**
```
OpenCode (Node.js process)
└── Plugins (JS/TS modules)
    ├── Custom tools
    ├── Event handlers
    └── State modifications
```

**Integration with CodeFoundry:**
OpenCode runs as a **harness** that implements CodeFoundry hooks via an HTTP server plugin:

```typescript
// .opencode/plugins/codefoundry-adapter.ts
export const CodeFoundryAdapter = async ({ client }) => {
  // Start HTTP server for CodeFoundry hooks
  const server = Bun.serve({
    port: 8081,
    routes: {
      "/hooks/pre-subagent": async (req) => {
        const { task } = await req.json();
        
        // Use OpenCode client to spawn subagent
        const subagent = await client.subagent.spawn({
          task: task.description,
          worktree: task.worktree_path
        });
        
        return Response.json({
          status: "ok",
          continue: true,
          subagent_id: subagent.id
        });
      }
    }
  });

  return {
    "session.created": async () => {
      console.log("CodeFoundry integration active on :8081");
    }
  };
};
```

#### GitHub Copilot SDK

**Extension Model:** Multi-language SDK communicating via JSON-RPC

**Key Features:**
- SDKs available: TypeScript, Python, Go, .NET
- Communicates with Copilot CLI via JSON-RPC
- Custom agents, skills, and tools
- BYOK (Bring Your Own Key) support
- Supports external CLI server mode

**Architecture:**
```
Your Application
    ↓
SDK Client (TS/Python/Go/.NET)
    ↓ JSON-RPC
Copilot CLI (server mode)
```

**Integration with CodeFoundry:**
Use Copilot SDK within a CodeFoundry hook implementation:

```typescript
// HTTP server implementing CodeFoundry hooks
// Uses Copilot SDK for LLM execution

import { CopilotClient } from "@github/copilot-sdk";

const copilot = new CopilotClient();

// CodeFoundry calls this hook
app.post("/hooks/pre-subagent", async (req, res) => {
  const { task, worktree_path } = req.body;
  
  // Use Copilot SDK to execute task
  const result = await copilot.execute({
    task: task.description,
    worktree: worktree_path,
    tools: ["read_file", "edit_file", "bash"]
  });
  
  res.json({
    status: result.success ? "ok" : "failed",
    continue: result.success,
    subagent_result: {
      output: result.output,
      files_modified: result.modifiedFiles,
      turns_used: result.turns
    }
  });
});
```

#### OpenAI Codex

**Extension Model:** CLI with TypeScript SDK

**Key Features:**
- CLI tool with `codex` command
- TypeScript SDK (`sdk/typescript`)
- Rust-based core (`codex-rs`)
- No explicit plugin system
- Primarily CLI-focused

**Architecture:**
```
Codex CLI (Rust core)
└── TypeScript SDK wrapper (optional)
```

**Integration with CodeFoundry:**
Since Codex lacks a plugin system, use the TypeScript SDK in a standalone service:

```typescript
// Standalone HTTP service implementing CodeFoundry hooks
import { Codex } from "@openai/codex";

const codex = new Codex();

app.post("/hooks/pre-subagent", async (req, res) => {
  const { task, worktree_path } = req.body;
  
  // Execute via Codex SDK
  const result = await codex.run({
    prompt: task.description,
    cwd: worktree_path
  });
  
  res.json({
    status: "ok",
    continue: true,
    subagent_result: result
  });
});
```

#### Claude Code

**Extension Model:** Plugin system similar to OpenCode

**Key Features:**
- Plugin directory: `plugins/`
- Custom commands and agents
- Event-driven (similar to OpenCode)
- Likely TypeScript-based

**Integration with CodeFoundry:**
Similar to OpenCode - implement hooks via HTTP server in Claude Code plugin.

### Why External Hooks (Not Internal SDKs)?

CodeFoundry uses **external HTTP hooks** rather than internal SDK integration for these reasons:

#### 1. Language Agnostic

**Problem:** Each harness SDK is language-specific
- OpenCode: JavaScript/TypeScript only
- Copilot SDK: TypeScript, Python, Go, .NET
- Codex: TypeScript only
- Claude Code: Likely TypeScript

**Solution:** HTTP is universal
- Any language can implement hooks
- Go, Rust, Python, TypeScript, etc.
- No dependency on harness language

#### 2. Loose Coupling

**Internal SDK Integration (Tight Coupling):**
```go
// Bad: CodeFoundry imports harness SDK
import (
    opencode "github.com/opencode-ai/sdk"
    copilot "github.com/github/copilot-sdk/go"
    codex "github.com/openai/codex-sdk/go"
)

// CodeFoundry must know about every harness
func ExecuteTask(task Task, harnessType string) {
    switch harnessType {
    case "opencode":
        opencode.SubagentSpawn(task)
    case "copilot":
        copilot.Execute(task)
    case "codex":
        codex.Run(task)
    }
}
```

**External Hooks (Loose Coupling):**
```go
// Good: CodeFoundry knows only HTTP
func ExecuteTask(task Task, hookURL string) {
    resp := http.Post(hookURL+"/pre-subagent", JSON, task)
    // Hook implementation uses whatever SDK it wants
}
```

#### 3. Fail-Closed Guarantees

**Internal SDK:**
- SDK errors may crash CodeFoundry
- Hard to enforce timeouts
- Unclear failure boundaries

**External Hooks:**
- HTTP timeouts are enforced
- Clear failure modes (network error, timeout, HTTP error)
- CodeFoundry controls retry logic
- Deterministic behavior

#### 4. Process Isolation

**Internal SDK:**
- Same process = shared memory
- Harness bugs affect CodeFoundry
- No resource limits on harness

**External Hooks:**
- Separate processes
- Harness crashes don't affect CodeFoundry
- Can enforce memory/CPU limits
- Can kill runaway subagents

#### 5. Determinism

**Key Principle:** CodeFoundry must be deterministic

**Internal SDKs:**
- SDK behavior changes with versions
- Hard to reproduce exact execution
- Non-deterministic LLM outputs mixed with deterministic logic

**External Hooks:**
- Contract-based (JSON schemas)
- Versioned hook APIs
- Deterministic state machine
- LLM uncertainty isolated in harness

### Comparison Matrix

| Aspect | Internal SDK | External Hooks |
|--------|--------------|----------------|
| **Language** | Harness-specific | Any language |
| **Coupling** | Tight | Loose |
| **Isolation** | Same process | Separate process |
| **Fail-closed** | Hard | Natural (HTTP timeouts) |
| **Determinism** | Difficult | Enforced |
| **Maintainability** | High | Low |
| **Performance** | Lower latency | Higher latency (HTTP) |
| **Flexibility** | Low | High |
| **Multi-harness** | Complex | Simple |

### When to Use Each Approach

**Use Internal SDK When:**
- Building a single-harness solution
- Performance is critical (sub-100ms latency)
- Team uses only one harness
- Fine-grained control over harness behavior needed

**Use External Hooks (CodeFoundry) When:**
- Supporting multiple harnesses
- Determinism and safety are priorities
- Want loose coupling
- Team uses different harnesses for different projects
- Need fail-closed guarantees
- Process isolation required

### Hybrid Approach

For maximum flexibility, CodeFoundry supports both:

```yaml
# Protocol definition
stages:
  - id: implement
    type: task_prompt
    hooks:
      pre_subagent:
        # Option 1: External HTTP hook
        - type: api
          url: "http://localhost:8081/hooks/pre-subagent"
        
        # Option 2: Internal command (no HTTP overhead)
        - type: command
          command: "./spawn-subagent.sh {{.TaskID}}"
```

**Internal commands** are useful when:
- Harness runs on same machine
- HTTP overhead is unacceptable
- Using a single harness consistently

**External HTTP hooks** are better when:
- Harness runs remotely
- Supporting multiple harness types
- Need process isolation
- Want network-level security (TLS, auth)

### Implementation Recommendation

**For Production Use:**

1. **Primary:** External HTTP hooks with HTTPS + HMAC signing
2. **Fallback:** Commands for local development
3. **Security:** Always verify hook signatures
4. **Monitoring:** Log all hook calls and responses
5. **Timeouts:** Enforce strict timeouts (30s default)

**Example Production Setup:**

```yaml
# Production protocol
stages:
  - id: implement
    hooks:
      pre_subagent:
        - type: api
          url: "https://harness.company.com/hooks/pre-subagent"
          timeout: 60
          retry: 3
          secret: "${HOOK_SECRET}"  # HMAC signing
          headers:
            Authorization: "Bearer ${API_TOKEN}"
      
      pre_merge:
        - type: api
          url: "https://harness.company.com/hooks/security-gate"
          timeout: 120
          # No retry - security gate should not be retried blindly
```

## Support

For integration questions:

- GitHub Issues: [mhingston/codefoundry](https://github.com/mhingston/codefoundry)
- Documentation: [docs/](/docs/)
- Examples: [examples/](/examples/)
