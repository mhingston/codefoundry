# Hooks System

Hooks provide event-driven customization points where harnesses can inject logic without polling the CodeFoundry API.

## Overview

- **Event-driven:** CodeFoundry calls harness at key lifecycle points
- **Customizable:** Harness can modify behavior, add context, or block execution
- **Fail-closed:** Hook failures block progression by default
- **Optional:** Basic integration works without hooks

## CodeFoundry Hooks vs Harness Plugins

CodeFoundry hooks are **complementary but different** from harness plugin systems:

### Comparison

| Aspect | CodeFoundry Hooks | Harness Plugins (OpenCode, Claude Code) |
|--------|-------------------|-------------------------------------------|
| **Location** | External HTTP service | In-process (JS/TS modules) |
| **Language** | Any language | JavaScript/TypeScript only |
| **Scope** | Workflow orchestration | Tool customization, UI |
| **Determinism** | Enforced | Not guaranteed |
| **Fail-closed** | Required | Optional |
| **State** | Persistent JSON contracts | In-memory |
| **Isolation** | Process boundary | Same process |

### Relationship Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         HARNESS (e.g., OpenCode)                     │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                    Harness Plugin System                        │ │
│  │  ├─ Custom tools (file read, edit, bash)                     │ │
│  │  ├─ Event handlers (file.edited, tool.execute)                 │ │
│  │  ├─ UI customizations                                        │ │
│  │  └─ Session management                                        │ │
│  └────────────────────────────────────────────────────────────────┘ │
│                           │                                          │
│                           ▼                                          │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │            CodeFoundry Integration Plugin/Adapter               │ │
│  │  - Exposes HTTP server for CodeFoundry hooks                    │ │
│  │  - Uses harness SDK/client for LLM execution                  │ │
│  └────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
                              │
                              │ HTTP hooks
                              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                          CODEFOUNDRY                                 │
│  ┌────────────────────────────────────────────────────────────────┐ │
│  │                      CodeFoundry Hooks                        │ │
│  │  - pre_stage: Setup and context loading                     │ │
│  │  - pre_subagent: Spawn subagent with worktree               │ │
│  │  - post_subagent: Validate output                           │ │
│  │  - pre_merge: Security gates                                │ │
│  │  - post_stage: Cleanup                                      │ │
│  └────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────┘
```

### Separation of Concerns

**Harness Plugins Handle:**
- Custom tools (file operations, shell commands)
- UI customizations (themes, keybindings)
- Event handling (file changes, tool usage)
- Session compaction and context management
- LLM provider configuration

**CodeFoundry Hooks Handle:**
- Workflow orchestration (stages, dependencies)
- Worktree lifecycle (create, merge, destroy)
- Quality gates (lint, test, typecheck)
- Lock decisions (resolved/reopen)
- Artifact contracts and persistence
- Deterministic state management

### Example: OpenCode with CodeFoundry

**OpenCode Plugin (In-Process):**
```typescript
// .opencode/plugins/codefoundry-adapter.ts
export const CodeFoundryAdapter = async ({ client, $ }) => {
  // Start HTTP server for CodeFoundry hooks
  const server = Bun.serve({
    port: 8081,
    routes: {
      "/hooks/pre-subagent": async (req) => {
        const { task, worktree_path } = await req.json();
        
        // Use OpenCode's native capabilities
        const subagent = await client.subagent.spawn({
          task: task.description,
          worktree: worktree_path,
          // OpenCode handles LLM calls, tools, etc.
        });
        
        return Response.json({
          status: "ok",
          continue: true,
          subagent_id: subagent.id
        });
      },
      
      "/hooks/post-subagent": async (req) => {
        const { worktree } = await req.json();
        
        // Use OpenCode's validation tools
        const lint = await $`cd ${worktree.path} && npm run lint`;
        
        return Response.json({
          status: lint.exitCode === 0 ? "ok" : "failed",
          continue: lint.exitCode === 0,
          validation: { passed: lint.exitCode === 0 }
        });
      }
    }
  });

  return {
    // OpenCode native event handlers
    "session.created": async () => {
      console.log("CodeFoundry integration active");
    },
    
    "tool.execute.before": async (input, output) => {
      // Block .env file reads
      if (input.tool === "read" && output.args.filePath.includes(".env")) {
        throw new Error("Security: Cannot read .env files");
      }
    }
  };
};
```

**Key Insight:**
- OpenCode plugin system handles **harness-level** concerns (tools, UI, events)
- CodeFoundry hooks handle **workflow-level** concerns (orchestration, gates, worktrees)
- The adapter bridges both worlds

### Example: GitHub Copilot SDK with CodeFoundry

```typescript
// Standalone service using Copilot SDK
import { CopilotClient } from "@github/copilot-sdk";

const copilot = new CopilotClient();
const app = express();

// CodeFoundry calls this hook
app.post("/hooks/pre-subagent", async (req, res) => {
  const { task, worktree_path } = req.body;
  
  // Use Copilot SDK for LLM execution
  const result = await copilot.execute({
    task: task.description,
    worktree: worktree_path,
    tools: ["read_file", "edit_file", "bash"],
    model: "gpt-4o"
  });
  
  res.json({
    status: result.success ? "ok" : "failed",
    continue: result.success,
    subagent_result: {
      output: result.output,
      files_modified: result.modifiedFiles,
      turns_used: result.turns,
      tokens_used: result.tokenUsage
    }
  });
});
```

### When to Use What

**Use Harness Plugins When:**
- Customizing the harness itself (tools, UI, themes)
- Adding new capabilities to the harness
- Modifying harness behavior
- Working within a single harness ecosystem

**Use CodeFoundry Hooks When:**
- Orchestrating multi-stage workflows
- Managing worktrees and parallel execution
- Enforcing quality gates
- Making lock decisions
- Supporting multiple harness types
- Need deterministic, fail-closed behavior

**Use Both When:**
- Building production workflows
- Need harness customization AND workflow orchestration
- Want best of both worlds

### Best Practice

**Recommended Architecture:**

1. **CodeFoundry** - Central workflow orchestrator
2. **CodeFoundry Hooks** - HTTP endpoints implemented by harness
3. **Harness Plugin/Adapter** - Bridges CodeFoundry hooks to harness SDK
4. **Harness Native Plugins** - Handle harness-specific customizations

This keeps concerns separated while allowing rich integration.

## Hook Lifecycle

```
┌─────────────────────────────────────────────────────────────────────┐
│                           STAGE EXECUTION                           │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. PRE_STAGE                                                       │
│     └─ CodeFoundry → Harness: Setup, context loading                │
│                                                                     │
│  2. For each task wave:                                              │
│     ├─ Create worktrees                                             │
│     │                                                                  │
│     ├─ PRE_SUBAGENT (per task)                                      │
│     │  └─ CodeFoundry → Harness: Spawn subagent                       │
│     │     └─ Harness spawns LLM in worktree                         │
│     │                                                                  │
│     ├─ [Subagent executes]                                          │
│     │                                                                  │
│     ├─ POST_SUBAGENT (per task)                                     │
│     │  └─ CodeFoundry → Harness: Validate output                    │
│     │     └─ Harness reports completion                             │
│     │                                                                  │
│     ├─ PRE_MERGE (per worktree)                                     │
│     │  └─ CodeFoundry → Harness: Final validation                 │
│     │     └─ Harness can approve/block merge                        │
│     │                                                                  │
│     └─ CodeFoundry merges worktrees                                 │
│                                                                     │
│  3. POST_STAGE                                                      │
│     └─ CodeFoundry → Harness: Cleanup, notifications                │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Hook Types

### pre_stage

Called before stage execution begins.

**Use Cases:**
- Load custom context or templates
- Set up external resources
- Validate preconditions
- Inject secrets or configuration

**Example:**
```yaml
hooks:
  pre_stage:
    - type: api
      url: "http://localhost:8081/hooks/pre-stage"
      timeout: 30
```

### pre_subagent

Called before spawning each subagent.

**Use Cases:**
- Customize resource limits per task
- Add task-specific context
- Modify environment variables
- Inject additional template variables

**Example:**
```yaml
hooks:
  pre_subagent:
    - type: api
      url: "http://localhost:8081/hooks/pre-subagent"
```

### post_subagent

Called after subagent completes.

**Use Cases:**
- Validate output quality
- Run custom checks
- Transform artifacts
- Decide whether to proceed

**Example:**
```yaml
hooks:
  post_subagent:
    - type: api
      url: "http://localhost:8081/hooks/post-subagent"
```

### pre_merge

Called before merging each worktree.

**Use Cases:**
- Security review of changes
- Final validation
- Human approval for sensitive changes
- Conflict detection

**Example:**
```yaml
hooks:
  pre_merge:
    - type: api
      url: "http://localhost:8081/hooks/pre-merge"
      timeout: 60
```

### post_stage

Called after stage completes.

**Use Cases:**
- Send notifications
- Update metrics
- Clean up resources
- Archive artifacts

**Example:**
```yaml
hooks:
  post_stage:
    - type: api
      url: "http://localhost:8081/hooks/post-stage"
      async: true  # Non-blocking
```

## Configuration

### Protocol Definition

```yaml
stages:
  - id: implement
    type: task_prompt
    source: decompose
    hooks:
      pre_stage:
        - type: api
          url: "http://localhost:8081/hooks/pre-stage"
          timeout: 30
          retry: 3
          headers:
            Authorization: "Bearer ${HOOK_TOKEN}"
          secret: "${HOOK_SECRET}"  # For request signing
      
      pre_subagent:
        - type: api
          url: "http://localhost:8081/hooks/pre-subagent"
      
      post_subagent:
        - type: api
          url: "http://localhost:8081/hooks/post-subagent"
      
      pre_merge:
        - type: api
          url: "http://localhost:8081/hooks/pre-merge"
          timeout: 60
      
      post_stage:
        - type: api
          url: "http://localhost:8081/hooks/post-stage"
          async: true
```

### Hook Types

**api** - HTTP POST request
```yaml
type: api
url: "http://host/hook"
timeout: 30        # Seconds
retry: 3           # Retry attempts on 5xx
async: false       # Whether non-blocking
headers:           # Custom headers
  X-Custom: value
secret: "token"    # For HMAC signing
```

**script** - Execute shell script
```yaml
type: script
command: "./scripts/hook.sh {{.RunID}} {{.StageID}}"
timeout: 30
env:               # Environment variables
  CF_RUN_ID: "{{.RunID}}"
```

**command** - Execute command directly
```yaml
type: command
command: "python validate.py"
timeout: 60
```

## Request/Response Contracts

### Request Schema

All hooks receive the same base structure with type-specific fields.

**Common Fields:**
```json
{
  "schema_version": "codefoundry_hook_request.v1",
  "hook_type": "pre_subagent",
  "run_id": "run-abc123",
  "stage_id": "implement",
  "stage_type": "task_prompt",
  "protocol_version": "1.0.0",
  "timestamp": "2026-02-16T10:00:00Z"
}
```

**Type-Specific Fields:**

- `pre_stage`: `previous_stage_outputs`
- `pre_subagent`: `task`, `worktree`, `default_limits`, `default_env`
- `post_subagent`: `task`, `worktree`, `subagent_result`, `artifacts`
- `pre_merge`: `task`, `worktree`, `merge_info`, `gate_results`
- `post_stage`: `stage_result`, `artifacts`

### Response Schema

**Success:**
```json
{
  "status": "ok",
  "continue": true,
  "metadata": {...}
}
```

**Block:**
```json
{
  "status": "blocked",
  "continue": false,
  "reason": "Validation failed"
}
```

**With Overrides (pre_subagent):**
```json
{
  "status": "ok",
  "continue": true,
  "overrides": {
    "limits": {
      "max_turns": 100
    },
    "template_vars": {
      "additional_context": "..."
    }
  }
}
```

## Security

### Request Signing

CodeFoundry signs requests with HMAC-SHA256:

```
X-CodeFoundry-Signature: sha256=<hex>
X-CodeFoundry-Timestamp: <unix-epoch>
X-CodeFoundry-Run-ID: <run-id>
```

**Verification:**
```go
func verifySignature(body []byte, signature string, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(signature), []byte(expected))
}
```

### Timeout and Retry

- **Timeout:** Default 30 seconds, configurable per hook
- **Retry:** Exponential backoff on 5xx or network errors
- **Async:** Post-stage hooks can be non-blocking

## Fail-Closed Behavior

| Hook Type | Failure Mode | Result |
|-----------|-------------|---------|
| pre_stage | Network error | Stage fails |
| pre_stage | HTTP 4xx/5xx | Stage fails |
| pre_stage | continue: false | Stage fails |
| pre_subagent | Network error | Task fails |
| pre_subagent | HTTP 4xx/5xx | Task fails |
| pre_subagent | continue: false | Task skipped |
| post_subagent | Network error | Task fails |
| post_subagent | HTTP 4xx/5xx | Task fails |
| post_subagent | continue: false | Task fails |
| pre_merge | Network error | Merge fails |
| pre_merge | HTTP 4xx/5xx | Merge fails |
| pre_merge | continue: false | Human escalation |
| post_stage | Any error | Logged only |

## Implementation Examples

### Express.js Harness

```javascript
const express = require('express');
const crypto = require('crypto');

const app = express();
app.use(express.json());

// Verify signature middleware
function verifySignature(req, res, next) {
  const signature = req.headers['x-codefoundry-signature'];
  const timestamp = req.headers['x-codefoundry-timestamp'];
  const secret = process.env.HOOK_SECRET;
  
  const body = JSON.stringify(req.body);
  const expected = crypto
    .createHmac('sha256', secret)
    .update(body)
    .digest('hex');
  
  if (!crypto.timingSafeEqual(
    Buffer.from(signature), 
    Buffer.from(expected)
  )) {
    return res.status(401).json({ error: 'Invalid signature' });
  }
  
  next();
}

app.use(verifySignature);

// Pre-stage: Load context
app.post('/hooks/pre-stage', (req, res) => {
  const { stage_id } = req.body;
  
  // Load custom patterns
  const patterns = loadPatterns();
  
  res.json({
    status: 'ok',
    continue: true,
    metadata: { patterns_loaded: patterns.length }
  });
});

// Pre-subagent: Customize by task
app.post('/hooks/pre-subagent', (req, res) => {
  const { task } = req.body;
  
  if (task.id.includes('auth')) {
    res.json({
      status: 'ok',
      continue: true,
      overrides: {
        limits: { max_turns: 100 },
        template_vars: { 
          security_guidelines: loadSecurityGuidelines() 
        }
      }
    });
  } else {
    res.json({ status: 'ok', continue: true });
  }
});

// Post-subagent: Validate
app.post('/hooks/post-subagent', async (req, res) => {
  const { worktree, subagent_result } = req.body;
  
  const validation = await validateOutput(worktree.path);
  
  if (!validation.passed) {
    return res.status(400).json({
      status: 'failed',
      continue: false,
      validation: validation
    });
  }
  
  res.json({ status: 'ok', continue: true });
});

// Pre-merge: Security check
app.post('/hooks/pre-merge', (req, res) => {
  const { merge_info } = req.body;
  
  if (hasSecuritySensitiveFiles(merge_info.files_changed)) {
    return res.status(403).json({
      status: 'blocked',
      continue: false,
      merge_approved: false,
      escalation_required: true,
      reason: 'Security review required'
    });
  }
  
  res.json({ status: 'ok', continue: true, merge_approved: true });
});

app.listen(8081);
```

### Go Harness

```go
package main

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "net/http"
)

type HookRequest struct {
    SchemaVersion string `json:"schema_version"`
    HookType      string `json:"hook_type"`
    RunID         string `json:"run_id"`
    StageID       string `json:"stage_id"`
}

type HookResponse struct {
    Status   string `json:"status"`
    Continue bool   `json:"continue"`
    Reason   string `json:"reason,omitempty"`
}

func verifySignature(body []byte, sig string, secret string) bool {
    mac := hmac.New(sha256.New, []byte(secret))
    mac.Write(body)
    expected := hex.EncodeToString(mac.Sum(nil))
    return hmac.Equal([]byte(sig), []byte(expected))
}

func hookHandler(w http.ResponseWriter, r *http.Request) {
    // Verify signature
    sig := r.Header.Get("X-CodeFoundry-Signature")
    body, _ := io.ReadAll(r.Body)
    
    if !verifySignature(body, sig, os.Getenv("HOOK_SECRET")) {
        http.Error(w, "Invalid signature", http.StatusUnauthorized)
        return
    }
    
    var req HookRequest
    json.Unmarshal(body, &req)
    
    // Handle based on hook type
    var resp HookResponse
    switch req.HookType {
    case "pre_stage":
        resp = handlePreStage(req)
    case "pre_subagent":
        resp = handlePreSubagent(req)
    // ...
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(resp)
}

func handlePreStage(req HookRequest) HookResponse {
    // Custom logic
    return HookResponse{Status: "ok", Continue: true}
}

func main() {
    http.HandleFunc("/hooks/", hookHandler)
    http.ListenAndServe(":8081", nil)
}
```

## Testing Hooks

### Local Development

```bash
# Start CodeFoundry with hook debugging
codefoundry run --hooks-debug

# Hook calls are logged with full request/response
```

### Mock Server

```javascript
// test/mock-harness.js
const express = require('express');
const app = express();
app.use(express.json());

app.post('/hooks/:type', (req, res) => {
  console.log(`Hook: ${req.params.type}`);
  console.log('Request:', JSON.stringify(req.body, null, 2));
  
  res.json({
    status: 'ok',
    continue: true
  });
});

app.listen(8081, () => {
  console.log('Mock harness on :8081');
});
```

## Troubleshooting

### Hook Not Called

Check:
1. Protocol YAML defines hook
2. URL is accessible from CodeFoundry
3. No firewall blocking
4. Correct Content-Type headers

### Hook Times Out

Solutions:
1. Increase `timeout` in hook config
2. Make hook async if it doesn't need to block
3. Optimize hook implementation
4. Check for infinite loops

### Signature Verification Fails

Check:
1. Secret matches between CodeFoundry and harness
2. Body not modified before verification
3. No extra whitespace in JSON
4. Timestamp within acceptable window

## Best Practices

1. **Idempotent:** Hooks should be safe to retry
2. **Fast:** Keep hooks under 5 seconds when possible
3. **Validated:** Always verify request signatures
4. **Logged:** Log all hook calls and responses
5. **Fail Gracefully:** Provide clear error messages
6. **Async for Cleanup:** Use `async: true` for post-stage cleanup

## Migration from Polling

If currently polling:

**Before (Polling):**
```javascript
while (true) {
  const state = await codefoundry.getState(runId);
  if (state.current_stage === 'implement') {
    // Do work
  }
  await sleep(1000);
}
```

**After (Hooks):**
```javascript
app.post('/hooks/pre-subagent', (req, res) => {
  const { task, worktree } = req.body;
  // Do work when CodeFoundry calls
  spawnSubagent(task, worktree.path);
  res.json({ status: 'ok', continue: true });
});
```

**Benefits:**
- No polling overhead
- Real-time response
- Cleaner code
- Better resource usage
