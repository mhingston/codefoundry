# Decomposition Stage

## Context

Based on spec: {{.Inputs.spec}}

## Task

Break the specification into concrete, executable tasks.

## Output Format

Provide a YAML list of tasks:

```yaml
tasks:
  - id: task-001
    title: "Task title"
    description: "Detailed description"
    acceptance_criteria:
      - "Criterion 1"
      - "Criterion 2"
    dependencies: []
    estimated_effort: "small" # small/medium/large
    files_to_modify:
      - "path/to/file.go"
    
  - id: task-002
    title: "Another task"
    description: "Description"
    acceptance_criteria:
      - "Criterion 1"
    dependencies: [task-001]
    estimated_effort: "medium"
    files_to_modify:
      - "path/to/file.go"
      - "path/to/test.go"
```

## Guidelines

- Tasks should be independently verifiable
- Each task should have clear success criteria
- Dependencies must form a DAG (no cycles)
- Keep tasks small enough for single-session work
- Include test files in task scope
