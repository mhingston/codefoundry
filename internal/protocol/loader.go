package protocol

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Protocol represents a workflow protocol definition
type Protocol struct {
	Name        string           `yaml:"name" json:"name"`
	Version     string           `yaml:"version" json:"version"`
	Description string           `yaml:"description,omitempty" json:"description,omitempty"`
	Stages      []Stage          `yaml:"stages" json:"stages"`
	Gates       []GateDefinition `yaml:"gates,omitempty" json:"gates,omitempty"`
}

// Stage represents a workflow stage
type Stage struct {
	ID               string            `yaml:"id" json:"id"`
	Name             string            `yaml:"name" json:"name"`
	Description      string            `yaml:"description,omitempty" json:"description,omitempty"`
	Type             string            `yaml:"type,omitempty" json:"type,omitempty"`
	Source           string            `yaml:"source,omitempty" json:"source,omitempty"`
	Parallel         bool              `yaml:"parallel,omitempty" json:"parallel,omitempty"`
	WorktreeStrategy string            `yaml:"worktree_strategy,omitempty" json:"worktree_strategy,omitempty"`
	MaxConcurrent    int               `yaml:"max_concurrent,omitempty" json:"max_concurrent,omitempty"`
	Template         string            `yaml:"template,omitempty" json:"template,omitempty"`
	DependsOn        []string          `yaml:"depends_on,omitempty" json:"depends_on,omitempty"`
	Inputs           []string          `yaml:"inputs,omitempty" json:"inputs,omitempty"`
	Outputs          []string          `yaml:"outputs,omitempty" json:"outputs,omitempty"`
	Gates            []string          `yaml:"gates,omitempty" json:"gates,omitempty"`
	Condition        string            `yaml:"condition,omitempty" json:"condition,omitempty"`
	Hooks            map[string][]Hook `yaml:"hooks,omitempty" json:"hooks,omitempty"`
}

// GateDefinition represents a gate definition
type GateDefinition struct {
	ID       string            `yaml:"id" json:"id"`
	Name     string            `yaml:"name,omitempty" json:"name,omitempty"`
	Command  string            `yaml:"command" json:"command"`
	Required bool              `yaml:"required,omitempty" json:"required,omitempty"`
	Timeout  int               `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Env      map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// Hook represents a hook definition
type Hook struct {
	Type    string            `yaml:"type" json:"type"`
	URL     string            `yaml:"url" json:"url"`
	Timeout int               `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Retry   int               `yaml:"retry,omitempty" json:"retry,omitempty"`
	Async   bool              `yaml:"async,omitempty" json:"async,omitempty"`
	Headers map[string]string `yaml:"headers,omitempty" json:"headers,omitempty"`
	Secret  string            `yaml:"secret,omitempty" json:"secret,omitempty"`
	Env     map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// Loader handles protocol loading
type Loader struct {
	schemaValidator *Validator
}

// NewLoader creates a new protocol loader
func NewLoader() *Loader {
	return &Loader{
		schemaValidator: NewValidator(),
	}
}

// NewLoaderWithValidator creates a loader with a custom validator
func NewLoaderWithValidator(validator *Validator) *Loader {
	return &Loader{
		schemaValidator: validator,
	}
}

// Load loads a protocol from a YAML file
func (l *Loader) Load(path string) (*Protocol, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read protocol file: %w", err)
	}

	var protocol Protocol
	if err := yaml.Unmarshal(data, &protocol); err != nil {
		return nil, fmt.Errorf("failed to parse protocol YAML: %w", err)
	}

	// Set defaults
	for i := range protocol.Stages {
		if protocol.Stages[i].Type == "" {
			protocol.Stages[i].Type = "spec"
		}
		if protocol.Stages[i].MaxConcurrent == 0 {
			protocol.Stages[i].MaxConcurrent = 5
		}
	}

	// Set default gate timeouts
	for i := range protocol.Gates {
		if protocol.Gates[i].Timeout == 0 {
			protocol.Gates[i].Timeout = 300
		}
	}

	return &protocol, nil
}

// LoadAndValidate loads and validates a protocol
func (l *Loader) LoadAndValidate(path string) (*Protocol, error) {
	protocol, err := l.Load(path)
	if err != nil {
		return nil, err
	}

	// Validate against JSON schema
	schemaPath := filepath.Join(filepath.Dir(path), "..", "schemas", "protocol.schema.json")
	if _, err := os.Stat(schemaPath); err == nil {
		if err := l.schemaValidator.ValidateProtocol(protocol, schemaPath); err != nil {
			return nil, fmt.Errorf("protocol validation failed: %w", err)
		}
	}

	// Validate internal consistency
	if err := l.validateInternalConsistency(protocol); err != nil {
		return nil, fmt.Errorf("protocol consistency check failed: %w", err)
	}

	return protocol, nil
}

// validateInternalConsistency checks protocol for internal consistency
func (l *Loader) validateInternalConsistency(p *Protocol) error {
	stageIDs := make(map[string]bool)
	gateIDs := make(map[string]bool)

	// Check for duplicate stage IDs
	for _, stage := range p.Stages {
		if stageIDs[stage.ID] {
			return fmt.Errorf("duplicate stage ID: %s", stage.ID)
		}
		stageIDs[stage.ID] = true
	}

	// Check for duplicate gate IDs
	for _, gate := range p.Gates {
		if gateIDs[gate.ID] {
			return fmt.Errorf("duplicate gate ID: %s", gate.ID)
		}
		gateIDs[gate.ID] = true
	}

	// Check stage dependencies exist
	for _, stage := range p.Stages {
		for _, dep := range stage.DependsOn {
			if !stageIDs[dep] {
				return fmt.Errorf("stage '%s' depends on unknown stage: %s", stage.ID, dep)
			}
		}
	}

	// Check gate references exist
	for _, stage := range p.Stages {
		for _, gateID := range stage.Gates {
			if !gateIDs[gateID] {
				return fmt.Errorf("stage '%s' references unknown gate: %s", stage.ID, gateID)
			}
		}
	}

	// Validate stage types
	validTypes := map[string]bool{
		"spec":        true,
		"decompose":   true,
		"task_prompt": true,
		"parallel":    true,
		"decision":    true,
	}
	for _, stage := range p.Stages {
		if !validTypes[stage.Type] {
			return fmt.Errorf("stage '%s' has invalid type: %s", stage.ID, stage.Type)
		}
	}

	return nil
}

// GetStage returns a stage by ID
func (p *Protocol) GetStage(id string) (*Stage, error) {
	for i := range p.Stages {
		if p.Stages[i].ID == id {
			return &p.Stages[i], nil
		}
	}
	return nil, fmt.Errorf("stage not found: %s", id)
}

// GetGate returns a gate definition by ID
func (p *Protocol) GetGate(id string) (*GateDefinition, error) {
	for i := range p.Gates {
		if p.Gates[i].ID == id {
			return &p.Gates[i], nil
		}
	}
	return nil, fmt.Errorf("gate not found: %s", id)
}
