package protocol

import (
	"fmt"
	"sort"
)

// Resolver handles dependency resolution and DAG operations
type Resolver struct {
	protocol *Protocol
	adjList  map[string][]string // adjacency list for dependency graph
	inDegree map[string]int      // in-degree count for each stage
}

// NewResolver creates a new resolver for a protocol
func NewResolver(protocol *Protocol) *Resolver {
	r := &Resolver{
		protocol: protocol,
		adjList:  make(map[string][]string),
		inDegree: make(map[string]int),
	}
	r.buildGraph()
	return r
}

// buildGraph constructs the dependency graph
func (r *Resolver) buildGraph() {
	// Initialize stages
	for _, stage := range r.protocol.Stages {
		r.inDegree[stage.ID] = 0
		r.adjList[stage.ID] = []string{}
	}

	// Build adjacency list and calculate in-degrees
	for _, stage := range r.protocol.Stages {
		for _, dep := range stage.DependsOn {
			// Add edge: dep -> stage
			r.adjList[dep] = append(r.adjList[dep], stage.ID)
			r.inDegree[stage.ID]++
		}
	}
}

// TopologicalSort returns stages in dependency order using Kahn's algorithm
func (r *Resolver) TopologicalSort() ([]string, error) {
	// Make a copy of in-degrees
	inDegreeCopy := make(map[string]int)
	for k, v := range r.inDegree {
		inDegreeCopy[k] = v
	}

	// Find all stages with no dependencies
	queue := []string{}
	for id, degree := range inDegreeCopy {
		if degree == 0 {
			queue = append(queue, id)
		}
	}

	// Sort for deterministic order
	sort.Strings(queue)

	result := []string{}
	visited := make(map[string]bool)

	for len(queue) > 0 {
		// Pop from front
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true
		result = append(result, current)

		// Update in-degrees of neighbors
		for _, neighbor := range r.adjList[current] {
			inDegreeCopy[neighbor]--
			if inDegreeCopy[neighbor] == 0 && !visited[neighbor] {
				queue = append(queue, neighbor)
			}
		}

		// Keep queue sorted for determinism
		if len(queue) > 1 {
			sort.Strings(queue)
		}
	}

	// Check if all stages were visited
	if len(result) != len(r.protocol.Stages) {
		return nil, fmt.Errorf("dependency cycle detected in protocol stages")
	}

	return result, nil
}

// GetDependencies returns all dependencies (direct and transitive) for a stage
func (r *Resolver) GetDependencies(stageID string) ([]string, error) {
	// Verify stage exists
	found := false
	for _, stage := range r.protocol.Stages {
		if stage.ID == stageID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("stage not found: %s", stageID)
	}

	// BFS to find all dependencies
	visited := make(map[string]bool)
	queue := []string{stageID}
	result := []string{}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		stage, _ := r.protocol.GetStage(current)
		if stage == nil {
			continue
		}

		for _, dep := range stage.DependsOn {
			if !visited[dep] {
				visited[dep] = true
				result = append(result, dep)
				queue = append(queue, dep)
			}
		}
	}

	return result, nil
}

// GetDependents returns all stages that depend on the given stage
func (r *Resolver) GetDependents(stageID string) ([]string, error) {
	// Verify stage exists
	found := false
	for _, stage := range r.protocol.Stages {
		if stage.ID == stageID {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("stage not found: %s", stageID)
	}

	// Use adjacency list to find dependents
	dependents := make([]string, 0)
	for _, dependent := range r.adjList[stageID] {
		dependents = append(dependents, dependent)
	}

	return dependents, nil
}

// IsReady returns true if all dependencies are satisfied
func (r *Resolver) IsReady(stageID string, completedStages map[string]bool) (bool, error) {
	stage, err := r.protocol.GetStage(stageID)
	if err != nil {
		return false, err
	}

	for _, dep := range stage.DependsOn {
		if !completedStages[dep] {
			return false, nil
		}
	}

	return true, nil
}

// GetReadyStages returns all stages whose dependencies are satisfied
func (r *Resolver) GetReadyStages(completedStages map[string]bool) ([]string, error) {
	ready := []string{}

	for _, stage := range r.protocol.Stages {
		if completedStages[stage.ID] {
			continue // Already completed
		}

		isReady, err := r.IsReady(stage.ID, completedStages)
		if err != nil {
			return nil, err
		}

		if isReady {
			ready = append(ready, stage.ID)
		}
	}

	return ready, nil
}

// HasCycle checks if the dependency graph has a cycle
func (r *Resolver) HasCycle() bool {
	_, err := r.TopologicalSort()
	return err != nil
}

// ValidateDAG validates that the protocol forms a valid DAG
func (r *Resolver) ValidateDAG() error {
	if r.HasCycle() {
		return fmt.Errorf("protocol contains a dependency cycle")
	}
	return nil
}
