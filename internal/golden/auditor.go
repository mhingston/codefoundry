package golden

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Violation represents a principle violation
type Violation struct {
	Principle Principle `json:"principle"`
	File      string    `json:"file"`
	Line      int       `json:"line"`
	Column    int       `json:"column"`
	Message   string    `json:"message"`
	Snippet   string    `json:"snippet,omitempty"`
}

// AuditReport contains the results of a golden principles audit
type AuditReport struct {
	Violations []Violation `json:"violations"`
	Summary    Summary     `json:"summary"`
	Files      []FileInfo  `json:"files"`
}

// Summary contains audit summary statistics
type Summary struct {
	Total       int `json:"total"`
	Warnings    int `json:"warnings"`
	Errors      int `json:"errors"`
	FilesChecked int `json:"files_checked"`
}

// FileInfo contains information about a checked file
type FileInfo struct {
	Path       string `json:"path"`
	Violations int    `json:"violations"`
	Lines      int    `json:"lines"`
}

// Auditor audits code against golden principles
type Auditor struct {
	principles []Principle
	exclude    []string // Patterns to exclude
}

// NewAuditor creates a new auditor with the given principles
func NewAuditor(principles []Principle) *Auditor {
	return &Auditor{
		principles: principles,
		exclude: []string{
			"vendor/",
			"node_modules/",
			".git/",
			"_test.go", // Can be configured to include tests
		},
	}
}

// WithExclude sets exclude patterns
func (a *Auditor) WithExclude(patterns []string) *Auditor {
	a.exclude = patterns
	return a
}

// Audit audits a project directory
func (a *Auditor) Audit(projectPath string) (*AuditReport, error) {
	report := &AuditReport{
		Violations: make([]Violation, 0),
		Files:      make([]FileInfo, 0),
	}

	// Walk the directory
	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		if info.IsDir() {
			// Check if directory should be excluded
			if a.shouldExclude(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip excluded files
		if a.shouldExclude(path) {
			return nil
		}

		// Check file extension
		if !a.shouldCheckFile(path) {
			return nil
		}

		// Audit the file
		violations, fileInfo, err := a.CheckFile(path)
		if err != nil {
			return nil // Continue on error
		}

		report.Violations = append(report.Violations, violations...)
		if fileInfo != nil {
			report.Files = append(report.Files, *fileInfo)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("audit failed: %w", err)
	}

	// Calculate summary
	a.calculateSummary(report)

	return report, nil
}

// CheckFile checks a single file for violations
func (a *Auditor) CheckFile(filePath string) ([]Violation, *FileInfo, error) {
	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	contentStr := string(content)
	lines := strings.Split(contentStr, "\n")

	violations := make([]Violation, 0)

	// Check each principle
	for _, principle := range a.principles {
		if principle.regex == nil {
			continue
		}

		// Find all matches
		matches := principle.FindAllMatches(contentStr)
		for _, match := range matches {
			// Calculate line and column
			line, column := positionToLineCol(contentStr, match.Start)

			violation := Violation{
				Principle: principle,
				File:      filePath,
				Line:      line,
				Column:    column,
				Message:   principle.Message,
				Snippet:   getSnippet(lines, line),
			}
			violations = append(violations, violation)
		}
	}

	fileInfo := &FileInfo{
		Path:       filePath,
		Violations: len(violations),
		Lines:      len(lines),
	}

	return violations, fileInfo, nil
}

// shouldExclude checks if a path should be excluded
func (a *Auditor) shouldExclude(path string) bool {
	for _, pattern := range a.exclude {
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// shouldCheckFile checks if a file should be checked
func (a *Auditor) shouldCheckFile(path string) bool {
	extensions := []string{".go", ".js", ".ts", ".py", ".java", ".md"}
	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// calculateSummary calculates the report summary
func (a *Auditor) calculateSummary(report *AuditReport) {
	report.Summary.Total = len(report.Violations)
	report.Summary.FilesChecked = len(report.Files)

	for _, v := range report.Violations {
		if v.Principle.IsError() {
			report.Summary.Errors++
		} else {
			report.Summary.Warnings++
		}
	}
}

// positionToLineCol converts a byte position to line and column numbers
func positionToLineCol(content string, pos int) (line, column int) {
	line = 1
	column = 1

	for i := 0; i < pos && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			column = 1
		} else {
			column++
		}
	}

	return line, column
}

// getSnippet gets a snippet of code around a line
func getSnippet(lines []string, lineNum int) string {
	if lineNum < 1 || lineNum > len(lines) {
		return ""
	}

	// Return the line with some context (previous, current, next)
	start := lineNum - 2
	if start < 0 {
		start = 0
	}

	end := lineNum + 1
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

// HasErrors returns true if the report contains errors
func (r *AuditReport) HasErrors() bool {
	return r.Summary.Errors > 0
}

// HasWarnings returns true if the report contains warnings
func (r *AuditReport) HasWarnings() bool {
	return r.Summary.Warnings > 0
}

// GetViolationsByFile returns violations grouped by file
func (r *AuditReport) GetViolationsByFile() map[string][]Violation {
	grouped := make(map[string][]Violation)
	for _, v := range r.Violations {
		grouped[v.File] = append(grouped[v.File], v)
	}
	return grouped
}

// GetViolationsByPrinciple returns violations grouped by principle ID
func (r *AuditReport) GetViolationsByPrinciple() map[string][]Violation {
	grouped := make(map[string][]Violation)
	for _, v := range r.Violations {
		grouped[v.Principle.ID] = append(grouped[v.Principle.ID], v)
	}
	return grouped
}

// FilterViolationsBySeverity returns violations filtered by severity
func (r *AuditReport) FilterViolationsBySeverity(severity string) []Violation {
	filtered := make([]Violation, 0)
	for _, v := range r.Violations {
		if v.Principle.Severity == severity {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// String returns a string representation of the audit report
func (r *AuditReport) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Golden Principles Audit Report\n"))
	sb.WriteString(fmt.Sprintf("================================\n\n"))
	sb.WriteString(fmt.Sprintf("Files checked: %d\n", r.Summary.FilesChecked))
	sb.WriteString(fmt.Sprintf("Total violations: %d\n", r.Summary.Total))
	sb.WriteString(fmt.Sprintf("  Errors: %d\n", r.Summary.Errors))
	sb.WriteString(fmt.Sprintf("  Warnings: %d\n\n", r.Summary.Warnings))

	if len(r.Violations) > 0 {
		sb.WriteString("Violations:\n")
		sb.WriteString("-----------\n")

		byFile := r.GetViolationsByFile()
		for file, violations := range byFile {
			sb.WriteString(fmt.Sprintf("\n%s:\n", file))
			for _, v := range violations {
				prefix := "⚠️"
				if v.Principle.IsError() {
					prefix = "❌"
				}
				sb.WriteString(fmt.Sprintf("  %s [%s] Line %d: %s\n",
					prefix, v.Principle.ID, v.Line, v.Message))
			}
		}
	} else {
		sb.WriteString("✅ No violations found!\n")
	}

	return sb.String()
}
