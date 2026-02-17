package gate

import (
	"bufio"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Parser handles parsing gate command output
type Parser struct {
	patterns map[string]*regexp.Regexp
}

// NewParser creates a new output parser
func NewParser() *Parser {
	return &Parser{
		patterns: map[string]*regexp.Regexp{
			"go_error":   regexp.MustCompile(`^(.+\.go):(\d+):(\d+:)?\s*(.+)$`),
			"go_test":    regexp.MustCompile(`^(.+):(\d+):\s*(.+)$`),
			"line_error": regexp.MustCompile(`^(.+):(\d+):\s*(.+)$`),
		},
	}
}

// ParseOutput parses command output and extracts structured failures
func (p *Parser) ParseOutput(gateID, stdout, stderr string) ([]GateFailure, error) {
	failures := []GateFailure{}

	// First try to parse as JSON
	if jsonFailures, err := p.ParseJSON(stdout); err == nil {
		return jsonFailures, nil
	}

	// Otherwise parse based on gate type
	switch gateID {
	case "lint", "go-vet":
		failures = p.parseGoVetOutput(stdout, stderr)
	case "test", "go-test":
		failures = p.parseGoTestOutput(stdout, stderr)
	case "fmt", "go-fmt":
		failures = p.parseGoFmtOutput(stdout, stderr)
	default:
		failures = p.parseGenericOutput(stdout, stderr)
	}

	return failures, nil
}

// ParseJSON attempts to parse output as JSON failures
func (p *Parser) ParseJSON(output string) ([]GateFailure, error) {
	// Try to parse as an array of failures
	var failures []GateFailure
	if err := json.Unmarshal([]byte(output), &failures); err == nil {
		return failures, nil
	}

	// Try to parse as a single failure
	var failure GateFailure
	if err := json.Unmarshal([]byte(output), &failure); err == nil {
		if failure.Message != "" {
			return []GateFailure{failure}, nil
		}
	}

	return nil, fmt.Errorf("output is not valid JSON")
}

// parseGoVetOutput parses Go vet output
func (p *Parser) parseGoVetOutput(stdout, stderr string) []GateFailure {
	failures := []GateFailure{}
	output := stdout + "\n" + stderr

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		
		// Match: file.go:123:4: message
		if matches := p.patterns["go_error"].FindStringSubmatch(line); len(matches) >= 5 {
			lineNum, _ := strconv.Atoi(matches[2])
			failures = append(failures, GateFailure{
				File:     matches[1],
				Line:     lineNum,
				Message:  strings.TrimSpace(matches[4]),
				Severity: "error",
			})
		}
	}

	return failures
}

// parseGoTestOutput parses Go test output
func (p *Parser) parseGoTestOutput(stdout, stderr string) []GateFailure {
	failures := []GateFailure{}
	output := stdout + "\n" + stderr

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		
		// Match test failures
		if strings.HasPrefix(line, "--- FAIL:") {
			failures = append(failures, GateFailure{
				Message:  strings.TrimSpace(line),
				Severity: "error",
			})
		}
		
		// Match assertion failures with file info
		if matches := p.patterns["go_test"].FindStringSubmatch(line); len(matches) >= 4 {
			lineNum, _ := strconv.Atoi(matches[2])
			failures = append(failures, GateFailure{
				File:     matches[1],
				Line:     lineNum,
				Message:  strings.TrimSpace(matches[3]),
				Severity: "error",
			})
		}
	}

	return failures
}

// parseGoFmtOutput parses Go fmt output
func (p *Parser) parseGoFmtOutput(stdout, stderr string) []GateFailure {
	failures := []GateFailure{}
	
	// gofmt outputs files that need formatting, one per line
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		
		// Each line is a file that needs formatting
		failures = append(failures, GateFailure{
			File:     strings.TrimSpace(line),
			Message:  "File needs formatting",
			Severity: "warning",
		})
	}

	return failures
}

// parseGenericOutput parses generic error output
func (p *Parser) parseGenericOutput(stdout, stderr string) []GateFailure {
	failures := []GateFailure{}
	output := stdout + "\n" + stderr

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		
		// Try line_error pattern
		if matches := p.patterns["line_error"].FindStringSubmatch(line); len(matches) >= 4 {
			lineNum, _ := strconv.Atoi(matches[2])
			failures = append(failures, GateFailure{
				File:     matches[1],
				Line:     lineNum,
				Message:  strings.TrimSpace(matches[3]),
				Severity: "error",
			})
			continue
		}
		
		// Look for error keywords
		if containsError(line) {
			failures = append(failures, GateFailure{
				Message:  strings.TrimSpace(line),
				Severity: "error",
			})
		}
	}

	return failures
}

// containsError checks if a line contains error indicators
func containsError(line string) bool {
	keywords := []string{"error:", "ERROR:", "failed:", "FAILED:", "fail:", "FAIL:"}
	lower := strings.ToLower(line)
	
	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}
	
	return false
}

// ParseExitCode determines status from exit code
func ParseExitCode(exitCode int) string {
	if exitCode == 0 {
		return "pass"
	}
	return "fail"
}

// FailureFormatter formats failures for display
type FailureFormatter struct{}

// NewFailureFormatter creates a new formatter
func NewFailureFormatter() *FailureFormatter {
	return &FailureFormatter{}
}

// Format formats a failure for display
func (f *FailureFormatter) Format(failure GateFailure) string {
	if failure.File != "" {
		if failure.Line > 0 {
			return fmt.Sprintf("%s:%d: %s", failure.File, failure.Line, failure.Message)
		}
		return fmt.Sprintf("%s: %s", failure.File, failure.Message)
	}
	return failure.Message
}

// FormatList formats a list of failures
func (f *FailureFormatter) FormatList(failures []GateFailure) []string {
	formatted := make([]string, len(failures))
	for i, failure := range failures {
		formatted[i] = f.Format(failure)
	}
	return formatted
}

// Summary creates a summary of failures
func (f *FailureFormatter) Summary(failures []GateFailure) string {
	if len(failures) == 0 {
		return "No failures"
	}
	
	if len(failures) == 1 {
		return f.Format(failures[0])
	}
	
	return fmt.Sprintf("%d failures found", len(failures))
}

// AddPattern adds a custom parsing pattern
func (p *Parser) AddPattern(name, pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid pattern: %w", err)
	}
	
	p.patterns[name] = re
	return nil
}

// ParseWithPattern parses output with a specific pattern
func (p *Parser) ParseWithPattern(output, patternName string) []GateFailure {
	pattern, exists := p.patterns[patternName]
	if !exists {
		return []GateFailure{}
	}
	
	failures := []GateFailure{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	
	for scanner.Scan() {
		line := scanner.Text()
		if matches := pattern.FindStringSubmatch(line); len(matches) > 0 {
			failure := GateFailure{
				Message: matches[len(matches)-1],
				Severity: "error",
			}
			
			// Try to extract file and line if available
			if len(matches) >= 3 {
				failure.File = matches[1]
				if lineNum, err := strconv.Atoi(matches[2]); err == nil {
					failure.Line = lineNum
				}
			}
			
			failures = append(failures, failure)
		}
	}
	
	return failures
}

// CountBySeverity counts failures by severity
func CountBySeverity(failures []GateFailure) map[string]int {
	counts := map[string]int{
		"error":   0,
		"warning": 0,
		"info":    0,
	}
	
	for _, failure := range failures {
		severity := failure.Severity
		if severity == "" {
			severity = "error"
		}
		counts[severity]++
	}
	
	return counts
}

// FilterBySeverity filters failures by severity
func FilterBySeverity(failures []GateFailure, severity string) []GateFailure {
	filtered := []GateFailure{}
	for _, failure := range failures {
		if failure.Severity == severity {
			filtered = append(filtered, failure)
		}
	}
	return filtered
}

// HasErrors returns true if any failures are errors
func HasErrors(failures []GateFailure) bool {
	for _, failure := range failures {
		if failure.Severity == "error" || failure.Severity == "" {
			return true
		}
	}
	return false
}

// HasWarnings returns true if any failures are warnings
func HasWarnings(failures []GateFailure) bool {
	for _, failure := range failures {
		if failure.Severity == "warning" {
			return true
		}
	}
	return false
}
