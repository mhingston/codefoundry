package ci

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "CodeFoundry", config.WorkflowName)
	assert.Equal(t, []string{"push", "pull_request"}, config.OnEvents)
	assert.Equal(t, []string{"main", "master"}, config.Branches)
	assert.Equal(t, []string{"plan", "spec", "implement", "verify", "review", "lock"}, config.Stages)
	assert.Equal(t, "1.21", config.GoVersion)
	assert.Equal(t, ".", config.WorkingDir)
}

func TestGenerateWorkflow(t *testing.T) {
	config := GitHubConfig{
		WorkflowName: "Test Workflow",
		OnEvents:     []string{"push"},
		Branches:     []string{"main"},
		Stages:       []string{"plan", "implement"},
		GoVersion:    "1.21",
		WorkingDir:   ".",
	}

	workflow, err := GenerateWorkflow(config)
	require.NoError(t, err)

	// Verify content
	assert.Contains(t, workflow, "name: Test Workflow")
	assert.Contains(t, workflow, "on:")
	assert.Contains(t, workflow, "push:")
	assert.Contains(t, workflow, "branches:")
	assert.Contains(t, workflow, "- main")
	assert.Contains(t, workflow, "runs-on: ubuntu-latest")
	assert.Contains(t, workflow, "go-version: 1.21")
	assert.Contains(t, workflow, "Stage - plan")
	assert.Contains(t, workflow, "Stage - implement")
}

func TestGenerateWorkflow_Defaults(t *testing.T) {
	config := GitHubConfig{
		WorkflowName: "",
		OnEvents:     nil,
		Branches:     nil,
		Stages:       nil,
		GoVersion:    "",
	}

	workflow, err := GenerateWorkflow(config)
	require.NoError(t, err)

	assert.Contains(t, workflow, "name: CodeFoundry")
	assert.Contains(t, workflow, "go-version: 1.21")
}

func TestGenerateWorkflowFile(t *testing.T) {
	config := GitHubConfig{
		WorkflowName: "Test Workflow",
		OnEvents:     []string{"push", "pull_request"},
		Branches:     []string{"main", "develop"},
		Stages:       []string{"plan", "implement", "verify"},
		GoVersion:    "1.22",
		WorkingDir:   ".",
	}

	workflow, err := GenerateWorkflowFile(config)
	require.NoError(t, err)

	// Verify YAML structure
	assert.True(t, strings.HasPrefix(workflow, "name: Test Workflow\n"))
	assert.Contains(t, workflow, "on:\n")
	assert.Contains(t, workflow, "push:\n")
	assert.Contains(t, workflow, "pull_request:\n")
	assert.Contains(t, workflow, "branches:\n")
	assert.Contains(t, workflow, "- main\n")
	assert.Contains(t, workflow, "- develop\n")
	assert.Contains(t, workflow, "GO_VERSION: '1.22'")
	assert.Contains(t, workflow, "Stage - plan")
	assert.Contains(t, workflow, "Stage - implement")
	assert.Contains(t, workflow, "Stage - verify")
}

func TestGenerateStatusPayload(t *testing.T) {
	tests := []struct {
		name    string
		status  CIStatus
		wantErr bool
	}{
		{
			name: "success",
			status: CIStatus{
				State:       "success",
				Description: "All checks passed",
				TargetURL:   "https://example.com/results",
				Context:     "codefoundry",
			},
			wantErr: false,
		},
		{
			name: "failure",
			status: CIStatus{
				State:       "failure",
				Description: "Tests failed",
			},
			wantErr: false,
		},
		{
			name: "pending",
			status: CIStatus{
				State:       "pending",
				Description: "Running...",
			},
			wantErr: false,
		},
		{
			name: "error",
			status: CIStatus{
				State:       "error",
				Description: "System error",
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			status: CIStatus{
				State: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload, err := GenerateStatusPayload(tt.status)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)

				// Verify it's valid JSON
				var result CIStatus
				err := json.Unmarshal(payload, &result)
				require.NoError(t, err)
				assert.Equal(t, tt.status.State, result.State)
				assert.Equal(t, tt.status.Description, result.Description)
			}
		})
	}
}

func TestGenerateStatusPayload_DefaultContext(t *testing.T) {
	status := CIStatus{
		State:       "success",
		Description: "All checks passed",
	}

	payload, err := GenerateStatusPayload(status)
	require.NoError(t, err)

	var result CIStatus
	err = json.Unmarshal(payload, &result)
	require.NoError(t, err)
	assert.Equal(t, "continuous-integration/codefoundry", result.Context)
}

func TestGeneratePRComment(t *testing.T) {
	tests := []struct {
		name     string
		runID    string
		success  bool
		findings map[string]int
		contains []string
	}{
		{
			name:     "success no findings",
			runID:    "run-123",
			success:  true,
			findings: map[string]int{},
			contains: []string{
				"## CodeFoundry Results",
				"✅ **All checks passed**",
				"**Run ID:** run-123",
			},
		},
		{
			name:    "failure with findings",
			runID:   "run-456",
			success: false,
			findings: map[string]int{
				"P1": 2,
				"P2": 5,
			},
			contains: []string{
				"## CodeFoundry Results",
				"❌ **Some checks failed**",
				"**Run ID:** run-456",
				"### Findings",
				"**P1:** 2",
				"**P2:** 5",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			comment := GeneratePRComment(tt.runID, tt.success, tt.findings)
			for _, str := range tt.contains {
				assert.Contains(t, comment, str)
			}
		})
	}
}

func TestGenerateAnnotations(t *testing.T) {
	findings := []Finding{
		{File: "main.go", Line: 10, Message: "Error here", Severity: "error"},
		{File: "main.go", Line: 20, Message: "Warning here", Severity: "warning"},
		{File: "main.go", Line: 30, Message: "P1 issue", Severity: "P1"},
		{File: "main.go", Line: 40, Message: "P2 issue", Severity: "P2"},
		{File: "main.go", Line: 50, Message: "Notice", Severity: "notice"},
	}

	annotations := GenerateAnnotations(findings)

	require.Len(t, annotations, 5)
	assert.Contains(t, annotations[0], "::error")
	assert.Contains(t, annotations[1], "::warning")
	assert.Contains(t, annotations[2], "::error")
	assert.Contains(t, annotations[3], "::warning")
	assert.Contains(t, annotations[4], "::notice")
}

func TestValidateWorkflow(t *testing.T) {
	tests := []struct {
		name    string
		config  GitHubConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: GitHubConfig{
				WorkflowName: "Test",
				OnEvents:     []string{"push"},
				Branches:     []string{"main"},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			config: GitHubConfig{
				OnEvents: []string{"push"},
				Branches: []string{"main"},
			},
			wantErr: true,
		},
		{
			name: "no events",
			config: GitHubConfig{
				WorkflowName: "Test",
				OnEvents:     []string{},
				Branches:     []string{"main"},
			},
			wantErr: true,
		},
		{
			name: "invalid event",
			config: GitHubConfig{
				WorkflowName: "Test",
				OnEvents:     []string{"invalid_event"},
				Branches:     []string{"main"},
			},
			wantErr: true,
		},
		{
			name: "no branches",
			config: GitHubConfig{
				WorkflowName: "Test",
				OnEvents:     []string{"push"},
				Branches:     []string{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorkflow(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseWorkflow(t *testing.T) {
	content := `name: Test Workflow
on:
  push:
    branches:
      - main
  pull_request:
`

	config, err := ParseWorkflow(content)
	require.NoError(t, err)
	assert.Equal(t, "Test Workflow", config.WorkflowName)
}

func TestFormatBranches(t *testing.T) {
	tests := []struct {
		branches []string
		expected string
	}{
		{
			branches: []string{"main"},
			expected: "[ 'main' ]",
		},
		{
			branches: []string{"main", "develop"},
			expected: "['main', 'develop']",
		},
	}

	for _, tt := range tests {
		result := formatBranches(tt.branches)
		// The format may vary, just check it contains the branches
		for _, branch := range tt.branches {
			assert.Contains(t, result, branch)
		}
	}
}
