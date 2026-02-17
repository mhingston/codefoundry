package golden

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPrinciples(t *testing.T) {
	principles := DefaultPrinciples()

	assert.True(t, len(principles) > 0)

	// Check that all principles have required fields
	for _, p := range principles {
		assert.NotEmpty(t, p.ID, "Principle ID should not be empty")
		assert.NotEmpty(t, p.Name, "Principle Name should not be empty")
		assert.NotEmpty(t, p.Pattern, "Principle Pattern should not be empty")
		assert.NotEmpty(t, p.Severity, "Principle Severity should not be empty")
		assert.True(t, p.IsError() || p.IsWarning(), "Principle should be either error or warning")
		assert.NotNil(t, p.regex, "Principle regex should be compiled")
	}
}

func TestPrinciple_IsError(t *testing.T) {
	tests := []struct {
		severity string
		expected bool
	}{
		{"error", true},
		{"warning", false},
		{"info", false},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			p := Principle{Severity: tt.severity}
			assert.Equal(t, tt.expected, p.IsError())
		})
	}
}

func TestPrinciple_IsWarning(t *testing.T) {
	tests := []struct {
		severity string
		expected bool
	}{
		{"error", false},
		{"warning", true},
		{"info", false},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			p := Principle{Severity: tt.severity}
			assert.Equal(t, tt.expected, p.IsWarning())
		})
	}
}

func TestPrinciple_Match(t *testing.T) {
	// Create a test principle
	p := Principle{
		ID:      "TEST-001",
		Pattern: `func\s+\w+Helper\(`,
	}
	p.regex = compileRegex(t, p.Pattern)

	tests := []struct {
		content  string
		expected bool
	}{
		{"func MyHelper() {}", true},
		{"func HelperFunc() {}", false},
		{"// Helper comment", false},
	}

	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			result := p.Match(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPrinciple_FindMatches(t *testing.T) {
	p := Principle{
		ID:      "TEST-002",
		Pattern: `func\s+\w+`,
	}
	p.regex = compileRegex(t, p.Pattern)

	content := `func Foo() {}
func Bar() {}
var x int`

	matches := p.FindMatches(content)

	assert.Len(t, matches, 2)
	assert.Contains(t, matches, "func Foo")
	assert.Contains(t, matches, "func Bar")
}

func TestPrinciple_FindAllMatches(t *testing.T) {
	p := Principle{
		ID:      "TEST-003",
		Pattern: `func\s+\w+`,
	}
	p.regex = compileRegex(t, p.Pattern)

	content := `func Foo() {}
func Bar() {}`

	matches := p.FindAllMatches(content)

	assert.Len(t, matches, 2)
	assert.Equal(t, "func Foo", matches[0].Text)
	assert.Equal(t, 0, matches[0].Start)
	assert.Equal(t, 8, matches[0].End)
}

func TestGetPrincipleByID(t *testing.T) {
	principles := []Principle{
		{ID: "GP-001", Name: "Test 1"},
		{ID: "GP-002", Name: "Test 2"},
		{ID: "GP-003", Name: "Test 3"},
	}

	tests := []struct {
		id       string
		expected string
	}{
		{"GP-001", "Test 1"},
		{"GP-002", "Test 2"},
		{"GP-999", ""},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			result := GetPrincipleByID(tt.id, principles)
			if tt.expected == "" {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected, result.Name)
			}
		})
	}
}

func TestFilterPrinciplesByCategory(t *testing.T) {
	principles := []Principle{
		{ID: "GP-001", Category: "security"},
		{ID: "GP-002", Category: "code-quality"},
		{ID: "GP-003", Category: "security"},
	}

	filtered := FilterPrinciplesByCategory("security", principles)

	assert.Len(t, filtered, 2)
	assert.Equal(t, "GP-001", filtered[0].ID)
	assert.Equal(t, "GP-003", filtered[1].ID)
}

func TestFilterPrinciplesByCategory_Empty(t *testing.T) {
	principles := []Principle{
		{ID: "GP-001", Category: "security"},
		{ID: "GP-002", Category: "code-quality"},
	}

	filtered := FilterPrinciplesByCategory("", principles)

	assert.Len(t, filtered, 2)
}

func TestFilterPrinciplesBySeverity(t *testing.T) {
	principles := []Principle{
		{ID: "GP-001", Severity: "error"},
		{ID: "GP-002", Severity: "warning"},
		{ID: "GP-003", Severity: "error"},
	}

	filtered := FilterPrinciplesBySeverity("error", principles)

	assert.Len(t, filtered, 2)
}

func TestCategories(t *testing.T) {
	principles := []Principle{
		{ID: "GP-001", Category: "security"},
		{ID: "GP-002", Category: "code-quality"},
		{ID: "GP-003", Category: "security"},
		{ID: "GP-004", Category: ""},
	}

	categories := Categories(principles)

	assert.Len(t, categories, 2)
	assert.Contains(t, categories, "security")
	assert.Contains(t, categories, "code-quality")
}

func TestValidatePrinciple(t *testing.T) {
	tests := []struct {
		name    string
		p       Principle
		wantErr bool
	}{
		{
			name: "valid",
			p: Principle{
				ID:       "GP-001",
				Name:     "Test",
				Severity: "warning",
				Pattern:  `func\s+\w+`,
			},
			wantErr: false,
		},
		{
			name: "missing id",
			p: Principle{
				Name:     "Test",
				Severity: "warning",
				Pattern:  `func\s+\w+`,
			},
			wantErr: true,
		},
		{
			name: "missing name",
			p: Principle{
				ID:       "GP-001",
				Severity: "warning",
				Pattern:  `func\s+\w+`,
			},
			wantErr: true,
		},
		{
			name: "invalid severity",
			p: Principle{
				ID:       "GP-001",
				Name:     "Test",
				Severity: "invalid",
				Pattern:  `func\s+\w+`,
			},
			wantErr: true,
		},
		{
			name: "missing pattern",
			p: Principle{
				ID:       "GP-001",
				Name:     "Test",
				Severity: "warning",
			},
			wantErr: true,
		},
		{
			name: "invalid pattern",
			p: Principle{
				ID:       "GP-001",
				Name:     "Test",
				Severity: "warning",
				Pattern:  `[invalid`,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePrinciple(tt.p)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "ID",
		Message: "ID is required",
	}

	assert.Equal(t, "ID: ID is required", err.Error())
}

// Helper function
func compileRegex(t *testing.T, pattern string) *regexp.Regexp {
	r, err := regexp.Compile(pattern)
	require.NoError(t, err)
	return r
}
