package review

import (
	"fmt"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   interface{}
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s: %v - %s", e.Field, e.Value, e.Message)
}

// NewValidationError creates a new validation error
func NewValidationError(field string, value interface{}) error {
	return ValidationError{
		Field:   field,
		Value:   value,
		Message: "value out of range",
	}
}

// TemplateError represents a template rendering error
type TemplateError struct {
	Op      string
	Message string
	Cause   error
}

func (e TemplateError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("template %s error: %s: %v", e.Op, e.Message, e.Cause)
	}
	return fmt.Sprintf("template %s error: %s", e.Op, e.Message)
}

func (e TemplateError) Unwrap() error {
	return e.Cause
}

// ExecutionError represents a review execution error
type ExecutionError struct {
	StageID string
	Message string
	Cause   error
}

func (e ExecutionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("review execution failed for stage %s: %s: %v", e.StageID, e.Message, e.Cause)
	}
	return fmt.Sprintf("review execution failed for stage %s: %s", e.StageID, e.Message)
}

func (e ExecutionError) Unwrap() error {
	return e.Cause
}
