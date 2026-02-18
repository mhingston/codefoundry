package review

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/mhingston/codefoundry/internal/gate"
)

// TemplateContext provides data for review template rendering
type TemplateContext struct {
	StageID     string
	Diff        string
	GateReports []GateReport
	Files       []FileInfo
}

// GateReport summarizes a gate execution
type GateReport struct {
	GateID   string
	Status   string
	Failures []gate.GateFailure
}

// FileInfo represents a file in the review
type FileInfo struct {
	Path     string
	Added    int
	Removed  int
	Language string
}

// TemplateLoader loads and renders review templates
type TemplateLoader struct {
	templatePath string
}

// NewTemplateLoader creates a new template loader
func NewTemplateLoader(templatePath string) *TemplateLoader {
	return &TemplateLoader{
		templatePath: templatePath,
	}
}

// Load loads the review template from file
func (l *TemplateLoader) Load() (string, error) {
	content, err := os.ReadFile(l.templatePath)
	if err != nil {
		return "", TemplateError{Op: "load", Message: fmt.Sprintf("failed to read template from %s", l.templatePath), Cause: err}
	}
	return string(content), nil
}

// DefaultTemplate returns the built-in default template
func DefaultTemplate() string {
	return `You are an expert code reviewer evaluating a code change. Rate the following across four dimensions on a scale of 1-5.

## Code Change Context

**Stage:** {{.StageID}}

{{if .Diff}}
### Diff
{{.Diff}}
{{end}}

{{if .GateReports}}
### Gate Results
{{range .GateReports}}
- **{{.GateID}}**: {{.Status}}
  {{if .Failures}}
  Failures:
  {{range .Failures}}
  - {{.File}}:{{.Line}} - {{.Message}}
  {{end}}
  {{end}}
{{end}}
{{end}}

## Rubric (Rate 1-5 for each dimension)

1. **CORRECTNESS** (1-5): Did the change solve the stated problem accurately?
   - 5: Perfect solution, meets all requirements
   - 4: Minor issues, mostly correct
   - 3: Partial solution with notable gaps
   - 2: Major issues, minimally functional
   - 1: Incorrect or completely off-track

2. **EFFICIENCY** (1-5): How optimal is the solution?
   - 5: Highly optimized, minimal overhead
   - 4: Reasonably efficient
   - 3: Acceptable but could be improved
   - 2: Inefficient with unnecessary overhead
   - 1: Severely inefficient or wasteful

3. **MAINTAINABILITY** (1-5): How easy is it to maintain and extend?
   - 5: Clean, well-documented, follows best practices
   - 4: Good structure, minor improvements needed
   - 3: Acceptable, some technical debt
   - 2: Hard to maintain, lacks documentation
   - 1: Unmaintainable, no structure

4. **SAFETY** (1-5): Are there security issues or harmful changes?
   - 5: No issues, follows security best practices
   - 4: Minor concerns, generally safe
   - 3: Some concerns need attention
   - 2: Notable security issues
   - 1: Critical security flaws or harmful content

## Output Format

Provide your evaluation as valid JSON:

{
  "schema_version": "codefoundry_review_result.v1",
  "rubric_score": 0-100,
  "confidence_score": 0.0-1.0,
  "confidence_threshold": 0.7,
  "dimensions": {
    "correctness": 1-5,
    "efficiency": 1-5,
    "maintainability": 1-5,
    "safety": 1-5
  },
  "findings": [
    {
      "id": "finding-1",
      "severity": "P1|P2|P3",
      "file": "path/to/file",
      "line": 123,
      "message": "Description of the issue",
      "category": "security|performance|maintainability|style"
    }
  ],
  "summary": "Brief summary of the review"
}
`
}

// Render renders the template with the given context
func (l *TemplateLoader) Render(ctx TemplateContext) (string, error) {
	tmplContent, err := l.Load()
	if err != nil {
		// Fall back to default template
		tmplContent = DefaultTemplate()
	}

	tmpl, err := template.New("review").Parse(tmplContent)
	if err != nil {
		return "", TemplateError{Op: "parse", Message: "failed to parse template", Cause: err}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", TemplateError{Op: "execute", Message: "failed to execute template", Cause: err}
	}

	return buf.String(), nil
}

// DetectLanguage detects the programming language from file extension
func DetectLanguage(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".go":
		return "go"
	case ".js", ".jsx":
		return "javascript"
	case ".ts", ".tsx":
		return "typescript"
	case ".py":
		return "python"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	default:
		return "unknown"
	}
}

// BuildContext builds a template context from inputs
func BuildContext(stageID, diff string, gateReports []GateReport) TemplateContext {
	return TemplateContext{
		StageID:     stageID,
		Diff:        diff,
		GateReports: gateReports,
	}
}
