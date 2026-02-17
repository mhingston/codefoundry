# Specification Stage

## Context

Based on plan: {{.Inputs.plan}}

## Task

Create a detailed technical specification based on the approved plan.

## Output Format

```markdown
# Specification: [Feature Name]

## Technical Design

### Architecture
Describe the high-level architecture and key components.

### Data Models
Define any data structures or schema changes.

### API Changes
Document any new or modified APIs.

## Assumptions
- Assumption 1
- Assumption 2

## Constraints
- Constraint 1
- Constraint 2

## Implementation Strategy

### Phase 1
Description of first phase

### Phase 2
Description of second phase

## Testing Strategy
- Unit tests: [what to test]
- Integration tests: [what to test]
- Edge cases: [what to consider]

## Rollback Plan
Steps to revert if issues arise
```

## Constraints

- Must align with existing patterns
- Consider performance implications
- Security review required for auth changes
