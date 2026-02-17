package protocol

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_ValidateProtocol(t *testing.T) {
	// Create test schema
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")
	
	schema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["name", "version", "stages"],
  "properties": {
    "name": {"type": "string"},
    "version": {"type": "string"},
    "stages": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["id", "name"],
        "properties": {
          "id": {"type": "string"},
          "name": {"type": "string"}
        }
      }
    }
  }
}`
	require.NoError(t, os.WriteFile(schemaPath, []byte(schema), 0644))
	
	validator := NewValidator()
	
	// Valid protocol
	protocol := &Protocol{
		Name:    "test",
		Version: "1.0.0",
		Stages: []Stage{
			{ID: "stage1", Name: "Stage 1"},
		},
	}
	
	err := validator.ValidateProtocol(protocol, schemaPath)
	assert.NoError(t, err)
}

func TestValidator_ValidateProtocol_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "schema.json")
	
	schema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["name", "version"],
  "properties": {
    "name": {"type": "string"},
    "version": {"type": "string", "pattern": "^[0-9]+\\.[0-9]+\\.[0-9]+$"}
  }
}`
	require.NoError(t, os.WriteFile(schemaPath, []byte(schema), 0644))
	
	validator := NewValidator()
	
	// Invalid version format
	protocol := &Protocol{
		Name:    "test",
		Version: "invalid",
	}
	
	err := validator.ValidateProtocol(protocol, schemaPath)
	assert.Error(t, err)
	
	validationErr, ok := err.(*ValidationError)
	assert.True(t, ok)
	assert.NotEmpty(t, validationErr.Errors)
}

func TestValidator_LoadSchema_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "invalid.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte("not json"), 0644))
	
	validator := NewValidator()
	err := validator.LoadSchema(schemaPath)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to compile schema")
}

func TestValidator_Validate_MissingSchema(t *testing.T) {
	validator := NewValidator()
	
	data := map[string]string{"key": "value"}
	err := validator.Validate("/nonexistent/schema.json", data)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read schema file")
}

func TestValidationError_Error(t *testing.T) {
	// Empty errors
	err := &ValidationError{Errors: []string{}}
	assert.Equal(t, "validation failed", err.Error())
	
	// With errors
	err = &ValidationError{
		Errors: []string{"error 1", "error 2"},
	}
	assert.Contains(t, err.Error(), "validation failed")
	assert.Contains(t, err.Error(), "error 1")
	assert.Contains(t, err.Error(), "error 2")
}

func TestValidator_ValidateStatus(t *testing.T) {
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "status-schema.json")
	
	schema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["schema_version", "stage_id", "status"],
  "properties": {
    "schema_version": {"type": "string"},
    "stage_id": {"type": "string"},
    "status": {"enum": ["pending", "running", "pass", "fail", "skipped"]}
  }
}`
	require.NoError(t, os.WriteFile(schemaPath, []byte(schema), 0644))
	
	validator := NewValidator()
	
	// Valid status
	status := map[string]string{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       "test",
		"status":         "pass",
	}
	
	err := validator.ValidateStatus(status, schemaPath)
	assert.NoError(t, err)
	
	// Invalid status
	status["status"] = "invalid"
	err = validator.ValidateStatus(status, schemaPath)
	assert.Error(t, err)
}

func TestValidator_ValidateGateReport(t *testing.T) {
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "gate-report-schema.json")
	
	schema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["schema_version", "gate_id", "status"],
  "properties": {
    "schema_version": {"type": "string"},
    "gate_id": {"type": "string"},
    "status": {"enum": ["pass", "fail", "error", "skipped"]}
  }
}`
	require.NoError(t, os.WriteFile(schemaPath, []byte(schema), 0644))
	
	validator := NewValidator()
	
	report := map[string]interface{}{
		"schema_version": "codefoundry_gate_report.v1",
		"gate_id":        "test-gate",
		"status":         "pass",
	}
	
	err := validator.ValidateGateReport(report, schemaPath)
	assert.NoError(t, err)
}

func TestValidator_ValidateState(t *testing.T) {
	tmpDir := t.TempDir()
	schemaPath := filepath.Join(tmpDir, "state-schema.json")
	
	schema := `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["schema_version", "run_id", "protocol_version", "stages"],
  "properties": {
    "schema_version": {"type": "string"},
    "run_id": {"type": "string"},
    "protocol_version": {"type": "string"},
    "stages": {"type": "object"}
  }
}`
	require.NoError(t, os.WriteFile(schemaPath, []byte(schema), 0644))
	
	validator := NewValidator()
	
	state := map[string]interface{}{
		"schema_version":   "codefoundry_state.v1",
		"run_id":          "run-123",
		"protocol_version": "1.0.0",
		"stages":          map[string]interface{}{},
	}
	
	err := validator.ValidateState(state, schemaPath)
	assert.NoError(t, err)
}
