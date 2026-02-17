package golden

import (
	"regexp"
)

// Principle represents a golden principle to enforce
type Principle struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Severity string   `json:"severity"` // warning, error
	Pattern  string   `json:"pattern"`  // regex pattern
	Message  string   `json:"message"`
	Category string   `json:"category"`
	regex    *regexp.Regexp
}

// DefaultPrinciples returns the default set of golden principles
func DefaultPrinciples() []Principle {
	principles := []Principle{
		{
			ID:       "GP-001",
			Name:     "Prefer shared utilities over hand-rolled helpers",
			Severity: "warning",
			Pattern:  `func\s+\w*[Hh]elper\w*\s*\(`,
			Message:  "Consider using shared utilities instead of hand-rolled helpers",
			Category: "code-quality",
		},
		{
			ID:       "GP-002",
			Name:     "Solution docs must link affected files and tests",
			Severity: "warning",
			Pattern:  `(?i)(solution|design).*\.md`,
			Message:  "Solution documents should reference affected files and tests",
			Category: "documentation",
		},
		{
			ID:       "GP-003",
			Name:     "Validate all API boundary contracts",
			Severity: "error",
			Pattern:  `^\s*(func|type\s+\w+).*\{`,
			Message:  "Ensure API contracts are validated with tests",
			Category: "testing",
		},
		{
			ID:       "GP-004",
			Name:     "No hardcoded secrets or credentials",
			Severity: "error",
			Pattern:  `(?i)(password|secret|token|key)\s*[:=]\s*["\']\w+["\']`,
			Message:  "Hardcoded secrets detected - use environment variables or secret management",
			Category: "security",
		},
		{
			ID:       "GP-005",
			Name:     "Error handling must not swallow errors",
			Severity: "error",
			Pattern:  `_\s*=\s*\w+\(`,
			Message:  "Discarding error return values with '_' - handle errors properly",
			Category: "error-handling",
		},
		{
			ID:       "GP-006",
			Name:     "Context should be first parameter",
			Severity: "warning",
			Pattern:  `func\s*\([^)]*context\.Context[^)]*\)`,
			Message:  "Context should typically be the first parameter in function signatures",
			Category: "code-style",
		},
		{
			ID:       "GP-007",
			Name:     "Avoid global variables",
			Severity: "warning",
			Pattern:  `^\s*var\s+\w+\s+\w+`,
			Message:  "Global variables should be avoided - use dependency injection instead",
			Category: "code-quality",
		},
		{
			ID:       "GP-008",
			Name:     "Test coverage for critical paths",
			Severity: "error",
			Pattern:  `func\s+\w+.*\(.*error.*\)`,
			Message:  "Functions returning errors should have test coverage for error paths",
			Category: "testing",
		},
		{
			ID:       "GP-009",
			Name:     "Consistent naming conventions",
			Severity: "warning",
			Pattern:  `(?i)(get|fetch|retrieve|obtain)\w*`,
			Message:  "Use consistent naming for similar operations (choose one pattern)",
			Category: "code-style",
		},
		{
			ID:       "GP-010",
			Name:     "Documentation for exported symbols",
			Severity: "warning",
			Pattern:  `^\s*(func|type|var|const)\s+[A-Z]\w+`,
			Message:  "Exported symbols should have documentation comments",
			Category: "documentation",
		},
	}

	// Compile regex patterns
	for i := range principles {
		if principles[i].Pattern != "" {
			principles[i].regex = regexp.MustCompile(principles[i].Pattern)
		}
	}

	return principles
}

// IsError returns true if the principle is an error severity
func (p Principle) IsError() bool {
	return p.Severity == "error"
}

// IsWarning returns true if the principle is a warning severity
func (p Principle) IsWarning() bool {
	return p.Severity == "warning"
}

// Match checks if the content matches the principle pattern
func (p Principle) Match(content string) bool {
	if p.regex == nil {
		return false
	}
	return p.regex.MatchString(content)
}

// FindMatches returns all matches in the content
func (p Principle) FindMatches(content string) []string {
	if p.regex == nil {
		return nil
	}
	return p.regex.FindAllString(content, -1)
}

// FindAllMatches returns all matches with their positions
func (p Principle) FindAllMatches(content string) []Match {
	if p.regex == nil {
		return nil
	}

	matches := make([]Match, 0)
	for _, loc := range p.regex.FindAllStringIndex(content, -1) {
		matches = append(matches, Match{
			Text:  content[loc[0]:loc[1]],
			Start: loc[0],
			End:   loc[1],
		})
	}

	return matches
}

// Match represents a pattern match
type Match struct {
	Text  string
	Start int
	End   int
}

// GetPrincipleByID returns a principle by ID
func GetPrincipleByID(id string, principles []Principle) *Principle {
	for i := range principles {
		if principles[i].ID == id {
			return &principles[i]
		}
	}
	return nil
}

// FilterPrinciplesByCategory returns principles filtered by category
func FilterPrinciplesByCategory(category string, principles []Principle) []Principle {
	if category == "" {
		return principles
	}

	filtered := make([]Principle, 0)
	for _, p := range principles {
		if p.Category == category {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// FilterPrinciplesBySeverity returns principles filtered by severity
func FilterPrinciplesBySeverity(severity string, principles []Principle) []Principle {
	if severity == "" {
		return principles
	}

	filtered := make([]Principle, 0)
	for _, p := range principles {
		if p.Severity == severity {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// Categories returns all unique categories
func Categories(principles []Principle) []string {
	seen := make(map[string]bool)
	categories := make([]string, 0)

	for _, p := range principles {
		if !seen[p.Category] && p.Category != "" {
			seen[p.Category] = true
			categories = append(categories, p.Category)
		}
	}

	return categories
}

// ValidatePrinciple validates a principle definition
func ValidatePrinciple(p Principle) error {
	if p.ID == "" {
		return &ValidationError{Field: "ID", Message: "ID is required"}
	}

	if p.Name == "" {
		return &ValidationError{Field: "Name", Message: "Name is required"}
	}

	if p.Severity != "error" && p.Severity != "warning" {
		return &ValidationError{Field: "Severity", Message: "Severity must be 'error' or 'warning'"}
	}

	if p.Pattern == "" {
		return &ValidationError{Field: "Pattern", Message: "Pattern is required"}
	}

	// Try to compile the pattern
	_, err := regexp.Compile(p.Pattern)
	if err != nil {
		return &ValidationError{Field: "Pattern", Message: "Invalid regex pattern: " + err.Error()}
	}

	return nil
}

// ValidationError represents a principle validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
