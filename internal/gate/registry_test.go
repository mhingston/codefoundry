package gate

import (
	"testing"

	"github.com/mhingston/codefoundry/internal/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	assert.NotNil(t, registry)
	assert.NotNil(t, registry.gates)
	assert.Empty(t, registry.List())
}

func TestNewRegistryFromProtocol(t *testing.T) {
	p := &protocol.Protocol{
		Gates: []protocol.GateDefinition{
			{ID: "gate1", Name: "Gate 1", Command: "echo 1"},
			{ID: "gate2", Name: "Gate 2", Command: "echo 2"},
		},
	}

	registry := NewRegistryFromProtocol(p)

	assert.NotNil(t, registry)
	assert.Len(t, registry.List(), 2)
	assert.True(t, registry.Exists("gate1"))
	assert.True(t, registry.Exists("gate2"))
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry()

	gate := &protocol.GateDefinition{
		ID:      "test-gate",
		Name:    "Test Gate",
		Command: "echo test",
	}

	err := registry.Register(gate)
	require.NoError(t, err)

	// Should be able to retrieve
	retrieved, err := registry.Get("test-gate")
	require.NoError(t, err)
	assert.Equal(t, "Test Gate", retrieved.Name)

	// Default timeout should be applied
	assert.Equal(t, 300, retrieved.Timeout)
}

func TestRegistry_Register_EmptyID(t *testing.T) {
	registry := NewRegistry()

	gate := &protocol.GateDefinition{
		ID:      "",
		Command: "echo test",
	}

	err := registry.Register(gate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ID cannot be empty")
}

func TestRegistry_Register_Duplicate(t *testing.T) {
	registry := NewRegistry()

	gate := &protocol.GateDefinition{ID: "dup", Command: "echo"}
	require.NoError(t, registry.Register(gate))

	err := registry.Register(gate)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_Unregister(t *testing.T) {
	registry := NewRegistry()

	gate := &protocol.GateDefinition{ID: "test", Command: "echo"}
	require.NoError(t, registry.Register(gate))
	assert.True(t, registry.Exists("test"))

	err := registry.Unregister("test")
	require.NoError(t, err)
	assert.False(t, registry.Exists("test"))
}

func TestRegistry_Unregister_NotFound(t *testing.T) {
	registry := NewRegistry()

	err := registry.Unregister("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry()

	original := &protocol.GateDefinition{
		ID:       "test",
		Name:     "Test",
		Command:  "echo test",
		Required: true,
	}
	require.NoError(t, registry.Register(original))

	retrieved, err := registry.Get("test")
	require.NoError(t, err)
	assert.Equal(t, "Test", retrieved.Name)
	assert.Equal(t, "echo test", retrieved.Command)

	// Modify retrieved should not affect original (copy returned)
	retrieved.Name = "Modified"
	retrieved2, _ := registry.Get("test")
	assert.Equal(t, "Test", retrieved2.Name)
}

func TestRegistry_Get_NotFound(t *testing.T) {
	registry := NewRegistry()

	_, err := registry.Get("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_MustGet(t *testing.T) {
	registry := NewRegistry()

	gate := &protocol.GateDefinition{ID: "test", Command: "echo"}
	require.NoError(t, registry.Register(gate))

	retrieved := registry.MustGet("test")
	assert.Equal(t, "test", retrieved.ID)
}

func TestRegistry_MustGet_Panic(t *testing.T) {
	registry := NewRegistry()

	assert.Panics(t, func() {
		registry.MustGet("nonexistent")
	})
}

func TestRegistry_Exists(t *testing.T) {
	registry := NewRegistry()

	assert.False(t, registry.Exists("test"))

	registry.Register(&protocol.GateDefinition{ID: "test", Command: "echo"})

	assert.True(t, registry.Exists("test"))
	assert.False(t, registry.Exists("other"))
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	// Empty
	assert.Empty(t, registry.List())

	// Add gates
	registry.Register(&protocol.GateDefinition{ID: "b", Command: "echo"})
	registry.Register(&protocol.GateDefinition{ID: "a", Command: "echo"})

	list := registry.List()
	assert.Len(t, list, 2)
	assert.Contains(t, list, "a")
	assert.Contains(t, list, "b")
}

func TestRegistry_ListAll(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{ID: "test", Command: "echo"})

	all := registry.ListAll()
	assert.Len(t, all, 1)
	assert.Equal(t, "test", all[0].ID)
}

func TestRegistry_GetRequired(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{ID: "req", Command: "echo", Required: true})
	registry.Register(&protocol.GateDefinition{ID: "opt", Command: "echo", Required: false})

	required := registry.GetRequired()
	assert.Len(t, required, 1)
	assert.Equal(t, "req", required[0].ID)
}

func TestRegistry_GetOptional(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{ID: "req", Command: "echo", Required: true})
	registry.Register(&protocol.GateDefinition{ID: "opt", Command: "echo", Required: false})

	optional := registry.GetOptional()
	assert.Len(t, optional, 1)
	assert.Equal(t, "opt", optional[0].ID)
}

func TestRegistry_Count(t *testing.T) {
	registry := NewRegistry()

	assert.Equal(t, 0, registry.Count())

	registry.Register(&protocol.GateDefinition{ID: "a", Command: "echo"})
	assert.Equal(t, 1, registry.Count())

	registry.Register(&protocol.GateDefinition{ID: "b", Command: "echo"})
	assert.Equal(t, 2, registry.Count())
}

func TestRegistry_Clear(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{ID: "test", Command: "echo"})
	assert.Equal(t, 1, registry.Count())

	registry.Clear()
	assert.Equal(t, 0, registry.Count())
	assert.False(t, registry.Exists("test"))
}

func TestRegistry_LoadFromProtocol(t *testing.T) {
	registry := NewRegistry()

	p := &protocol.Protocol{
		Gates: []protocol.GateDefinition{
			{ID: "gate1", Command: "echo 1"},
			{ID: "gate2", Command: "echo 2"},
		},
	}

	err := registry.LoadFromProtocol(p)
	require.NoError(t, err)
	assert.Equal(t, 2, registry.Count())
}

func TestRegistry_LoadFromProtocol_Duplicate(t *testing.T) {
	registry := NewRegistry()

	// First registration
	registry.Register(&protocol.GateDefinition{ID: "dup", Command: "echo"})

	p := &protocol.Protocol{
		Gates: []protocol.GateDefinition{
			{ID: "dup", Command: "echo"},
		},
	}

	err := registry.LoadFromProtocol(p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestRegistry_ValidateGates(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{ID: "gate1", Command: "echo"})
	registry.Register(&protocol.GateDefinition{ID: "gate2", Command: "echo"})

	err := registry.ValidateGates([]string{"gate1", "gate2"})
	assert.NoError(t, err)

	err = registry.ValidateGates([]string{"gate1", "nonexistent"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found: nonexistent")
}

func TestRegistry_GetGatesForStage(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{ID: "test", Command: "echo"})

	stage := &protocol.Stage{
		ID:    "verify",
		Gates: []string{"test"},
	}

	gates, err := registry.GetGatesForStage(stage)
	require.NoError(t, err)
	assert.Len(t, gates, 1)
	assert.Equal(t, "test", gates[0].ID)
}

func TestRegistry_GetGatesForStage_NotFound(t *testing.T) {
	registry := NewRegistry()

	stage := &protocol.Stage{
		ID:    "verify",
		Gates: []string{"nonexistent"},
	}

	_, err := registry.GetGatesForStage(stage)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_Clone(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{ID: "test", Command: "echo"})

	cloned := registry.Clone()
	assert.Equal(t, 1, cloned.Count())
	assert.True(t, cloned.Exists("test"))

	// Modifying clone should not affect original
	cloned.Register(&protocol.GateDefinition{ID: "new", Command: "echo"})
	assert.Equal(t, 2, cloned.Count())
	assert.Equal(t, 1, registry.Count())
}

func TestRegistry_Merge(t *testing.T) {
	registry1 := NewRegistry()
	registry2 := NewRegistry()

	registry1.Register(&protocol.GateDefinition{ID: "a", Command: "echo"})
	registry2.Register(&protocol.GateDefinition{ID: "b", Command: "echo"})

	err := registry1.Merge(registry2)
	require.NoError(t, err)
	assert.Equal(t, 2, registry1.Count())
	assert.True(t, registry1.Exists("a"))
	assert.True(t, registry1.Exists("b"))
}

func TestRegistry_Merge_Collision(t *testing.T) {
	registry1 := NewRegistry()
	registry2 := NewRegistry()

	registry1.Register(&protocol.GateDefinition{ID: "dup", Command: "echo"})
	registry2.Register(&protocol.GateDefinition{ID: "dup", Command: "echo"})

	err := registry1.Merge(registry2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "collision")
}

func TestRegistry_Filter(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{ID: "a", Command: "echo", Required: true})
	registry.Register(&protocol.GateDefinition{ID: "b", Command: "echo", Required: false})

	// Filter for required gates
	filtered := registry.Filter(func(g *protocol.GateDefinition) bool {
		return g.Required
	})

	assert.Len(t, filtered, 1)
	assert.Equal(t, "a", filtered[0].ID)
}

func TestRegistry_Update(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{
		ID:      "test",
		Name:    "Original",
		Command: "echo",
	})

	err := registry.Update("test", func(g *protocol.GateDefinition) {
		g.Name = "Updated"
	})
	require.NoError(t, err)

	updated, _ := registry.Get("test")
	assert.Equal(t, "Updated", updated.Name)
}

func TestRegistry_Update_NotFound(t *testing.T) {
	registry := NewRegistry()

	err := registry.Update("nonexistent", func(g *protocol.GateDefinition) {})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegistry_GetByCommand(t *testing.T) {
	registry := NewRegistry()

	registry.Register(&protocol.GateDefinition{ID: "a", Command: "go test ./..."})
	registry.Register(&protocol.GateDefinition{ID: "b", Command: "go test ./..."})
	registry.Register(&protocol.GateDefinition{ID: "c", Command: "go vet ./..."})

	matches := registry.GetByCommand("go test ./...")
	assert.Len(t, matches, 2)
}

func TestGetDefaultRegistry(t *testing.T) {
	registry := GetDefaultRegistry()

	assert.NotNil(t, registry)
	assert.True(t, registry.Exists("go-vet"))
	assert.True(t, registry.Exists("go-test"))
	assert.True(t, registry.Exists("go-fmt"))
}
