package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoader_Load(t *testing.T) {
	// Create a temporary test protocol file
	tmpDir := t.TempDir()
	protocolPath := filepath.Join(tmpDir, "test-protocol.yaml")

	protocolContent := `
name: "test-protocol"
version: "1.0.0"
description: "Test protocol"
stages:
  - id: stage1
    name: "Stage 1"
    template: stage1.md
    outputs: [stage1.md]
  - id: stage2
    name: "Stage 2"
    template: stage2.md
    depends_on: [stage1]
    inputs: [stage1.md]
    outputs: [stage2.md]
gates:
  - id: test-gate
    name: "Test Gate"
    command: "echo test"
    required: true
    timeout: 60
`
	require.NoError(t, os.WriteFile(protocolPath, []byte(protocolContent), 0644))

	loader := NewLoader()
	protocol, err := loader.Load(protocolPath)

	require.NoError(t, err)
	assert.Equal(t, "test-protocol", protocol.Name)
	assert.Equal(t, "1.0.0", protocol.Version)
	assert.Equal(t, "Test protocol", protocol.Description)
	assert.Len(t, protocol.Stages, 2)
	assert.Len(t, protocol.Gates, 1)

	// Check stage defaults
	assert.Equal(t, "spec", protocol.Stages[0].Type)
	assert.Equal(t, 5, protocol.Stages[0].MaxConcurrent)

	// Check gate timeout was NOT overridden (explicit value preserved)
	assert.Equal(t, 60, protocol.Gates[0].Timeout)
}

func TestLoader_Load_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	protocolPath := filepath.Join(tmpDir, "invalid.yaml")

	// Invalid YAML content
	require.NoError(t, os.WriteFile(protocolPath, []byte("invalid: yaml: content: ["), 0644))

	loader := NewLoader()
	_, err := loader.Load(protocolPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse protocol YAML")
}

func TestLoader_Load_MissingFile(t *testing.T) {
	loader := NewLoader()
	_, err := loader.Load("/nonexistent/path.yaml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read protocol file")
}

func TestLoader_LoadAndValidate_DuplicateStageID(t *testing.T) {
	tmpDir := t.TempDir()
	protocolPath := filepath.Join(tmpDir, "test.yaml")

	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: dup
    name: "Duplicate"
  - id: dup
    name: "Also Duplicate"
`
	require.NoError(t, os.WriteFile(protocolPath, []byte(protocolContent), 0644))

	loader := NewLoader()
	_, err := loader.LoadAndValidate(protocolPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate stage ID")
}

func TestLoader_LoadAndValidate_InvalidDependency(t *testing.T) {
	tmpDir := t.TempDir()
	protocolPath := filepath.Join(tmpDir, "test.yaml")

	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: stage1
    name: "Stage 1"
    depends_on: [nonexistent]
`
	require.NoError(t, os.WriteFile(protocolPath, []byte(protocolContent), 0644))

	loader := NewLoader()
	_, err := loader.LoadAndValidate(protocolPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "depends on unknown stage")
}

func TestLoader_LoadAndValidate_InvalidStageType(t *testing.T) {
	tmpDir := t.TempDir()
	protocolPath := filepath.Join(tmpDir, "test.yaml")

	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: stage1
    name: "Stage 1"
    type: invalid_type
`
	require.NoError(t, os.WriteFile(protocolPath, []byte(protocolContent), 0644))

	loader := NewLoader()
	_, err := loader.LoadAndValidate(protocolPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type")
}

func TestProtocol_GetStage(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "stage1", Name: "Stage 1"},
			{ID: "stage2", Name: "Stage 2"},
		},
	}

	stage, err := protocol.GetStage("stage1")
	require.NoError(t, err)
	assert.Equal(t, "Stage 1", stage.Name)

	stage, err = protocol.GetStage("stage2")
	require.NoError(t, err)
	assert.Equal(t, "Stage 2", stage.Name)

	_, err = protocol.GetStage("nonexistent")
	assert.Error(t, err)
}

func TestProtocol_GetGate(t *testing.T) {
	protocol := &Protocol{
		Gates: []GateDefinition{
			{ID: "gate1", Name: "Gate 1", Command: "echo 1"},
			{ID: "gate2", Name: "Gate 2", Command: "echo 2"},
		},
	}

	gate, err := protocol.GetGate("gate1")
	require.NoError(t, err)
	assert.Equal(t, "Gate 1", gate.Name)

	gate, err = protocol.GetGate("gate2")
	require.NoError(t, err)
	assert.Equal(t, "Gate 2", gate.Name)

	_, err = protocol.GetGate("nonexistent")
	assert.Error(t, err)
}

func TestLoader_LoadAndValidate_MissingGateReference(t *testing.T) {
	tmpDir := t.TempDir()
	protocolPath := filepath.Join(tmpDir, "test.yaml")

	protocolContent := `
name: "test"
version: "1.0.0"
stages:
  - id: stage1
    name: "Stage 1"
    gates: [nonexistent-gate]
`
	require.NoError(t, os.WriteFile(protocolPath, []byte(protocolContent), 0644))

	loader := NewLoader()
	_, err := loader.LoadAndValidate(protocolPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown gate")
}

func TestLoader_Load_ComplexProtocol(t *testing.T) {
	tmpDir := t.TempDir()
	protocolPath := filepath.Join(tmpDir, "complex.yaml")

	protocolContent := `
name: "complex-protocol"
version: "1.2.3"
description: "A complex test protocol"
stages:
  - id: plan
    name: "Plan"
    template: plan.md
    outputs: [plan.md]
  - id: spec
    name: "Specification"
    template: spec.md
    depends_on: [plan]
    inputs: [plan.md]
    outputs: [spec.md]
  - id: implement
    name: "Implement"
    template: implement.md
    depends_on: [spec]
    inputs: [spec.md]
    outputs: ["**/*.go"]
  - id: verify
    name: "Verify"
    depends_on: [implement]
    gates: [test, vet]
    outputs: [verify-report.json]
gates:
  - id: test
    name: "Test"
    command: "go test ./..."
    required: true
    timeout: 300
  - id: vet
    name: "Vet"
    command: "go vet ./..."
    required: true
    timeout: 60
`
	require.NoError(t, os.WriteFile(protocolPath, []byte(protocolContent), 0644))

	loader := NewLoader()
	protocol, err := loader.Load(protocolPath)

	require.NoError(t, err)
	assert.Equal(t, "complex-protocol", protocol.Name)
	assert.Equal(t, "1.2.3", protocol.Version)
	assert.Len(t, protocol.Stages, 4)
	assert.Len(t, protocol.Gates, 2)

	// Check stage dependencies
	planStage, _ := protocol.GetStage("plan")
	assert.Empty(t, planStage.DependsOn)

	specStage, _ := protocol.GetStage("spec")
	assert.Equal(t, []string{"plan"}, specStage.DependsOn)

	// Check gate configuration
	testGate, _ := protocol.GetGate("test")
	assert.Equal(t, 300, testGate.Timeout)
	assert.True(t, testGate.Required)
}
