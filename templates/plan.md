# Planning Stage

## Context

Repository: {{.Repository}}
Branch: {{.Branch}}
Current Stage: {{.StageID}}

## Task

Create a comprehensive plan for the requested feature or change.

## Output Format

Provide your response in the following structure:

```markdown
# Plan: [Feature Name]

## Overview
Brief description of what we're building and why.

## Scope
### In Scope
- Item 1
- Item 2

### Out of Scope
- Item 1
- Item 2

## Risks
| Risk | Mitigation | Severity |
|------|-----------|----------|
| Risk description | How to mitigate | High/Medium/Low |

## Acceptance Criteria
- [ ] Criterion 1
- [ ] Criterion 2

## Dependencies
- Dependency 1
- Dependency 2

## Estimation
- Complexity: [Low/Medium/High]
- Confidence: [0.0-1.0]
```

## Notes

- Be specific about scope boundaries
- Identify blockers early
- Consider testing strategy
- Think about backward compatibility
