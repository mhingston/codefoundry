package protocol

import (
	"path/filepath"
	"testing"
)

func TestNewTaskDAG(t *testing.T) {
	tasks := []Task{
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2", Dependencies: []string{"task-1"}},
		{ID: "task-3", Title: "Task 3", Dependencies: []string{"task-1", "task-2"}},
	}
	
	dag := NewTaskDAG(tasks)
	
	if len(dag.tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(dag.tasks))
	}
	
	// Check dependencies
	if len(dag.dependencies["task-2"]) != 1 {
		t.Errorf("expected task-2 to have 1 dependency, got %d", len(dag.dependencies["task-2"]))
	}
	
	// Check dependents
	if len(dag.dependents["task-1"]) != 2 {
		t.Errorf("expected task-1 to have 2 dependents, got %d", len(dag.dependents["task-1"]))
	}
}

func TestGetTask(t *testing.T) {
	tasks := []Task{
		{ID: "task-1", Title: "Task 1"},
	}
	
	dag := NewTaskDAG(tasks)
	
	task, err := dag.GetTask("task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if task.Title != "Task 1" {
		t.Errorf("expected title 'Task 1', got '%s'", task.Title)
	}
	
	_, err = dag.GetTask("non-existent")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestIsReady(t *testing.T) {
	tasks := []Task{
		{ID: "task-1", Title: "Task 1"},
		{ID: "task-2", Title: "Task 2", Dependencies: []string{"task-1"}},
	}
	
	dag := NewTaskDAG(tasks)
	
	// task-1 is ready with no completed tasks (no dependencies)
	completed := make(map[string]bool)
	if !dag.IsReady("task-1", completed) {
		t.Error("expected task-1 to be ready (no dependencies)")
	}
	
	// task-2 is not ready without task-1 completed
	if dag.IsReady("task-2", completed) {
		t.Error("expected task-2 to not be ready")
	}
	
	// Mark task-1 completed
	completed["task-1"] = true
	
	// Now task-2 should be ready
	if !dag.IsReady("task-2", completed) {
		t.Error("expected task-2 to be ready after task-1 completed")
	}
}

func TestTopologicalSort(t *testing.T) {
	tests := []struct {
		name     string
		tasks    []Task
		expected int // number of waves
		wantErr  bool
	}{
		{
			name: "linear chain",
			tasks: []Task{
				{ID: "task-1", Title: "Task 1"},
				{ID: "task-2", Title: "Task 2", Dependencies: []string{"task-1"}},
				{ID: "task-3", Title: "Task 3", Dependencies: []string{"task-2"}},
			},
			expected: 3,
			wantErr:  false,
		},
		{
			name: "parallel tasks",
			tasks: []Task{
				{ID: "task-1", Title: "Task 1"},
				{ID: "task-2", Title: "Task 2"},
				{ID: "task-3", Title: "Task 3", Dependencies: []string{"task-1", "task-2"}},
			},
			expected: 2,
			wantErr:  false,
		},
		{
			name: "complex dependencies",
			tasks: []Task{
				{ID: "task-1", Title: "Task 1"},
				{ID: "task-2", Title: "Task 2"},
				{ID: "task-3", Title: "Task 3", Dependencies: []string{"task-1"}},
				{ID: "task-4", Title: "Task 4", Dependencies: []string{"task-2", "task-3"}},
			},
			expected: 3,
			wantErr:  false,
		},
		{
			name: "single task",
			tasks: []Task{
				{ID: "task-1", Title: "Task 1"},
			},
			expected: 1,
			wantErr:  false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag := NewTaskDAG(tt.tasks)
			waves, err := dag.TopologicalSort()
			
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			if len(waves) != tt.expected {
				t.Errorf("expected %d waves, got %d", tt.expected, len(waves))
			}
		})
	}
}

func TestTopologicalSortCycle(t *testing.T) {
	tasks := []Task{
		{ID: "task-1", Title: "Task 1", Dependencies: []string{"task-3"}},
		{ID: "task-2", Title: "Task 2", Dependencies: []string{"task-1"}},
		{ID: "task-3", Title: "Task 3", Dependencies: []string{"task-2"}},
	}
	
	dag := NewTaskDAG(tasks)
	_, err := dag.TopologicalSort()
	
	if err == nil {
		t.Error("expected error for cyclic dependencies")
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		tasks   []Task
		wantErr bool
	}{
		{
			name: "valid DAG",
			tasks: []Task{
				{ID: "task-1", Title: "Task 1"},
				{ID: "task-2", Title: "Task 2", Dependencies: []string{"task-1"}},
			},
			wantErr: false,
		},
		{
			name: "unknown dependency",
			tasks: []Task{
				{ID: "task-1", Title: "Task 1", Dependencies: []string{"non-existent"}},
			},
			wantErr: true,
		},
		{
			name: "cyclic dependencies",
			tasks: []Task{
				{ID: "task-1", Title: "Task 1", Dependencies: []string{"task-2"}},
				{ID: "task-2", Title: "Task 2", Dependencies: []string{"task-1"}},
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dag := NewTaskDAG(tt.tasks)
			err := dag.Validate()
			
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestValidateTask(t *testing.T) {
	tests := []struct {
		name    string
		task    Task
		wantErr bool
	}{
		{
			name:    "valid task",
			task:    Task{ID: "task-1", Title: "Task 1", Description: "Description"},
			wantErr: false,
		},
		{
			name:    "missing ID",
			task:    Task{Title: "Task 1", Description: "Description"},
			wantErr: true,
		},
		{
			name:    "missing title",
			task:    Task{ID: "task-1", Description: "Description"},
			wantErr: true,
		},
		{
			name:    "missing description",
			task:    Task{ID: "task-1", Title: "Task 1"},
			wantErr: true,
		},
		{
			name:    "invalid effort",
			task:    Task{ID: "task-1", Title: "Task 1", Description: "Description", EstimatedEffort: "huge"},
			wantErr: true,
		},
		{
			name:    "valid effort small",
			task:    Task{ID: "task-1", Title: "Task 1", Description: "Description", EstimatedEffort: "small"},
			wantErr: false,
		},
		{
			name:    "valid effort medium",
			task:    Task{ID: "task-1", Title: "Task 1", Description: "Description", EstimatedEffort: "medium"},
			wantErr: false,
		},
		{
			name:    "valid effort large",
			task:    Task{ID: "task-1", Title: "Task 1", Description: "Description", EstimatedEffort: "large"},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTask(&tt.task)
			
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestTasksFile(t *testing.T) {
	tasksFile := &TasksFile{
		SchemaVersion: "codefoundry_tasks.v1",
		Tasks: []Task{
			{ID: "task-1", Title: "Task 1", Description: "Description 1"},
			{ID: "task-2", Title: "Task 2", Description: "Description 2"},
		},
	}
	
	// Test GetTask
	task, err := tasksFile.GetTask("task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if task.Title != "Task 1" {
		t.Errorf("expected title 'Task 1', got '%s'", task.Title)
	}
	
	_, err = tasksFile.GetTask("non-existent")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
	
	// Test AddTask
	newTask := Task{ID: "task-3", Title: "Task 3", Description: "Description 3"}
	if err := tasksFile.AddTask(newTask); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if len(tasksFile.Tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasksFile.Tasks))
	}
	
	// Test duplicate ID
	err = tasksFile.AddTask(Task{ID: "task-1", Title: "Duplicate", Description: "Description"})
	if err == nil {
		t.Error("expected error for duplicate task ID")
	}
	
	// Test RemoveTask
	if err := tasksFile.RemoveTask("task-2"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	
	if len(tasksFile.Tasks) != 2 {
		t.Errorf("expected 2 tasks after removal, got %d", len(tasksFile.Tasks))
	}
	
	// Test non-existent removal
	err = tasksFile.RemoveTask("non-existent")
	if err == nil {
		t.Error("expected error for non-existent task")
	}
}

func TestTasksFileValidation(t *testing.T) {
	tests := []struct {
		name    string
		tasks   []Task
		wantErr bool
	}{
		{
			name: "valid tasks",
			tasks: []Task{
				{ID: "task-1", Title: "Task 1", Description: "Description"},
				{ID: "task-2", Title: "Task 2", Description: "Description", Dependencies: []string{"task-1"}},
			},
			wantErr: false,
		},
		{
			name: "invalid task",
			tasks: []Task{
				{ID: "", Title: "Task 1", Description: "Description"},
			},
			wantErr: true,
		},
		{
			name: "cyclic dependencies",
			tasks: []Task{
				{ID: "task-1", Title: "Task 1", Description: "Description", Dependencies: []string{"task-2"}},
				{ID: "task-2", Title: "Task 2", Description: "Description", Dependencies: []string{"task-1"}},
			},
			wantErr: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tasksFile := &TasksFile{Tasks: tt.tasks}
			err := tasksFile.Validate()
			
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadAndSaveTasks(t *testing.T) {
	tmpDir := t.TempDir()
	tasksPath := filepath.Join(tmpDir, "tasks.yaml")
	
	tasksFile := &TasksFile{
		SchemaVersion: "codefoundry_tasks.v1",
		Tasks: []Task{
			{
				ID:                 "task-1",
				Title:              "Task 1",
				Description:        "Description 1",
				FilesToModify:      []string{"file1.go"},
				AcceptanceCriteria: []string{"Criterion 1"},
				EstimatedEffort:    "small",
			},
		},
	}
	
	// Save
	err := SaveTasks(tasksPath, tasksFile)
	if err != nil {
		t.Fatalf("failed to save tasks: %v", err)
	}
	
	// Load
	loaded, err := LoadTasks(tasksPath)
	if err != nil {
		t.Fatalf("failed to load tasks: %v", err)
	}
	
	if loaded.SchemaVersion != "codefoundry_tasks.v1" {
		t.Errorf("expected schema version codefoundry_tasks.v1, got %s", loaded.SchemaVersion)
	}
	
	if len(loaded.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(loaded.Tasks))
	}
	
	task := loaded.Tasks[0]
	if task.ID != "task-1" {
		t.Errorf("expected task ID task-1, got %s", task.ID)
	}
	if task.Title != "Task 1" {
		t.Errorf("expected title 'Task 1', got '%s'", task.Title)
	}
	if task.EstimatedEffort != "small" {
		t.Errorf("expected effort small, got %s", task.EstimatedEffort)
	}
}

func TestLoadTasksNonExistent(t *testing.T) {
	_, err := LoadTasks("/non/existent/path/tasks.yaml")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestEstimatedTotalEffort(t *testing.T) {
	tasksFile := &TasksFile{
		Tasks: []Task{
			{ID: "task-1", Title: "Task 1", Description: "Description", EstimatedEffort: "small"},   // 1
			{ID: "task-2", Title: "Task 2", Description: "Description", EstimatedEffort: "medium"}, // 3
			{ID: "task-3", Title: "Task 3", Description: "Description", EstimatedEffort: "large"},  // 5
			{ID: "task-4", Title: "Task 4", Description: "Description"},                             // default 2
		},
	}
	
	effort := tasksFile.EstimatedTotalEffort()
	if effort != 11 { // 1 + 3 + 5 + 2
		t.Errorf("expected total effort 11, got %d", effort)
	}
}

func TestGetTasksByEffort(t *testing.T) {
	tasksFile := &TasksFile{
		Tasks: []Task{
			{ID: "task-1", Title: "Task 1", Description: "Description", EstimatedEffort: "small"},
			{ID: "task-2", Title: "Task 2", Description: "Description", EstimatedEffort: "small"},
			{ID: "task-3", Title: "Task 3", Description: "Description", EstimatedEffort: "medium"},
		},
	}
	
	smallTasks := tasksFile.GetTasksByEffort("small")
	if len(smallTasks) != 2 {
		t.Errorf("expected 2 small tasks, got %d", len(smallTasks))
	}
	
	mediumTasks := tasksFile.GetTasksByEffort("medium")
	if len(mediumTasks) != 1 {
		t.Errorf("expected 1 medium task, got %d", len(mediumTasks))
	}
	
	largeTasks := tasksFile.GetTasksByEffort("large")
	if len(largeTasks) != 0 {
		t.Errorf("expected 0 large tasks, got %d", len(largeTasks))
	}
}

func TestGetTasksWithoutDependencies(t *testing.T) {
	tasksFile := &TasksFile{
		Tasks: []Task{
			{ID: "task-1", Title: "Task 1", Description: "Description"},
			{ID: "task-2", Title: "Task 2", Description: "Description", Dependencies: []string{"task-1"}},
			{ID: "task-3", Title: "Task 3", Description: "Description"},
		},
	}
	
	tasks := tasksFile.GetTasksWithoutDependencies()
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks without dependencies, got %d", len(tasks))
	}
	
	ids := make(map[string]bool)
	for _, task := range tasks {
		ids[task.ID] = true
	}
	
	if !ids["task-1"] || !ids["task-3"] {
		t.Error("expected task-1 and task-3 to be returned")
	}
}

func TestCreateTasksFile(t *testing.T) {
	tasks := []Task{
		{ID: "task-1", Title: "Task 1", Description: "Description"},
	}
	
	tasksFile := CreateTasksFile(tasks)
	
	if tasksFile.SchemaVersion != "codefoundry_tasks.v1" {
		t.Errorf("expected schema version codefoundry_tasks.v1, got %s", tasksFile.SchemaVersion)
	}
	
	if len(tasksFile.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasksFile.Tasks))
	}
}
