package review

import (
	"strings"
)

// Severity represents the severity level of a finding
type Severity string

const (
	SeverityP1 Severity = "P1" // Must fix - blocking
	SeverityP2 Severity = "P2" // Should fix - important
	SeverityP3 Severity = "P3" // Nice to fix - minor
)

// IsValidSeverity checks if a string is a valid severity
func IsValidSeverity(s string) bool {
	switch Severity(s) {
	case SeverityP1, SeverityP2, SeverityP3:
		return true
	default:
		return false
	}
}

// Finding represents a single review finding
type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	File     string   `json:"file"`
	Line     int      `json:"line,omitempty"`
	Column   int      `json:"column,omitempty"`
	Message  string   `json:"message"`
	Category string   `json:"category"`
	Code     string   `json:"code,omitempty"`      // Snippet of code
	Suggestion string `json:"suggestion,omitempty"` // Suggested fix
}

// ClassificationRule defines a rule for classifying findings
type ClassificationRule struct {
	Severity Severity
	Matchers []PatternMatcher
	Priority int // Higher priority rules evaluated first
}

// PatternMatcher matches against finding properties
type PatternMatcher interface {
	Match(finding Finding) bool
}

// CategoryMatcher matches by category
type CategoryMatcher struct {
	Categories []string
}

func (m CategoryMatcher) Match(finding Finding) bool {
	for _, cat := range m.Categories {
		if strings.EqualFold(finding.Category, cat) {
			return true
		}
	}
	return false
}

// KeywordMatcher matches by keyword in message
type KeywordMatcher struct {
	Keywords []string
}

func (m KeywordMatcher) Match(finding Finding) bool {
	msgLower := strings.ToLower(finding.Message)
	for _, kw := range m.Keywords {
		if strings.Contains(msgLower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// FilePatternMatcher matches by file pattern
type FilePatternMatcher struct {
	Extensions []string
}

func (m FilePatternMatcher) Match(finding Finding) bool {
	for _, ext := range m.Extensions {
		if strings.HasSuffix(finding.File, ext) {
			return true
		}
	}
	return false
}

// SeverityClassifier classifies findings by severity
type SeverityClassifier struct {
	Rules []ClassificationRule
}

// NewSeverityClassifier creates a classifier with default rules
func NewSeverityClassifier() *SeverityClassifier {
	return &SeverityClassifier{
		Rules: DefaultClassificationRules(),
	}
}

// Classify classifies a finding into P1/P2/P3
func (c *SeverityClassifier) Classify(finding Finding) Severity {
	// Check rules in priority order
	rules := make([]ClassificationRule, len(c.Rules))
	copy(rules, c.Rules)

	// Simple bubble sort by priority (highest first)
	for i := 0; i < len(rules)-1; i++ {
		for j := 0; j < len(rules)-i-1; j++ {
			if rules[j].Priority < rules[j+1].Priority {
				rules[j], rules[j+1] = rules[j+1], rules[j]
			}
		}
	}

	// Find first matching rule
	for _, rule := range rules {
		for _, matcher := range rule.Matchers {
			if matcher.Match(finding) {
				return rule.Severity
			}
		}
	}

	// Default to P3 if no rule matches
	return SeverityP3
}

// DefaultClassificationRules returns default P1/P2/P3 classification rules
func DefaultClassificationRules() []ClassificationRule {
	return []ClassificationRule{
		// P1 Rules - Must fix (blocking)
		{
			Severity: SeverityP1,
			Priority: 100,
			Matchers: []PatternMatcher{
				CategoryMatcher{Categories: []string{"security", "vulnerability", "cve", "exploit", "injection", "xss", "sql-injection"}},
				KeywordMatcher{Keywords: []string{
					"data loss", "crash", "panic", "segfault", "null pointer",
					"race condition", "deadlock", "data race", "memory leak",
					"infinite loop", "stack overflow", "buffer overflow",
					"hardcoded password", "secret exposed", "credential leak",
					"sql injection", "command injection", "path traversal",
					"unauthorized", "authentication bypass", "privilege escalation",
				}},
			},
		},

		// P2 Rules - Should fix (important)
		{
			Severity: SeverityP2,
			Priority: 50,
			Matchers: []PatternMatcher{
				CategoryMatcher{Categories: []string{"performance", "optimization", "missing-test", "test-coverage"}},
				KeywordMatcher{Keywords: []string{
					"performance", "slow", "inefficient", "bottleneck",
					"missing test", "no test", "low coverage", "untested",
					"maintainability", "technical debt", "refactor",
					"complexity", "cyclomatic", "nesting too deep",
					"duplicate code", "copy-paste", "dead code",
					"error handling", "missing error check", "ignored error",
				}},
			},
		},

		// P3 Rules - Nice to fix (minor)
		{
			Severity: SeverityP3,
			Priority: 10,
			Matchers: []PatternMatcher{
				CategoryMatcher{Categories: []string{"style", "formatting", "documentation", "naming"}},
				KeywordMatcher{Keywords: []string{
					"format", "indentation", "whitespace", "trailing space",
					"naming convention", "variable name", "function name",
					"comment", "documentation", "docstring",
					"minor", "nitpick", "suggestion",
					"unused import", "unused variable", "unused parameter",
				}},
			},
		},
	}
}

// CountFindingsBySeverity counts findings by severity level
func CountFindingsBySeverity(findings []Finding) (p1, p2, p3 int) {
	for _, f := range findings {
		switch f.Severity {
		case SeverityP1:
			p1++
		case SeverityP2:
			p2++
		case SeverityP3:
			p3++
		}
	}
	return
}

// FilterFindingsBySeverity returns findings matching a severity
func FilterFindingsBySeverity(findings []Finding, severity Severity) []Finding {
	var filtered []Finding
	for _, f := range findings {
		if f.Severity == severity {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// HasP1Findings checks if there are any P1 findings
func HasP1Findings(findings []Finding) bool {
	for _, f := range findings {
		if f.Severity == SeverityP1 {
			return true
		}
	}
	return false
}
