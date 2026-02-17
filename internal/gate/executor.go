package gate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/protocol"
)

// Executor executes gates (shell commands) with proper timeout and capture
type Executor struct {
	artifactStore *artifact.Store
}

// GateResult represents the result of gate execution
type GateResult struct {
	SchemaVersion string          `json:"schema_version"`
	GateID        string          `json:"gate_id"`
	Status        string          `json:"status"`
	Command       string          `json:"command"`
	ExitCode      int             `json:"exit_code,omitempty"`
	DurationMs    int64           `json:"duration_ms,omitempty"`
	Stdout        string          `json:"stdout,omitempty"`
	Stderr        string          `json:"stderr,omitempty"`
	Failures      []GateFailure   `json:"failures,omitempty"`
	Timestamp     string          `json:"timestamp"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// GateFailure represents a failure found by a gate
type GateFailure struct {
	Message  string `json:"message"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Severity string `json:"severity,omitempty"`
}

// NewExecutor creates a new gate executor
func NewExecutor(artifactStore *artifact.Store) *Executor {
	return &Executor{
		artifactStore: artifactStore,
	}
}

// Execute runs a single gate
func (e *Executor) Execute(ctx context.Context, gateDef *protocol.GateDefinition, workingDir string) (*GateResult, error) {
	start := time.Now()
	
	result := &GateResult{
		SchemaVersion: "codefoundry_gate_report.v1",
		GateID:        gateDef.ID,
		Command:       gateDef.Command,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Status:        "running",
	}

	// Prepare command
	cmd := exec.CommandContext(ctx, "sh", "-c", gateDef.Command)
	cmd.Dir = workingDir

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range gateDef.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Capture output
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Run command with timeout
	timeout := time.Duration(gateDef.Timeout) * time.Second
	if timeout == 0 {
		timeout = 300 * time.Second // default 5 minutes
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := cmd.Run()
	duration := time.Since(start)
	result.DurationMs = duration.Milliseconds()

	// Process output
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()

	// Determine status
	if ctx.Err() == context.DeadlineExceeded {
		result.Status = "fail"
		result.ExitCode = -1
		result.Failures = append(result.Failures, GateFailure{
			Message:  fmt.Sprintf("Gate timed out after %v", timeout),
			Severity: "error",
		})
	} else if err != nil {
		result.Status = "fail"
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = -1
		}

		// Parse failures from output
		result.Failures = e.parseFailures(gateDef.ID, result.Stdout, result.Stderr)
	} else {
		result.Status = "pass"
		result.ExitCode = 0
	}

	return result, nil
}

// ExecuteAndStore runs a gate and stores the result
func (e *Executor) ExecuteAndStore(ctx context.Context, gateDef *protocol.GateDefinition, workingDir, stageID string) (*GateResult, error) {
	result, err := e.Execute(ctx, gateDef, workingDir)
	if err != nil {
		return nil, err
	}

	// Store result
	if e.artifactStore != nil {
		if err := e.artifactStore.WriteJSON(stageID, fmt.Sprintf("%s.json", gateDef.ID), result); err != nil {
			return nil, fmt.Errorf("failed to store gate result: %w", err)
		}
	}

	return result, nil
}

// ExecuteGates runs multiple gates for a stage
func (e *Executor) ExecuteGates(ctx context.Context, gates []protocol.GateDefinition, workingDir, stageID string) (*GateExecutionResult, error) {
	result := &GateExecutionResult{
		StageID:      stageID,
		Results:      make([]*GateResult, 0, len(gates)),
		PassedGates:  []string{},
		FailedGates:  []string{},
		StartTime:    time.Now().UTC(),
	}

	for i := range gates {
		gateDef := &gates[i]
		
		gateResult, err := e.ExecuteAndStore(ctx, gateDef, workingDir, stageID)
		if err != nil {
			return nil, fmt.Errorf("failed to execute gate %s: %w", gateDef.ID, err)
		}

		result.Results = append(result.Results, gateResult)
		
		if gateResult.Status == "pass" {
			result.PassedGates = append(result.PassedGates, gateDef.ID)
		} else {
			result.FailedGates = append(result.FailedGates, gateDef.ID)
			if gateDef.Required {
				result.AllPassed = false
			}
		}
	}

	result.EndTime = time.Now().UTC()
	result.AllPassed = len(result.FailedGates) == 0

	return result, nil
}

// parseFailures attempts to parse structured failures from command output
func (e *Executor) parseFailures(gateID, stdout, stderr string) []GateFailure {
	failures := []GateFailure{}

	// Try to parse as JSON first
	var jsonFailures []GateFailure
	if err := json.Unmarshal([]byte(stdout), &jsonFailures); err == nil {
		return jsonFailures
	}

	// Otherwise, try common error formats
	output := stdout + stderr
	scanner := bufio.NewScanner(strings.NewReader(output))
	
	for scanner.Scan() {
		line := scanner.Text()
		failure := e.parseFailureLine(gateID, line)
		if failure != nil {
			failures = append(failures, *failure)
		}
	}

	return failures
}

// parseFailureLine parses a single line for failure information
func (e *Executor) parseFailureLine(gateID, line string) *GateFailure {
	// Common patterns: file.go:123: error message
	//                 file.go:123:5: error message
	//                 file.go(123): error message
	
	parts := strings.SplitN(line, ":", 3)
	if len(parts) >= 2 {
		file := strings.TrimSpace(parts[0])
		
		// Try to parse line number
		var lineNum int
		fmt.Sscanf(parts[1], "%d", &lineNum)
		
		message := ""
		if len(parts) >= 3 {
			message = strings.TrimSpace(parts[2])
		}

		// Only create failure if we have reasonable data
		if file != "" && (filepath.Ext(file) != "" || strings.Contains(file, "/")) {
			return &GateFailure{
				Message:  message,
				File:     file,
				Line:     lineNum,
				Severity: "error",
			}
		}
	}

	// Fallback: treat the whole line as a message if it looks like an error
	if strings.Contains(strings.ToLower(line), "error") || 
	   strings.Contains(strings.ToLower(line), "fail") {
		return &GateFailure{
			Message:  strings.TrimSpace(line),
			Severity: "error",
		}
	}

	return nil
}

// GateExecutionResult contains results for all gates in a stage
type GateExecutionResult struct {
	StageID      string         `json:"stage_id"`
	Results      []*GateResult  `json:"results"`
	PassedGates  []string       `json:"passed_gates"`
	FailedGates  []string       `json:"failed_gates"`
	AllPassed    bool           `json:"all_passed"`
	StartTime    time.Time      `json:"start_time"`
	EndTime      time.Time      `json:"end_time"`
}

// Duration returns the total duration
func (r *GateExecutionResult) Duration() time.Duration {
	return r.EndTime.Sub(r.StartTime)
}

// HasFailures returns true if any required gate failed
func (r *GateExecutionResult) HasFailures() bool {
	return len(r.FailedGates) > 0
}

// GetFailedRequiredGates returns IDs of failed required gates
func (r *GateExecutionResult) GetFailedRequiredGates(gates []protocol.GateDefinition) []string {
	failed := []string{}
	for _, gateID := range r.FailedGates {
		for i := range gates {
			if gates[i].ID == gateID && gates[i].Required {
				failed = append(failed, gateID)
				break
			}
		}
	}
	return failed
}

// ValidateGateDefinitions validates a list of gate definitions
func ValidateGateDefinitions(gates []protocol.GateDefinition) error {
	gateIDs := make(map[string]bool)
	
	for _, gate := range gates {
		if gate.ID == "" {
			return fmt.Errorf("gate ID cannot be empty")
		}
		
		if gateIDs[gate.ID] {
			return fmt.Errorf("duplicate gate ID: %s", gate.ID)
		}
		gateIDs[gate.ID] = true
		
		if gate.Command == "" {
			return fmt.Errorf("gate %s has no command", gate.ID)
		}
		
		if gate.Timeout < 0 {
			return fmt.Errorf("gate %s has invalid timeout: %d", gate.ID, gate.Timeout)
		}
	}
	
	return nil
}

// CreateArtifactStore creates an artifact store for the executor
func (e *Executor) CreateArtifactStore(basePath, runID string) *artifact.Store {
	ns := artifact.NewNamespace(basePath, runID)
	return artifact.NewStore(ns)
}
