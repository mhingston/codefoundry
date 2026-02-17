package review

import (
	"testing"
)

func TestIsValidSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"P1", true},
		{"P2", true},
		{"P3", true},
		{"p1", false}, // case sensitive
		{"P0", false},
		{"P4", false},
		{"critical", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidSeverity(tt.input)
			if got != tt.expected {
				t.Errorf("IsValidSeverity(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSeverityClassifier_Classify(t *testing.T) {
	classifier := NewSeverityClassifier()

	tests := []struct {
		name     string
		finding  Finding
		expected Severity
	}{
		{
			name: "P1 - security vulnerability",
			finding: Finding{
				Category: "security",
				Message:  "SQL injection vulnerability found",
			},
			expected: SeverityP1,
		},
		{
			name: "P1 - data loss keyword",
			finding: Finding{
				Category: "bug",
				Message:  "This could cause data loss",
			},
			expected: SeverityP1,
		},
		{
			name: "P1 - race condition",
			finding: Finding{
				Category: "concurrency",
				Message:  "Potential race condition detected",
			},
			expected: SeverityP1,
		},
		{
			name: "P2 - performance issue",
			finding: Finding{
				Category: "performance",
				Message:  "This loop has O(n^2) complexity",
			},
			expected: SeverityP2,
		},
		{
			name: "P2 - missing test",
			finding: Finding{
				Category: "testing",
				Message:  "Missing test coverage for edge case",
			},
			expected: SeverityP2,
		},
		{
			name: "P2 - technical debt",
			finding: Finding{
				Category: "maintainability",
				Message:  "Technical debt: duplicate code",
			},
			expected: SeverityP2,
		},
		{
			name: "P3 - style issue",
			finding: Finding{
				Category: "style",
				Message:  "Inconsistent indentation",
			},
			expected: SeverityP3,
		},
		{
			name: "P3 - documentation",
			finding: Finding{
				Category: "documentation",
				Message:  "Missing docstring",
			},
			expected: SeverityP3,
		},
		{
			name: "P3 - default for unknown",
			finding: Finding{
				Category: "other",
				Message:  "Some minor observation",
			},
			expected: SeverityP3,
		},
		{
			name: "P1 - hardcoded password",
			finding: Finding{
				Category: "security",
				Message:  "Hardcoded password detected in code",
			},
			expected: SeverityP1,
		},
		{
			name: "P1 - null pointer",
			finding: Finding{
				Category: "bug",
				Message:  "Potential null pointer dereference",
			},
			expected: SeverityP1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifier.Classify(tt.finding)
			if got != tt.expected {
				t.Errorf("Classify() = %v, want %v for message: %s", got, tt.expected, tt.finding.Message)
			}
		})
	}
}

func TestCountFindingsBySeverity(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityP1, ID: "1"},
		{Severity: SeverityP1, ID: "2"},
		{Severity: SeverityP2, ID: "3"},
		{Severity: SeverityP2, ID: "4"},
		{Severity: SeverityP2, ID: "5"},
		{Severity: SeverityP3, ID: "6"},
		{Severity: SeverityP3, ID: "7"},
		{Severity: SeverityP3, ID: "8"},
		{Severity: SeverityP3, ID: "9"},
	}

	p1, p2, p3 := CountFindingsBySeverity(findings)

	if p1 != 2 {
		t.Errorf("P1 count = %d, want 2", p1)
	}
	if p2 != 3 {
		t.Errorf("P2 count = %d, want 3", p2)
	}
	if p3 != 4 {
		t.Errorf("P3 count = %d, want 4", p3)
	}
}

func TestFilterFindingsBySeverity(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityP1, ID: "1"},
		{Severity: SeverityP2, ID: "2"},
		{Severity: SeverityP1, ID: "3"},
		{Severity: SeverityP3, ID: "4"},
	}

	p1Findings := FilterFindingsBySeverity(findings, SeverityP1)
	if len(p1Findings) != 2 {
		t.Errorf("Filtered P1 count = %d, want 2", len(p1Findings))
	}

	p2Findings := FilterFindingsBySeverity(findings, SeverityP2)
	if len(p2Findings) != 1 {
		t.Errorf("Filtered P2 count = %d, want 1", len(p2Findings))
	}

	p3Findings := FilterFindingsBySeverity(findings, SeverityP3)
	if len(p3Findings) != 1 {
		t.Errorf("Filtered P3 count = %d, want 1", len(p3Findings))
	}
}

func TestHasP1Findings(t *testing.T) {
	withP1 := []Finding{
		{Severity: SeverityP2, ID: "1"},
		{Severity: SeverityP1, ID: "2"},
	}

	withoutP1 := []Finding{
		{Severity: SeverityP2, ID: "1"},
		{Severity: SeverityP3, ID: "2"},
	}

	empty := []Finding{}

	if !HasP1Findings(withP1) {
		t.Error("HasP1Findings(withP1) = false, want true")
	}

	if HasP1Findings(withoutP1) {
		t.Error("HasP1Findings(withoutP1) = true, want false")
	}

	if HasP1Findings(empty) {
		t.Error("HasP1Findings(empty) = true, want false")
	}
}

func TestCategoryMatcher(t *testing.T) {
	matcher := CategoryMatcher{Categories: []string{"security", "vulnerability"}}

	tests := []struct {
		category string
		expected bool
	}{
		{"security", true},
		{"SECURITY", true}, // case insensitive
		{"vulnerability", true},
		{"performance", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			finding := Finding{Category: tt.category}
			got := matcher.Match(finding)
			if got != tt.expected {
				t.Errorf("Match(%q) = %v, want %v", tt.category, got, tt.expected)
			}
		})
	}
}

func TestKeywordMatcher(t *testing.T) {
	matcher := KeywordMatcher{Keywords: []string{"error", "failure"}}

	tests := []struct {
		message  string
		expected bool
	}{
		{"An error occurred", true},
		{"System failure detected", true},
		{"ERROR in processing", true}, // case insensitive
		{"Success", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			finding := Finding{Message: tt.message}
			got := matcher.Match(finding)
			if got != tt.expected {
				t.Errorf("Match(%q) = %v, want %v", tt.message, got, tt.expected)
			}
		})
	}
}

func TestFilePatternMatcher(t *testing.T) {
	matcher := FilePatternMatcher{Extensions: []string{".go", "_test.go"}}

	tests := []struct {
		file     string
		expected bool
	}{
		{"main.go", true},
		{"pkg_test.go", true},
		{"test.go", true},
		{"main.py", false},
		{"", false},
		{"go", false},
		{"file.go.txt", false},
		{"/path/to/file.go", true},
		{"/path/to/pkg_test.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			finding := Finding{File: tt.file}
			got := matcher.Match(finding)
			if got != tt.expected {
				t.Errorf("Match(%q) = %v, want %v", tt.file, got, tt.expected)
			}
		})
	}
}

func TestSeverityClassifier_Classify_EdgeCases(t *testing.T) {
	classifier := NewSeverityClassifier()

	tests := []struct {
		name     string
		finding  Finding
		expected Severity
	}{
		{
			name: "P1 - case insensitive security match",
			finding: Finding{
				Category: "SECURITY",
				Message:  "test",
			},
			expected: SeverityP1,
		},
		{
			name: "P1 - SQL injection in lowercase",
			finding: Finding{
				Category: "other",
				Message:  "potential sql injection found",
			},
			expected: SeverityP1,
		},
		{
			name: "P1 - panic in message",
			finding: Finding{
				Category: "bug",
				Message:  "This function may panic",
			},
			expected: SeverityP1,
		},
		{
			name: "P1 - CVE category",
			finding: Finding{
				Category: "cve",
				Message:  "CVE-2024-1234",
			},
			expected: SeverityP1,
		},
		{
			name: "P2 - missing-test category",
			finding: Finding{
				Category: "missing-test",
				Message:  "No tests for this function",
			},
			expected: SeverityP2,
		},
		{
			name: "P2 - ignored error",
			finding: Finding{
				Category: "error-handling",
				Message:  "ignored error in function",
			},
			expected: SeverityP2,
		},
		{
			name: "P2 - cyclomatic complexity",
			finding: Finding{
				Category: "maintainability",
				Message:  "Cyclomatic complexity too high",
			},
			expected: SeverityP2,
		},
		{
			name: "P3 - formatting",
			finding: Finding{
				Category: "formatting",
				Message:  "Inconsistent formatting",
			},
			expected: SeverityP3,
		},
		{
			name: "P3 - whitespace",
			finding: Finding{
				Category: "style",
				Message:  "Trailing whitespace",
			},
			expected: SeverityP3,
		},
		{
			name: "P3 - unused import",
			finding: Finding{
				Category: "style",
				Message:  "unused import found",
			},
			expected: SeverityP3,
		},
		{
			name: "P3 - nitpick",
			finding: Finding{
				Category: "other",
				Message:  "This is just a nitpick",
			},
			expected: SeverityP3,
		},
		{
			name: "P3 - suggestion",
			finding: Finding{
				Category: "other",
				Message:  "Minor suggestion",
			},
			expected: SeverityP3,
		},
		{
			name: "P3 - empty category and message",
			finding: Finding{
				Category: "",
				Message:  "",
			},
			expected: SeverityP3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifier.Classify(tt.finding)
			if got != tt.expected {
				t.Errorf("Classify() = %v, want %v for category=%q message=%q",
					got, tt.expected, tt.finding.Category, tt.finding.Message)
			}
		})
	}
}

func TestSeverityClassifier_Classify_PriorityOrder(t *testing.T) {
	// Create classifier with custom rules to test priority
	classifier := &SeverityClassifier{
		Rules: []ClassificationRule{
			{
				Severity: SeverityP1,
				Priority: 100,
				Matchers: []PatternMatcher{
					CategoryMatcher{Categories: []string{"critical"}},
				},
			},
			{
				Severity: SeverityP2,
				Priority: 50,
				Matchers: []PatternMatcher{
					CategoryMatcher{Categories: []string{"warning"}},
				},
			},
			{
				Severity: SeverityP3,
				Priority: 10,
				Matchers: []PatternMatcher{
					KeywordMatcher{Keywords: []string{"suggestion"}},
				},
			},
		},
	}

	// Test that P1 takes priority even with P3 keyword
	t.Run("priority over keyword", func(t *testing.T) {
		finding := Finding{
			Category: "critical",
			Message:  "This is a critical suggestion",
		}
		got := classifier.Classify(finding)
		if got != SeverityP1 {
			t.Errorf("Classify() = %v, want %v (priority should win over keyword)", got, SeverityP1)
		}
	})

	// Test that P2 takes priority over P3
	t.Run("P2 over P3", func(t *testing.T) {
		finding := Finding{
			Category: "warning",
			Message:  "This is a warning with suggestion",
		}
		got := classifier.Classify(finding)
		if got != SeverityP2 {
			t.Errorf("Classify() = %v, want %v", got, SeverityP2)
		}
	})
}

func TestCountFindingsBySeverity_Empty(t *testing.T) {
	// Test with empty findings
	p1, p2, p3 := CountFindingsBySeverity([]Finding{})
	if p1 != 0 || p2 != 0 || p3 != 0 {
		t.Errorf("CountFindingsBySeverity(empty) = (%d, %d, %d), want (0, 0, 0)", p1, p2, p3)
	}

	// Test with nil (should also work)
	p1, p2, p3 = CountFindingsBySeverity(nil)
	if p1 != 0 || p2 != 0 || p3 != 0 {
		t.Errorf("CountFindingsBySeverity(nil) = (%d, %d, %d), want (0, 0, 0)", p1, p2, p3)
	}
}

func TestFilterFindingsBySeverity_NoMatches(t *testing.T) {
	findings := []Finding{
		{Severity: SeverityP1, ID: "1"},
		{Severity: SeverityP2, ID: "2"},
	}

	// Test with no matches
	filtered := FilterFindingsBySeverity(findings, SeverityP3)
	if len(filtered) != 0 {
		t.Errorf("FilterFindingsBySeverity(P3) = %d, want 0", len(filtered))
	}

	// Test with empty slice
	filtered = FilterFindingsBySeverity([]Finding{}, SeverityP1)
	if len(filtered) != 0 {
		t.Errorf("FilterFindingsBySeverity(empty) = %d, want 0", len(filtered))
	}
}
