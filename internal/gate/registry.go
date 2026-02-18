package gate

import (
	"fmt"
	"sync"

	"github.com/mhingston/codefoundry/internal/protocol"
)

// Registry manages gate definitions
type Registry struct {
	gates map[string]*protocol.GateDefinition
	mutex sync.RWMutex
}

// NewRegistry creates a new gate registry
func NewRegistry() *Registry {
	return &Registry{
		gates: make(map[string]*protocol.GateDefinition),
	}
}

// NewRegistryFromProtocol creates a registry from a protocol
func NewRegistryFromProtocol(p *protocol.Protocol) *Registry {
	registry := NewRegistry()

	for i := range p.Gates {
		gate := &p.Gates[i]
		registry.Register(gate)
	}

	return registry
}

// Register adds a gate definition to the registry
func (r *Registry) Register(gate *protocol.GateDefinition) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if gate.ID == "" {
		return fmt.Errorf("gate ID cannot be empty")
	}

	if _, exists := r.gates[gate.ID]; exists {
		return fmt.Errorf("gate already registered: %s", gate.ID)
	}

	// Apply defaults
	if gate.Timeout == 0 {
		gate.Timeout = 300 // 5 minutes default
	}

	r.gates[gate.ID] = gate
	return nil
}

// Unregister removes a gate from the registry
func (r *Registry) Unregister(gateID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if _, exists := r.gates[gateID]; !exists {
		return fmt.Errorf("gate not found: %s", gateID)
	}

	delete(r.gates, gateID)
	return nil
}

// Get retrieves a gate definition by ID
func (r *Registry) Get(gateID string) (*protocol.GateDefinition, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	gate, exists := r.gates[gateID]
	if !exists {
		return nil, fmt.Errorf("gate not found: %s", gateID)
	}

	// Return a copy to prevent external modification
	gateCopy := *gate
	return &gateCopy, nil
}

// MustGet retrieves a gate definition or panics
func (r *Registry) MustGet(gateID string) *protocol.GateDefinition {
	gate, err := r.Get(gateID)
	if err != nil {
		panic(err)
	}
	return gate
}

// Exists checks if a gate exists in the registry
func (r *Registry) Exists(gateID string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	_, exists := r.gates[gateID]
	return exists
}

// List returns all registered gate IDs
func (r *Registry) List() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	ids := make([]string, 0, len(r.gates))
	for id := range r.gates {
		ids = append(ids, id)
	}

	return ids
}

// ListAll returns all registered gate definitions
func (r *Registry) ListAll() []*protocol.GateDefinition {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	gates := make([]*protocol.GateDefinition, 0, len(r.gates))
	for _, gate := range r.gates {
		gateCopy := *gate
		gates = append(gates, &gateCopy)
	}

	return gates
}

// GetRequired returns all required gates
func (r *Registry) GetRequired() []*protocol.GateDefinition {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	required := make([]*protocol.GateDefinition, 0)
	for _, gate := range r.gates {
		if gate.Required {
			gateCopy := *gate
			required = append(required, &gateCopy)
		}
	}

	return required
}

// GetOptional returns all optional gates
func (r *Registry) GetOptional() []*protocol.GateDefinition {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	optional := make([]*protocol.GateDefinition, 0)
	for _, gate := range r.gates {
		if !gate.Required {
			gateCopy := *gate
			optional = append(optional, &gateCopy)
		}
	}

	return optional
}

// Count returns the total number of registered gates
func (r *Registry) Count() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	return len(r.gates)
}

// Clear removes all gates from the registry
func (r *Registry) Clear() {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.gates = make(map[string]*protocol.GateDefinition)
}

// LoadFromProtocol loads gates from a protocol
func (r *Registry) LoadFromProtocol(p *protocol.Protocol) error {
	for i := range p.Gates {
		gate := &p.Gates[i]
		if err := r.Register(gate); err != nil {
			return fmt.Errorf("failed to register gate %s: %w", gate.ID, err)
		}
	}

	return nil
}

// ValidateGates validates that all referenced gates exist
func (r *Registry) ValidateGates(gateIDs []string) error {
	for _, gateID := range gateIDs {
		if !r.Exists(gateID) {
			return fmt.Errorf("gate not found: %s", gateID)
		}
	}
	return nil
}

// GetGatesForStage returns gate definitions for a stage
func (r *Registry) GetGatesForStage(stage *protocol.Stage) ([]*protocol.GateDefinition, error) {
	gates := make([]*protocol.GateDefinition, 0, len(stage.Gates))

	for _, gateID := range stage.Gates {
		gate, err := r.Get(gateID)
		if err != nil {
			return nil, fmt.Errorf("failed to get gate %s for stage %s: %w", gateID, stage.ID, err)
		}
		gates = append(gates, gate)
	}

	return gates, nil
}

// Clone creates a deep copy of the registry
func (r *Registry) Clone() *Registry {
	newRegistry := NewRegistry()

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for _, gate := range r.gates {
		gateCopy := *gate
		newRegistry.Register(&gateCopy)
	}

	return newRegistry
}

// Merge merges another registry into this one
func (r *Registry) Merge(other *Registry) error {
	other.mutex.RLock()
	defer other.mutex.RUnlock()

	for _, gate := range other.gates {
		if r.Exists(gate.ID) {
			return fmt.Errorf("gate collision: %s", gate.ID)
		}
		if err := r.Register(gate); err != nil {
			return err
		}
	}

	return nil
}

// GateFilter is a function for filtering gates
type GateFilter func(*protocol.GateDefinition) bool

// Filter returns gates matching a filter function
func (r *Registry) Filter(filter GateFilter) []*protocol.GateDefinition {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	filtered := make([]*protocol.GateDefinition, 0)
	for _, gate := range r.gates {
		if filter(gate) {
			gateCopy := *gate
			filtered = append(filtered, &gateCopy)
		}
	}

	return filtered
}

// Update updates a gate definition
func (r *Registry) Update(gateID string, updates func(*protocol.GateDefinition)) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	gate, exists := r.gates[gateID]
	if !exists {
		return fmt.Errorf("gate not found: %s", gateID)
	}

	updates(gate)
	return nil
}

// GetByCommand returns gates that match a command
func (r *Registry) GetByCommand(command string) []*protocol.GateDefinition {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	matches := make([]*protocol.GateDefinition, 0)
	for _, gate := range r.gates {
		if gate.Command == command {
			gateCopy := *gate
			matches = append(matches, &gateCopy)
		}
	}

	return matches
}

// GetDefaultRegistry returns a registry with common gates
func GetDefaultRegistry() *Registry {
	registry := NewRegistry()

	// Common gates
	registry.Register(&protocol.GateDefinition{
		ID:       "go-vet",
		Name:     "Go Vet",
		Command:  "go vet ./...",
		Required: true,
		Timeout:  60,
	})

	registry.Register(&protocol.GateDefinition{
		ID:       "go-test",
		Name:     "Go Test",
		Command:  "go test ./...",
		Required: true,
		Timeout:  300,
	})

	registry.Register(&protocol.GateDefinition{
		ID:       "go-fmt",
		Name:     "Go Format",
		Command:  "test -z $(gofmt -l .)",
		Required: false,
		Timeout:  30,
	})

	return registry
}
