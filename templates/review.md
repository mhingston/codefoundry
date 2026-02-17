# Review Stage

## Context

Performing AI-powered code review with rubric-based scoring.

**Stage:** {{.StageID}}

{{if .Diff}}
### Diff to Review

```diff
{{.Diff}}
```
{{end}}

{{if .GateReports}}
### Gate Results

{{range .GateReports}}
**{{.GateID}}**: {{.Status}}
{{if .Failures}}
Failures:
{{range .Failures}}
- {{.File}}:{{.Line}} - {{.Message}}
{{end}}
{{end}}

{{end}}
{{end}}

## Review Rubric

Score each dimension on a scale of 1-5:

### 1. CORRECTNESS (1-5)
Does the code solve the stated problem accurately?
- **5**: Perfect solution, meets all requirements
- **4**: Minor issues, mostly correct
- **3**: Partial solution with notable gaps
- **2**: Major issues, minimally functional
- **1**: Incorrect or completely off-track

### 2. EFFICIENCY (1-5)
How optimal is the solution in terms of resource usage?
- **5**: Highly optimized, minimal overhead
- **4**: Reasonably efficient
- **3**: Acceptable but could be improved
- **2**: Inefficient with unnecessary overhead
- **1**: Severely inefficient or wasteful

### 3. MAINTAINABILITY (1-5)
How easy is it to maintain and extend?
- **5**: Clean, well-documented, follows best practices
- **4**: Good structure, minor improvements needed
- **3**: Acceptable, some technical debt
- **2**: Hard to maintain, lacks documentation
- **1**: Unmaintainable, no structure

### 4. SAFETY (1-5)
Are there security issues or harmful changes?
- **5**: No issues, follows security best practices
- **4**: Minor concerns, generally safe
- **3**: Some concerns need attention
- **2**: Notable security issues
- **1**: Critical security flaws or harmful content

## Output Format

Provide your evaluation as valid JSON:

```json
{
  "schema_version": "codefoundry_review_result.v1",
  "rubric_score": 85,
  "confidence_score": 0.85,
  "confidence_threshold": 0.7,
  "dimensions": {
    "correctness": 5,
    "efficiency": 4,
    "maintainability": 4,
    "safety": 5
  },
  "findings": [
    {
      "id": "finding-1",
      "severity": "P2",
      "file": "internal/example.go",
      "line": 42,
      "message": "Consider adding error handling for this edge case",
      "category": "maintainability"
    }
  ],
  "p1_count": 0,
  "p2_count": 1,
  "p3_count": 0,
  "summary": "Overall good implementation with minor maintainability improvements needed."
}
```

## Severity Definitions

- **P1 (Must fix)**: Security vulnerabilities, data loss, crashes, race conditions
- **P2 (Should fix)**: Performance issues, missing tests, maintainability concerns
- **P3 (Nice to fix)**: Style issues, minor optimizations, documentation

## Task

1. Review the code change carefully
2. Score each dimension 1-5 based on the rubric
3. Calculate rubric_score as weighted average: (Correctness*0.4 + Efficiency*0.2 + Maintainability*0.2 + Safety*0.2) * 5
4. Assess confidence_score (0.0-1.0) based on certainty
5. Identify findings and classify severity
6. Provide a brief summary
7. Output valid JSON matching the schema
