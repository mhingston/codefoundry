package protocol

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Task represents a single task in a task_prompt stage
type Task struct {
	ID                 string            `yaml:"id" json:"id"`
	Title              string            `yaml:"title" json:"title"`
	Description        string            `yaml:"description" json:"description"`
	FilesToModify      []string          `yaml:"files_to_modify,omitempty" json:"files_to_modify,omitempty"`
	AcceptanceCriteria []string          `yaml:"acceptance_criteria,omitempty" json:"acceptance_criteria,omitempty"`
	Dependencies       []string          `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	EstimatedEffort    string            `yaml:"estimated_effort,omitempty" json:"estimated_effort,omitempty"`
	TemplateVars       map[string]string `yaml:"template_vars,omitempty" json:"template_vars,omitempty"`
}

// TasksFile represents the structure of a tasks.yaml file
type TasksFile struct {
	SchemaVersion string `yaml:"schema_version" json:"schema_version"`
	Tasks         []Task `yaml:"tasks" json:"tasks"`
}

// TaskResult represents the result of a task execution
type TaskResult struct {
	TaskID       string                 `json:"task_id"`
	Success      bool                   `json:"success"`
	Output       string                 `json:"output,omitempty"`
	FilesChanged []string               `json:"files_changed,omitempty"`
	TurnsUsed    int                    `json:"turns_used"`
	TokensUsed   int                    `json:"tokens_used"`
	DurationMs   int64                  `json:"duration_ms"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TaskDAG represents a task dependency graph
type TaskDAG struct {
	tasks        map[string]*Task
	dependencies map[string][]string
	dependents   map[string][]string
}

// NewTaskDAG creates a new task DAG
func NewTaskDAG(tasks []Task) *TaskDAG {
	dag := &TaskDAG{
		tasks:        make(map[string]*Task),
		dependencies: make(map[string][]string),
		dependents:   make(map[string][]string),
	}

	for i := range tasks {
		task := &tasks[i]
		dag.tasks[task.ID] = task
		dag.dependencies[task.ID] = task.Dependencies

		for _, dep := range task.Dependencies {
			dag.dependents[dep] = append(dag.dependents[dep], task.ID)
		}
	}

	return dag
}

// GetTask returns a task by ID
func (d *TaskDAG) GetTask(id string) (*Task, error) {
	task, ok := d.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", id)
	}
	return task, nil
}

// GetDependencies returns dependencies for a task
func (d *TaskDAG) GetDependencies(id string) []string {
	return d.dependencies[id]
}

// GetDependents returns tasks that depend on a task
func (d *TaskDAG) GetDependents(id string) []string {
	return d.dependents[id]
}

// IsReady checks if a task is ready to execute (all dependencies complete)
func (d *TaskDAG) IsReady(id string, completed map[string]bool) bool {
	deps := d.dependencies[id]
	for _, dep := range deps {
		if !completed[dep] {
			return false
		}
	}
	return true
}

// GetReadyTasks returns tasks that are ready to execute
func (d *TaskDAG) GetReadyTasks(completed map[string]bool) []*Task {
	var ready []*Task
	for id, task := range d.tasks {
		if !completed[id] && d.IsReady(id, completed) {
			ready = append(ready, task)
		}
	}
	return ready
}

// TopologicalSort returns tasks in topological order (by wave)
func (d *TaskDAG) TopologicalSort() ([][]string, error) {
	// Build in-degree map
	inDegree := make(map[string]int)
	for id := range d.tasks {
		inDegree[id] = len(d.dependencies[id])
	}

	var waves [][]string
	completed := make(map[string]bool)

	for len(completed) < len(d.tasks) {
		// Find tasks with no remaining dependencies
		var wave []string
		for id, degree := range inDegree {
			if !completed[id] && degree == 0 {
				wave = append(wave, id)
			}
		}

		if len(wave) == 0 {
			return nil, fmt.Errorf("cycle detected in task dependencies")
		}

		waves = append(waves, wave)

		// Mark tasks as completed and reduce in-degree of dependents
		for _, id := range wave {
			completed[id] = true
			for _, dependent := range d.dependents[id] {
				inDegree[dependent]--
			}
		}
	}

	return waves, nil
}

// Validate checks task DAG for cycles and unknown dependencies
func (d *TaskDAG) Validate() error {
	// Check all dependencies exist
	for id, deps := range d.dependencies {
		for _, dep := range deps {
			if _, ok := d.tasks[dep]; !ok {
				return fmt.Errorf("task '%s' depends on unknown task: %s", id, dep)
			}
		}
	}

	// Check for cycles using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycle func(string) bool
	hasCycle = func(id string) bool {
		visited[id] = true
		recStack[id] = true

		for _, dep := range d.dependents[id] {
			if !visited[dep] {
				if hasCycle(dep) {
					return true
				}
			} else if recStack[dep] {
				return true
			}
		}

		recStack[id] = false
		return false
	}

	for id := range d.tasks {
		if !visited[id] {
			if hasCycle(id) {
				return fmt.Errorf("cycle detected in task dependencies")
			}
		}
	}

	return nil
}

// LoadTasks loads tasks from a tasks.yaml file
func LoadTasks(path string) (*TasksFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read tasks file: %w", err)
	}

	var tasksFile TasksFile
	if err := yaml.Unmarshal(data, &tasksFile); err != nil {
		return nil, fmt.Errorf("failed to parse tasks YAML: %w", err)
	}

	// Set default schema version
	if tasksFile.SchemaVersion == "" {
		tasksFile.SchemaVersion = "codefoundry_tasks.v1"
	}

	return &tasksFile, nil
}

// SaveTasks saves tasks to a tasks.yaml file
func SaveTasks(path string, tasksFile *TasksFile) error {
	data, err := yaml.Marshal(tasksFile)
	if err != nil {
		return fmt.Errorf("failed to marshal tasks: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write tasks file: %w", err)
	}

	return nil
}

// ValidateTask validates a single task
func ValidateTask(task *Task) error {
	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}

	if task.Title == "" {
		return fmt.Errorf("task '%s': title is required", task.ID)
	}

	if task.Description == "" {
		return fmt.Errorf("task '%s': description is required", task.ID)
	}

	// Validate estimated_effort if provided
	if task.EstimatedEffort != "" {
		validEfforts := map[string]bool{
			"small":  true,
			"medium": true,
			"large":  true,
		}
		if !validEfforts[task.EstimatedEffort] {
			return fmt.Errorf("task '%s': invalid estimated_effort '%s' (valid: small, medium, large)", task.ID, task.EstimatedEffort)
		}
	}

	return nil
}

// CreateTasksFile creates a new tasks file
func CreateTasksFile(tasks []Task) *TasksFile {
	return &TasksFile{
		SchemaVersion: "codefoundry_tasks.v1",
		Tasks:         tasks,
	}
}

// AddTask adds a task to the tasks file
func (t *TasksFile) AddTask(task Task) error {
	// Check for duplicate ID
	for _, existing := range t.Tasks {
		if existing.ID == task.ID {
			return fmt.Errorf("task with ID '%s' already exists", task.ID)
		}
	}

	// Validate task
	if err := ValidateTask(&task); err != nil {
		return err
	}

	t.Tasks = append(t.Tasks, task)
	return nil
}

// GetTask returns a task by ID
func (t *TasksFile) GetTask(id string) (*Task, error) {
	for i := range t.Tasks {
		if t.Tasks[i].ID == id {
			return &t.Tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task not found: %s", id)
}

// RemoveTask removes a task by ID
func (t *TasksFile) RemoveTask(id string) error {
	for i, task := range t.Tasks {
		if task.ID == id {
			t.Tasks = append(t.Tasks[:i], t.Tasks[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("task not found: %s", id)
}

// GetDAG returns the task DAG for this tasks file
func (t *TasksFile) GetDAG() *TaskDAG {
	return NewTaskDAG(t.Tasks)
}

// Validate validates all tasks in the file
func (t *TasksFile) Validate() error {
	// Validate each task
	for i := range t.Tasks {
		if err := ValidateTask(&t.Tasks[i]); err != nil {
			return err
		}
	}

	// Validate DAG
	dag := t.GetDAG()
	return dag.Validate()
}

// EstimatedTotalEffort returns the total estimated effort for all tasks
func (t *TasksFile) EstimatedTotalEffort() int {
	effortValues := map[string]int{
		"small":  1,
		"medium": 3,
		"large":  5,
	}

	total := 0
	for _, task := range t.Tasks {
		if value, ok := effortValues[task.EstimatedEffort]; ok {
			total += value
		} else {
			total += 2 // default to medium
		}
	}

	return total
}

// GetTasksByEffort returns tasks filtered by effort level
func (t *TasksFile) GetTasksByEffort(effort string) []Task {
	var result []Task
	for _, task := range t.Tasks {
		if task.EstimatedEffort == effort {
			result = append(result, task)
		}
	}
	return result
}

// GetTasksWithoutDependencies returns tasks with no dependencies
func (t *TasksFile) GetTasksWithoutDependencies() []Task {
	var result []Task
	for _, task := range t.Tasks {
		if len(task.Dependencies) == 0 {
			result = append(result, task)
		}
	}
	return result
}
