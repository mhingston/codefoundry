package protocol

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/xeipuuv/gojsonschema"
)

// Validator handles JSON schema validation
type Validator struct {
	schemas map[string]*gojsonschema.Schema
}

// NewValidator creates a new validator
func NewValidator() *Validator {
	return &Validator{
		schemas: make(map[string]*gojsonschema.Schema),
	}
}

// LoadSchema loads a JSON schema from file
func (v *Validator) LoadSchema(path string) error {
	schemaData, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	schemaLoader := gojsonschema.NewBytesLoader(schemaData)
	schema, err := gojsonschema.NewSchema(schemaLoader)
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	v.schemas[path] = schema
	return nil
}

// Validate validates data against a loaded schema
func (v *Validator) Validate(schemaPath string, data interface{}) error {
	schema, ok := v.schemas[schemaPath]
	if !ok {
		if err := v.LoadSchema(schemaPath); err != nil {
			return err
		}
		schema = v.schemas[schemaPath]
	}

	// Convert data to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	documentLoader := gojsonschema.NewBytesLoader(jsonData)
	result, err := schema.Validate(documentLoader)
	if err != nil {
		return fmt.Errorf("validation error: %w", err)
	}

	if !result.Valid() {
		errors := make([]string, 0, len(result.Errors()))
		for _, err := range result.Errors() {
			errors = append(errors, err.String())
		}
		return &ValidationError{Errors: errors}
	}

	return nil
}

// ValidateProtocol validates a protocol against the protocol schema
func (v *Validator) ValidateProtocol(protocol *Protocol, schemaPath string) error {
	return v.Validate(schemaPath, protocol)
}

// ValidateVariantBlock validates optional variant definitions under deterministic constraints.
func ValidateVariantBlock(variants []Variant) error {
	if len(variants) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(variants))
	for i, variant := range variants {
		if variant.ID == "" {
			return fmt.Errorf("variant at index %d is missing required id", i)
		}
		if _, ok := seen[variant.ID]; ok {
			return fmt.Errorf("duplicate variant id: %s", variant.ID)
		}
		seen[variant.ID] = struct{}{}
	}

	return nil
}

// ValidationError represents a validation error with details
type ValidationError struct {
	Errors []string
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if len(e.Errors) == 0 {
		return "validation failed"
	}
	return fmt.Sprintf("validation failed: %v", e.Errors)
}

// ValidateStatus validates a status object against the status schema
func (v *Validator) ValidateStatus(status interface{}, schemaPath string) error {
	return v.Validate(schemaPath, status)
}

// ValidateGateReport validates a gate report against the gate report schema
func (v *Validator) ValidateGateReport(report interface{}, schemaPath string) error {
	return v.Validate(schemaPath, report)
}

// ValidateState validates a state object against the state schema
func (v *Validator) ValidateState(state interface{}, schemaPath string) error {
	return v.Validate(schemaPath, state)
}
