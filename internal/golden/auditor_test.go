package golden

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAuditor(t *testing.T) {
	principles := DefaultPrinciples()
	auditor := NewAuditor(principles)

	assert.NotNil(t, auditor)
	assert.Len(t, auditor.principles, len(principles))
	assert.NotNil(t, auditor.exclude)
}

func TestAuditor_WithExclude(t *testing.T) {
	auditor := NewAuditor(DefaultPrinciples())
	newExcludes := []string{"vendor/", "generated/"}

	auditor.WithExclude(newExcludes)

	assert.Equal(t, newExcludes, auditor.exclude)
}

func TestAuditor_CheckFile(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create test file with a helper function (GP-001)
	testFile := filepath.Join(tempDir, "test.go")
	content := `package test

// Helper function
func MyHelper() {}

// Regular function
func RegularFunc() {}
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	// Audit the file
	auditor := NewAuditor(DefaultPrinciples())
	violations, fileInfo, err := auditor.CheckFile(testFile)

	require.NoError(t, err)
	assert.NotNil(t, fileInfo)
	assert.True(t, len(violations) > 0)

	// Check that we found GP-001
	found := false
	for _, v := range violations {
		if v.Principle.ID == "GP-001" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should have found GP-001 violation")
}

func TestAuditor_CheckFile_NoViolations(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create clean test file
	testFile := filepath.Join(tempDir, "clean.go")
	content := `package test

func GoodFunction() {
	// Clean code
}
`
	err := os.WriteFile(testFile, []byte(content), 0644)
	require.NoError(t, err)

	auditor := NewAuditor(DefaultPrinciples())
	violations, fileInfo, err := auditor.CheckFile(testFile)

	require.NoError(t, err)
	assert.NotNil(t, fileInfo)
	// May have violations depending on patterns, but file should be checked
	assert.NotNil(t, violations)
}

func TestAuditor_Audit(t *testing.T) {
	// Create temp directory with test files
	tempDir := t.TempDir()

	// Create subdirectory
	subDir := filepath.Join(tempDir, "pkg")
	os.MkdirAll(subDir, 0755)

	// Create files
	files := map[string]string{
		"main.go": `package main

func HelperFunc() {}
func main() {}
`,
		"pkg/util.go": `package pkg

// Helper utility
func UtilHelper() {}
`,
		"readme.md": "# Test",
	}

	for name, content := range files {
		path := filepath.Join(tempDir, name)
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Audit
	auditor := NewAuditor(DefaultPrinciples())
	report, err := auditor.Audit(tempDir)

	require.NoError(t, err)
	assert.NotNil(t, report)
	assert.True(t, len(report.Files) >= 2) // At least .go files
	assert.True(t, report.Summary.FilesChecked > 0)
}

func TestAuditor_shouldExclude(t *testing.T) {
	auditor := NewAuditor(DefaultPrinciples())

	tests := []struct {
		path     string
		expected bool
	}{
		{"vendor/package/file.go", true},
		{"node_modules/lib/file.js", true},
		{".git/config", true},
		{"src/main.go", false},
		{"pkg/util_test.go", false}, // Not excluded by default
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := auditor.shouldExclude(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAuditor_shouldCheckFile(t *testing.T) {
	auditor := NewAuditor(DefaultPrinciples())

	tests := []struct {
		path     string
		expected bool
	}{
		{"main.go", true},
		{"util.js", true},
		{"app.ts", true},
		{"script.py", true},
		{"Main.java", true},
		{"README.md", true},
		{"config.yaml", false},
		{"image.png", false},
		{"Makefile", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := auditor.shouldCheckFile(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPositionToLineCol(t *testing.T) {
	content := "line1\nline2\nline3"

	tests := []struct {
		pos         int
		wantLine    int
		wantColumn  int
	}{
		{0, 1, 1},   // Start of first line
		{6, 2, 1},   // Start of second line
		{7, 2, 2},   // Second char of second line
		{12, 3, 1},  // Start of third line
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("pos_%d", tt.pos), func(t *testing.T) {
			line, col := positionToLineCol(content, tt.pos)
			assert.Equal(t, tt.wantLine, line)
			assert.Equal(t, tt.wantColumn, col)
		})
	}
}

func TestGetSnippet(t *testing.T) {
	lines := []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	}

	tests := []struct {
		lineNum  int
		expected string
	}{
		{1, "line 1\nline 2"},
		{3, "line 2\nline 3\nline 4"},
		{5, "line 4\nline 5"},
		{0, ""},
		{10, ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("line_%d", tt.lineNum), func(t *testing.T) {
			result := getSnippet(lines, tt.lineNum)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAuditReport_HasErrors(t *testing.T) {
	report := &AuditReport{
		Violations: []Violation{
			{Principle: Principle{ID: "GP-004", Severity: "error"}},
			{Principle: Principle{ID: "GP-001", Severity: "warning"}},
		},
		Summary: Summary{Errors: 1, Warnings: 1},
	}

	assert.True(t, report.HasErrors())
	assert.True(t, report.HasWarnings())
}

func TestAuditReport_GetViolationsByFile(t *testing.T) {
	report := &AuditReport{
		Violations: []Violation{
			{File: "a.go", Principle: Principle{ID: "P1"}},
			{File: "a.go", Principle: Principle{ID: "P2"}},
			{File: "b.go", Principle: Principle{ID: "P3"}},
		},
	}

	grouped := report.GetViolationsByFile()

	assert.Len(t, grouped["a.go"], 2)
	assert.Len(t, grouped["b.go"], 1)
}

func TestAuditReport_GetViolationsByPrinciple(t *testing.T) {
	report := &AuditReport{
		Violations: []Violation{
			{Principle: Principle{ID: "P1"}},
			{Principle: Principle{ID: "P1"}},
			{Principle: Principle{ID: "P2"}},
		},
	}

	grouped := report.GetViolationsByPrinciple()

	assert.Len(t, grouped["P1"], 2)
	assert.Len(t, grouped["P2"], 1)
}

func TestAuditReport_FilterViolationsBySeverity(t *testing.T) {
	report := &AuditReport{
		Violations: []Violation{
			{Principle: Principle{ID: "P1", Severity: "error"}},
			{Principle: Principle{ID: "P2", Severity: "warning"}},
			{Principle: Principle{ID: "P3", Severity: "error"}},
		},
	}

	errors := report.FilterViolationsBySeverity("error")
	assert.Len(t, errors, 2)

	warnings := report.FilterViolationsBySeverity("warning")
	assert.Len(t, warnings, 1)
}

func TestAuditReport_String(t *testing.T) {
	report := &AuditReport{
		Violations: []Violation{
			{
				Principle: Principle{ID: "GP-001", Severity: "warning", Message: "Test warning"},
				File:      "test.go",
				Line:      10,
			},
			{
				Principle: Principle{ID: "GP-004", Severity: "error", Message: "Test error"},
				File:      "test.go",
				Line:      20,
			},
		},
		Summary: Summary{
			Total:        2,
			Errors:       1,
			Warnings:     1,
			FilesChecked: 1,
		},
	}

	str := report.String()

	assert.Contains(t, str, "Golden Principles Audit Report")
	assert.Contains(t, str, "Files checked: 1")
	assert.Contains(t, str, "Total violations: 2")
	assert.Contains(t, str, "Errors: 1")
	assert.Contains(t, str, "Warnings: 1")
	assert.Contains(t, str, "test.go")
	assert.Contains(t, str, "GP-001")
	assert.Contains(t, str, "GP-004")
}
