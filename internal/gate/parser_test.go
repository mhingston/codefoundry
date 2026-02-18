package gate

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewParser(t *testing.T) {
	parser := NewParser()
	assert.NotNil(t, parser)
	assert.NotNil(t, parser.patterns)
}

func TestParser_ParseJSON(t *testing.T) {
	parser := NewParser()

	// Valid JSON array
	json := `[{"message": "error 1"}, {"message": "error 2"}]`
	failures, err := parser.ParseJSON(json)
	require.NoError(t, err)
	assert.Len(t, failures, 2)
	assert.Equal(t, "error 1", failures[0].Message)

	// Valid JSON object
	json = `{"message": "single error"}`
	failures, err = parser.ParseJSON(json)
	require.NoError(t, err)
	assert.Len(t, failures, 1)
	assert.Equal(t, "single error", failures[0].Message)

	// Invalid JSON
	json = "not json"
	_, err = parser.ParseJSON(json)
	assert.Error(t, err)
}

func TestParser_ParseJSON_EmptyMessage(t *testing.T) {
	parser := NewParser()

	// JSON with empty message should not be parsed as failure
	json := `{"message": ""}`
	_, err := parser.ParseJSON(json)
	assert.Error(t, err)
}

func TestParser_ParseOutput_JSON(t *testing.T) {
	parser := NewParser()

	json := `[{"message": "error 1"}]`
	failures, err := parser.ParseOutput("any-gate", json, "")
	require.NoError(t, err)
	assert.Len(t, failures, 1)
}

func TestParser_ParseGoVetOutput(t *testing.T) {
	parser := NewParser()

	stdout := `main.go:42:15: undefined: someFunction
utils.go:10:5: exported function SomeFunc should have comment or be unexported`

	failures := parser.parseGoVetOutput(stdout, "")
	require.Len(t, failures, 2)

	assert.Equal(t, "main.go", failures[0].File)
	assert.Equal(t, 42, failures[0].Line)
	assert.Equal(t, "undefined: someFunction", failures[0].Message)
	assert.Equal(t, "error", failures[0].Severity)

	assert.Equal(t, "utils.go", failures[1].File)
	assert.Equal(t, 10, failures[1].Line)
}

func TestParser_ParseGoTestOutput(t *testing.T) {
	parser := NewParser()

	stdout := `--- FAIL: TestSomething (0.00s)
    something_test.go:15: expected true, got false
PASS
ok  	github.com/test	0.001s`

	failures := parser.parseGoTestOutput(stdout, "")

	// Should find the FAIL line and assertion failure
	foundFailLine := false
	for _, f := range failures {
		if f.Message == "--- FAIL: TestSomething (0.00s)" {
			foundFailLine = true
			break
		}
	}
	assert.True(t, foundFailLine)
}

func TestParser_ParseGoFmtOutput(t *testing.T) {
	parser := NewParser()

	stdout := `main.go
utils/helper.go
internal/module/file.go`

	failures := parser.parseGoFmtOutput(stdout, "")
	require.Len(t, failures, 3)

	assert.Equal(t, "main.go", failures[0].File)
	assert.Equal(t, "File needs formatting", failures[0].Message)
	assert.Equal(t, "warning", failures[0].Severity)
}

func TestParser_ParseGoFmtOutput_Empty(t *testing.T) {
	parser := NewParser()

	failures := parser.parseGoFmtOutput("", "")
	assert.Empty(t, failures)

	failures = parser.parseGoFmtOutput("   ", "")
	assert.Empty(t, failures)
}

func TestParser_ParseGenericOutput(t *testing.T) {
	parser := NewParser()

	stdout := `Error: something went wrong
FAIL: test failed
error in processing`

	failures := parser.parseGenericOutput(stdout, "")

	// Should find error keywords
	assert.True(t, len(failures) > 0)
}

func TestParser_ParseGenericOutput_WithFile(t *testing.T) {
	parser := NewParser()

	stdout := `file.txt:123: error message here`

	failures := parser.parseGenericOutput(stdout, "")

	// Should parse file and line
	found := false
	for _, f := range failures {
		if f.File == "file.txt" && f.Line == 123 {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected to parse file:line pattern")
}

func TestContainsError(t *testing.T) {
	assert.True(t, containsError("Error: something"))
	assert.True(t, containsError("ERROR: something"))
	assert.True(t, containsError("failed:"))
	assert.True(t, containsError("FAILED:"))
	assert.True(t, containsError("FAIL: test"))

	assert.False(t, containsError("failed to compile")) // No colon, not detected
	assert.False(t, containsError("Success!"))
	assert.False(t, containsError("passed"))
	assert.False(t, containsError(""))
}

func TestNewFailureFormatter(t *testing.T) {
	formatter := NewFailureFormatter()
	assert.NotNil(t, formatter)
}

func TestFailureFormatter_Format(t *testing.T) {
	formatter := NewFailureFormatter()

	// Failure with file and line
	failure := GateFailure{
		File:    "main.go",
		Line:    42,
		Message: "undefined variable",
	}
	formatted := formatter.Format(failure)
	assert.Equal(t, "main.go:42: undefined variable", formatted)

	// Failure with file only
	failure = GateFailure{
		File:    "main.go",
		Message: "syntax error",
	}
	formatted = formatter.Format(failure)
	assert.Equal(t, "main.go: syntax error", formatted)

	// Failure with message only
	failure = GateFailure{
		Message: "general error",
	}
	formatted = formatter.Format(failure)
	assert.Equal(t, "general error", formatted)
}

func TestFailureFormatter_FormatList(t *testing.T) {
	formatter := NewFailureFormatter()

	failures := []GateFailure{
		{File: "a.go", Line: 1, Message: "error 1"},
		{File: "b.go", Line: 2, Message: "error 2"},
	}

	formatted := formatter.FormatList(failures)
	assert.Len(t, formatted, 2)
	assert.Equal(t, "a.go:1: error 1", formatted[0])
}

func TestFailureFormatter_Summary(t *testing.T) {
	formatter := NewFailureFormatter()

	// Empty
	summary := formatter.Summary([]GateFailure{})
	assert.Equal(t, "No failures", summary)

	// Single
	summary = formatter.Summary([]GateFailure{{Message: "one error"}})
	assert.Equal(t, "one error", summary)

	// Multiple
	summary = formatter.Summary([]GateFailure{
		{Message: "error 1"},
		{Message: "error 2"},
	})
	assert.Equal(t, "2 failures found", summary)
}

func TestParser_AddPattern(t *testing.T) {
	parser := NewParser()

	err := parser.AddPattern("custom", `^(.*):(\d+):\s*(.+)$`)
	require.NoError(t, err)

	// Use custom pattern
	output := "file.txt:123: error message"
	failures := parser.ParseWithPattern(output, "custom")
	assert.Len(t, failures, 1)
	assert.Equal(t, "file.txt", failures[0].File)
	assert.Equal(t, 123, failures[0].Line)
}

func TestParser_AddPattern_Invalid(t *testing.T) {
	parser := NewParser()

	err := parser.AddPattern("invalid", "[")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pattern")
}

func TestParser_ParseWithPattern_NotFound(t *testing.T) {
	parser := NewParser()

	failures := parser.ParseWithPattern("output", "nonexistent")
	assert.Empty(t, failures)
}

func TestCountBySeverity(t *testing.T) {
	failures := []GateFailure{
		{Severity: "error", Message: "1"},
		{Severity: "error", Message: "2"},
		{Severity: "warning", Message: "3"},
		{Severity: "info", Message: "4"},
		{Message: "5"}, // No severity = error
	}

	counts := CountBySeverity(failures)
	assert.Equal(t, 3, counts["error"])
	assert.Equal(t, 1, counts["warning"])
	assert.Equal(t, 1, counts["info"])
}

func TestFilterBySeverity(t *testing.T) {
	failures := []GateFailure{
		{Severity: "error", Message: "1"},
		{Severity: "warning", Message: "2"},
		{Severity: "error", Message: "3"},
	}

	errors := FilterBySeverity(failures, "error")
	assert.Len(t, errors, 2)

	warnings := FilterBySeverity(failures, "warning")
	assert.Len(t, warnings, 1)

	infos := FilterBySeverity(failures, "info")
	assert.Len(t, infos, 0)
}

func TestHasErrors(t *testing.T) {
	assert.True(t, HasErrors([]GateFailure{
		{Severity: "error", Message: "1"},
	}))

	assert.True(t, HasErrors([]GateFailure{
		{Message: "1"}, // No severity = error
	}))

	assert.False(t, HasErrors([]GateFailure{
		{Severity: "warning", Message: "1"},
	}))

	assert.False(t, HasErrors([]GateFailure{}))
}

func TestHasWarnings(t *testing.T) {
	assert.True(t, HasWarnings([]GateFailure{
		{Severity: "warning", Message: "1"},
	}))

	assert.False(t, HasWarnings([]GateFailure{
		{Severity: "error", Message: "1"},
	}))

	assert.False(t, HasWarnings([]GateFailure{}))
}

func TestParseExitCode(t *testing.T) {
	assert.Equal(t, "pass", ParseExitCode(0))
	assert.Equal(t, "fail", ParseExitCode(1))
	assert.Equal(t, "fail", ParseExitCode(255))
	assert.Equal(t, "fail", ParseExitCode(-1))
}

func TestParser_ParseOutput_GoVet(t *testing.T) {
	parser := NewParser()

	stdout := `main.go:42:15: undefined: someFunction`
	failures, err := parser.ParseOutput("go-vet", stdout, "")

	require.NoError(t, err)
	assert.Len(t, failures, 1)
	assert.Equal(t, "main.go", failures[0].File)
	assert.Equal(t, 42, failures[0].Line)
}

func TestParser_ParseOutput_GoTest(t *testing.T) {
	parser := NewParser()

	stdout := `--- FAIL: TestSomething (0.00s)
    something_test.go:15: expected true, got false`
	failures, err := parser.ParseOutput("go-test", stdout, "")

	require.NoError(t, err)
	assert.True(t, len(failures) > 0)
}

func TestParser_ParseOutput_GoFmt(t *testing.T) {
	parser := NewParser()

	stdout := "main.go\nutils.go"
	failures, err := parser.ParseOutput("go-fmt", stdout, "")

	require.NoError(t, err)
	assert.Len(t, failures, 2)
	assert.Equal(t, "main.go", failures[0].File)
	assert.Equal(t, "warning", failures[0].Severity)
}

func TestParser_ParseOutput_Lint(t *testing.T) {
	parser := NewParser()

	stdout := `main.go:42:15: undefined: someFunction`
	failures, err := parser.ParseOutput("lint", stdout, "")

	require.NoError(t, err)
	assert.Len(t, failures, 1)
}

func TestParser_ParseOutput_UnknownGate(t *testing.T) {
	parser := NewParser()

	stdout := "Error: something went wrong"
	failures, err := parser.ParseOutput("unknown-gate", stdout, "")

	require.NoError(t, err)
	assert.True(t, len(failures) > 0)
}

func TestParser_ParseOutput_Empty(t *testing.T) {
	parser := NewParser()

	failures, err := parser.ParseOutput("test", "", "")

	require.NoError(t, err)
	assert.Empty(t, failures)
}
