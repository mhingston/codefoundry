# Contracts

CodeFoundry uses machine-checkable contracts for all artifacts.

## Schema Validation

All JSON artifacts must validate against their schemas:

```bash
# Validate status.json
codefoundry validate --schema schemas/status.schema.json --file status.json

# Validate protocol
codefoundry validate --schema schemas/protocol.schema.json --file protocol.yaml
```

## Stage Status Contract

**File:** `.codefoundry/artifacts/<run-id>/<stage-id>/status.json`

**Schema:** `codefoundry_stage_status.v1`

**Required Fields:**
- `schema_version` - Must be `"codefoundry_stage_status.v1"`
- `stage_id` - Stage identifier
- `status` - One of: `pending`, `running`, `pass`, `fail`, `skipped`

**Optional Fields:**
- `summary` - Human-readable summary
- `evidence` - Array of evidence file paths
- `started_at` - ISO 8601 timestamp
- `completed_at` - ISO 8601 timestamp
- `duration_ms` - Duration in milliseconds
- `error` - Error message if failed
- `metadata` - Stage-specific metadata

**Example:**
```json
{
  "schema_version": "codefoundry_stage_status.v1",
  "stage_id": "verify",
  "status": "pass",
  "summary": "All required gates passed",
  "evidence": [
    ".codefoundry/artifacts/run-abc/verify/lint.json",
    ".codefoundry/artifacts/run-abc/verify/test.json"
  ],
  "started_at": "2026-02-16T10:00:00Z",
  "completed_at": "2026-02-16T10:05:30Z",
  "duration_ms": 330000
}
```

## Gate Report Contract

**File:** `.codefoundry/artifacts/<run-id>/<stage-id>/<gate-id>.json`

**Schema:** `codefoundry_gate_report.v1`

**Required Fields:**
- `schema_version` - Must be `"codefoundry_gate_report.v1"`
- `gate_id` - Gate identifier
- `status` - One of: `pass`, `fail`, `error`, `skipped`

**Optional Fields:**
- `command` - Command that was executed
- `exit_code` - Process exit code
- `duration_ms` - Execution duration
- `stdout` - Standard output
- `stderr` - Standard error
- `failures` - Array of failure objects
- `timestamp` - ISO 8601 timestamp

**Failure Object:**
- `message` - Required
- `file` - Optional file path
- `line` - Optional line number
- `severity` - One of: `error`, `warning`, `info`

**Example:**
```json
{
  "schema_version": "codefoundry_gate_report.v1",
  "gate_id": "test",
  "status": "fail",
  "command": "go test ./...",
  "exit_code": 1,
  "duration_ms": 12345,
  "stdout": "...",
  "stderr": "",
  "failures": [
    {
      "message": "TestGetUser failed",
      "file": "user_test.go",
      "line": 45,
      "severity": "error"
    }
  ],
  "timestamp": "2026-02-16T10:00:00Z"
}
```

## Lock Decision Contract

**File:** `.codefoundry/artifacts/<run-id>/lock/lock-decision.json`

**Schema:** `codefoundry_lock_decision.v1`

**Required Fields:**
- `schema_version` - Must be `"codefoundry_lock_decision.v1"`
- `decision` - One of: `resolved`, `reopen`
- `timestamp` - ISO 8601 timestamp

**Optional Fields:**
- `reason` - Human-readable reasoning
- `required_gate_ids` - Array of required gate IDs
- `passed_gate_ids` - Array of passed gate IDs
- `failed_gate_ids` - Array of failed gate IDs
- `confidence_score` - 0.0-1.0
- `confidence_threshold` - 0.0-1.0
- `p1_findings` - Count of P1 findings
- `p2_findings` - Count of P2 findings
- `p3_findings` - Count of P3 findings
- `rubric_score` - 0-100
- `escalation_required` - Boolean
- `escalation_reason` - String
- `metadata` - Additional metadata

**Example:**
```json
{
  "schema_version": "codefoundry_lock_decision.v1",
  "decision": "resolved",
  "reason": "All gates passed, confidence 0.92 >= 0.80, no P1 findings",
  "required_gate_ids": ["lint", "typecheck", "test"],
  "passed_gate_ids": ["lint", "typecheck", "test"],
  "failed_gate_ids": [],
  "confidence_score": 0.92,
  "confidence_threshold": 0.80,
  "p1_findings": 0,
  "p2_findings": 2,
  "p3_findings": 1,
  "rubric_score": 92,
  "timestamp": "2026-02-16T10:00:00Z"
}
```

## State Contract

**File:** `.codefoundry/state/state.json`

**Schema:** `codefoundry_state.v1`

**Required Fields:**
- `schema_version` - Must be `"codefoundry_state.v1"`
- `run_id` - Unique run identifier
- `protocol_version` - Protocol version used
- `stages` - Object keyed by stage ID

**Stage Object:**
- `status` - Required, one of: `pending`, `running`, `pass`, `fail`, `skipped`
- `artifact_path` - Path to artifacts
- `started_at` - ISO 8601 timestamp
- `completed_at` - ISO 8601 timestamp
- `error` - Error message

**Optional Fields:**
- `current_stage` - ID of currently executing stage
- `checkpoint` - Checkpoint data
- `metadata` - Run metadata
- `updated_at` - Last update timestamp

**Example:**
```json
{
  "schema_version": "codefoundry_state.v1",
  "run_id": "run-20260216-abc123",
  "protocol_version": "1.0.0",
  "stages": {
    "plan": {
      "status": "pass",
      "artifact_path": ".codefoundry/artifacts/run-abc/plan"
    },
    "spec": {
      "status": "running",
      "started_at": "2026-02-16T10:30:00Z"
    }
  },
  "current_stage": "spec",
  "updated_at": "2026-02-16T10:30:00Z"
}
```

## Protocol Contract

**File:** `.codefoundry/protocols/*.yaml`

**Schema:** See `schemas/protocol.schema.json`

**Required Fields:**
- `name` - Protocol name
- `version` - Semantic version
- `stages` - Array of stages

**Stage Fields:**
- `id` - Required, unique identifier
- `name` - Required, human-readable name
- `type` - Optional: `spec`, `decompose`, `parallel`, `decision`
- `template` - Optional, template filename
- `depends_on` - Optional, array of stage IDs
- `inputs` - Optional, array of input files
- `outputs` - Optional, array of output files
- `gates` - Optional, array of gate IDs
- `condition` - Optional, conditional expression
- `source` - Optional, task source for decompose

**Gate Fields:**
- `id` - Required, gate identifier
- `command` - Required, shell command
- `name` - Optional, human-readable name
- `required` - Optional, boolean (default: true)
- `timeout` - Optional, seconds (default: 300)
- `env` - Optional, environment variables

**Example:**
```yaml
name: "default"
version: "1.0.0"

stages:
  - id: plan
    name: "Plan"
    template: plan.md
    outputs: [plan.md]
    
  - id: verify
    name: "Verify"
    depends_on: [implement]
    gates: [lint, test]

gates:
  - id: lint
    command: "go vet ./..."
    required: true
```

## Validation Rules

### Automatic Validation

The system automatically validates:

1. All JSON artifacts against their schemas
2. Protocol DAG (no cycles in dependencies)
3. Stage input/output contracts
4. Gate existence (referenced gates must be defined)

### Manual Validation

You can manually validate:

```bash
# Validate a specific artifact
codefoundry validate --file artifact.json --schema schemas/status.schema.json

# Validate protocol
codefoundry validate --protocol protocol.yaml

# Validate entire run
codefoundry validate --run run-id
```

### CI Integration

In CI, validate contracts:

```yaml
# .github/workflows/ci.yml
- name: Validate Contracts
  run: |
    codefoundry validate --run latest
    codefoundry bundle --run latest
```

## Schema Evolution

### Versioning

- Schemas use semantic versioning
- Minor changes are backward compatible
- Major changes require migration

### Adding Fields

New optional fields can be added in minor versions:

```json
{
  "schema_version": "codefoundry_stage_status.v1",
  // ... existing fields
  "new_field": "value"  // Optional, new in v1.1
}
```

### Breaking Changes

Breaking changes require new schema version:

```json
{
  "schema_version": "codefoundry_stage_status.v2",  // Breaking change
  // ... fields may differ from v1
}
```

## Contract Testing

All contracts have corresponding tests:

```go
func TestStatusContract(t *testing.T) {
    tests := []struct {
        name    string
        status  StageStatus
        wantErr bool
    }{
        {
            name: "valid status",
            status: StageStatus{
                SchemaVersion: "codefoundry_stage_status.v1",
                StageID:       "test",
                Status:        "pass",
            },
            wantErr: false,
        },
        // ... more cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateStatus(tt.status)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateStatus() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Contract Violations

If a contract is violated:

1. Stage execution fails
2. Error is logged
3. State shows `status: "fail"`
4. Human intervention required

Violations are not silently ignored.
