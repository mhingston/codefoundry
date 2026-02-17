package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResolver(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
			{ID: "c", Name: "C", DependsOn: []string{"b"}},
		},
	}
	
	resolver := NewResolver(protocol)
	require.NotNil(t, resolver)
	assert.NotNil(t, resolver.adjList)
	assert.NotNil(t, resolver.inDegree)
}

func TestResolver_TopologicalSort_Linear(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
			{ID: "c", Name: "C", DependsOn: []string{"b"}},
		},
	}
	
	resolver := NewResolver(protocol)
	order, err := resolver.TopologicalSort()
	
	require.NoError(t, err)
	assert.Equal(t, []string{"a", "b", "c"}, order)
}

func TestResolver_TopologicalSort_Branching(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
			{ID: "c", Name: "C", DependsOn: []string{"a"}},
			{ID: "d", Name: "D", DependsOn: []string{"b", "c"}},
		},
	}
	
	resolver := NewResolver(protocol)
	order, err := resolver.TopologicalSort()
	
	require.NoError(t, err)
	assert.Equal(t, "a", order[0])
	assert.Equal(t, "d", order[3])
	// b and c can be in either order
	assert.Contains(t, order, "b")
	assert.Contains(t, order, "c")
}

func TestResolver_TopologicalSort_Cycle(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A", DependsOn: []string{"c"}},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
			{ID: "c", Name: "C", DependsOn: []string{"b"}},
		},
	}
	
	resolver := NewResolver(protocol)
	_, err := resolver.TopologicalSort()
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}

func TestResolver_TopologicalSort_SingleNode(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
		},
	}
	
	resolver := NewResolver(protocol)
	order, err := resolver.TopologicalSort()
	
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, order)
}

func TestResolver_TopologicalSort_Empty(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{},
	}
	
	resolver := NewResolver(protocol)
	order, err := resolver.TopologicalSort()
	
	require.NoError(t, err)
	assert.Empty(t, order)
}

func TestResolver_GetDependencies(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
			{ID: "c", Name: "C", DependsOn: []string{"b"}},
			{ID: "d", Name: "D", DependsOn: []string{"a", "b"}},
		},
	}
	
	resolver := NewResolver(protocol)
	
	// Get dependencies for c (should be [b, a])
	deps, err := resolver.GetDependencies("c")
	require.NoError(t, err)
	assert.Equal(t, []string{"b", "a"}, deps)
	
	// Get dependencies for a (should be empty)
	deps, err = resolver.GetDependencies("a")
	require.NoError(t, err)
	assert.Empty(t, deps)
	
	// Get dependencies for d (should be [a, b])
	deps, err = resolver.GetDependencies("d")
	require.NoError(t, err)
	assert.Equal(t, 2, len(deps))
	assert.Contains(t, deps, "a")
	assert.Contains(t, deps, "b")
}

func TestResolver_GetDependencies_InvalidStage(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
		},
	}
	
	resolver := NewResolver(protocol)
	_, err := resolver.GetDependencies("nonexistent")
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestResolver_GetDependents(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
			{ID: "c", Name: "C", DependsOn: []string{"a"}},
			{ID: "d", Name: "D", DependsOn: []string{"b"}},
		},
	}
	
	resolver := NewResolver(protocol)
	
	// Get dependents of a (should be [b, c])
	dependents, err := resolver.GetDependents("a")
	require.NoError(t, err)
	assert.Equal(t, 2, len(dependents))
	assert.Contains(t, dependents, "b")
	assert.Contains(t, dependents, "c")
	
	// Get dependents of d (should be empty)
	dependents, err = resolver.GetDependents("d")
	require.NoError(t, err)
	assert.Empty(t, dependents)
}

func TestResolver_IsReady(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
			{ID: "c", Name: "C", DependsOn: []string{"a", "b"}},
		},
	}
	
	resolver := NewResolver(protocol)
	
	// a has no dependencies, should always be ready
	ready, err := resolver.IsReady("a", map[string]bool{})
	require.NoError(t, err)
	assert.True(t, ready)
	
	// b requires a
	ready, err = resolver.IsReady("b", map[string]bool{})
	require.NoError(t, err)
	assert.False(t, ready)
	
	ready, err = resolver.IsReady("b", map[string]bool{"a": true})
	require.NoError(t, err)
	assert.True(t, ready)
	
	// c requires both a and b
	ready, err = resolver.IsReady("c", map[string]bool{"a": true})
	require.NoError(t, err)
	assert.False(t, ready)
	
	ready, err = resolver.IsReady("c", map[string]bool{"a": true, "b": true})
	require.NoError(t, err)
	assert.True(t, ready)
}

func TestResolver_GetReadyStages(t *testing.T) {
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
			{ID: "c", Name: "C", DependsOn: []string{"a"}},
			{ID: "d", Name: "D", DependsOn: []string{"b", "c"}},
		},
	}
	
	resolver := NewResolver(protocol)
	
	// Initially only a is ready
	ready, err := resolver.GetReadyStages(map[string]bool{})
	require.NoError(t, err)
	assert.Equal(t, []string{"a"}, ready)
	
	// After a completes, b and c are ready
	ready, err = resolver.GetReadyStages(map[string]bool{"a": true})
	require.NoError(t, err)
	assert.Equal(t, 2, len(ready))
	assert.Contains(t, ready, "b")
	assert.Contains(t, ready, "c")
	
	// After a, b, c complete, d is ready
	ready, err = resolver.GetReadyStages(map[string]bool{"a": true, "b": true, "c": true})
	require.NoError(t, err)
	assert.Equal(t, []string{"d"}, ready)
}

func TestResolver_HasCycle(t *testing.T) {
	// No cycle
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
		},
	}
	
	resolver := NewResolver(protocol)
	assert.False(t, resolver.HasCycle())
	
	// With cycle
	protocol = &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A", DependsOn: []string{"b"}},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
		},
	}
	
	resolver = NewResolver(protocol)
	assert.True(t, resolver.HasCycle())
}

func TestResolver_ValidateDAG(t *testing.T) {
	// Valid DAG
	protocol := &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
		},
	}
	
	resolver := NewResolver(protocol)
	assert.NoError(t, resolver.ValidateDAG())
	
	// Invalid DAG (cycle)
	protocol = &Protocol{
		Stages: []Stage{
			{ID: "a", Name: "A", DependsOn: []string{"b"}},
			{ID: "b", Name: "B", DependsOn: []string{"a"}},
		},
	}
	
	resolver = NewResolver(protocol)
	err := resolver.ValidateDAG()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cycle")
}
