package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestGetIsolationLevel(t *testing.T) {
	tests := []struct {
		name    string
		level   string
		want    IsolationLevel
		wantErr bool
	}{
		{
			name:    "basic level",
			level:   "basic",
			want:    IsolationLevelBasic,
			wantErr: false,
		},
		{
			name:    "strict level",
			level:   "strict",
			want:    IsolationLevelStrict,
			wantErr: false,
		},
		{
			name:    "full level",
			level:   "full",
			want:    IsolationLevelFull,
			wantErr: false,
		},
		{
			name:    "invalid level",
			level:   "invalid",
			want:    "",
			wantErr: true,
		},
		{
			name:    "empty level",
			level:   "",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetIsolationLevel(tt.level)
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetIsolationLevel() error = nil, wantErr %v", tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("GetIsolationLevel() unexpected error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("GetIsolationLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateIsolationConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *IsolationConfig
		wantErr bool
	}{
		{
			name: "valid basic config",
			config: &IsolationConfig{
				Level:          IsolationLevelBasic,
				PreventNetwork: false,
				PreventWrite:   false,
			},
			wantErr: false,
		},
		{
			name: "valid strict config with limits",
			config: &IsolationConfig{
				Level:          IsolationLevelStrict,
				PreventNetwork: true,
				PreventWrite:   false,
				ResourceLimits: &ResourceLimits{
					MaxMemoryMB:   1024,
					MaxCPUPercent: 50,
					MaxDiskMB:     10000,
				},
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "invalid isolation level",
			config: &IsolationConfig{
				Level: IsolationLevel("invalid"),
			},
			wantErr: true,
		},
		{
			name: "negative memory limit",
			config: &IsolationConfig{
				Level: IsolationLevelBasic,
				ResourceLimits: &ResourceLimits{
					MaxMemoryMB: -1,
				},
			},
			wantErr: true,
		},
		{
			name: "negative cpu limit",
			config: &IsolationConfig{
				Level: IsolationLevelBasic,
				ResourceLimits: &ResourceLimits{
					MaxCPUPercent: -1,
				},
			},
			wantErr: true,
		},
		{
			name: "cpu limit over 100",
			config: &IsolationConfig{
				Level: IsolationLevelBasic,
				ResourceLimits: &ResourceLimits{
					MaxCPUPercent: 101,
				},
			},
			wantErr: true,
		},
		{
			name: "negative disk limit",
			config: &IsolationConfig{
				Level: IsolationLevelBasic,
				ResourceLimits: &ResourceLimits{
					MaxDiskMB: -1,
				},
			},
			wantErr: true,
		},
		{
			name: "valid zero limits",
			config: &IsolationConfig{
				Level: IsolationLevelBasic,
				ResourceLimits: &ResourceLimits{
					MaxMemoryMB:   0,
					MaxCPUPercent: 0,
					MaxDiskMB:     0,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIsolationConfig(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateIsolationConfig() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateIsolationConfig() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestGetIsolationGuarantees(t *testing.T) {
	tests := []struct {
		name     string
		level    IsolationLevel
		expected int // minimum number of guarantees
	}{
		{
			name:     "basic level",
			level:    IsolationLevelBasic,
			expected: 3,
		},
		{
			name:     "strict level",
			level:    IsolationLevelStrict,
			expected: 5,
		},
		{
			name:     "full level",
			level:    IsolationLevelFull,
			expected: 7,
		},
		{
			name:     "invalid level",
			level:    IsolationLevel("invalid"),
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guarantees := GetIsolationGuarantees(tt.level)
			if len(guarantees) < tt.expected {
				t.Errorf("GetIsolationGuarantees(%v) returned %d guarantees, expected at least %d", tt.level, len(guarantees), tt.expected)
			}
		})
	}
}

func TestIsolationLevels(t *testing.T) {
	// Test that each isolation level has the expected guarantees
	basicGuarantees := GetIsolationGuarantees(IsolationLevelBasic)
	if len(basicGuarantees) == 0 {
		t.Error("expected basic isolation level to have guarantees")
	}

	strictGuarantees := GetIsolationGuarantees(IsolationLevelStrict)
	if len(strictGuarantees) <= len(basicGuarantees) {
		t.Error("expected strict isolation level to have more guarantees than basic")
	}

	fullGuarantees := GetIsolationGuarantees(IsolationLevelFull)
	if len(fullGuarantees) <= len(strictGuarantees) {
		t.Error("expected full isolation level to have more guarantees than strict")
	}

	// Check specific guarantees exist
	for _, g := range basicGuarantees {
		if g == "" {
			t.Error("expected non-empty guarantee in basic level")
		}
	}
}

func TestResourceLimits(t *testing.T) {
	config := &IsolationConfig{
		Level:          IsolationLevelStrict,
		PreventNetwork: true,
		PreventWrite:   false,
		ResourceLimits: &ResourceLimits{
			MaxMemoryMB:   2048,
			MaxCPUPercent: 75,
			MaxDiskMB:     50000,
		},
	}

	if config.ResourceLimits.MaxMemoryMB != 2048 {
		t.Errorf("expected max memory 2048, got %d", config.ResourceLimits.MaxMemoryMB)
	}

	if config.ResourceLimits.MaxCPUPercent != 75 {
		t.Errorf("expected max CPU 75%%, got %d%%", config.ResourceLimits.MaxCPUPercent)
	}

	if config.ResourceLimits.MaxDiskMB != 50000 {
		t.Errorf("expected max disk 50000, got %d", config.ResourceLimits.MaxDiskMB)
	}
}

func TestEnforceIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()

	m := NewManager(repoDir, worktreeBase)

	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}

	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)

	// Test enforce basic isolation
	isolationConfig := &IsolationConfig{
		Level:          IsolationLevelBasic,
		PreventNetwork: false,
		PreventWrite:   false,
	}

	guarantee, err := m.EnforceIsolation(worktree.ID, isolationConfig)
	if err != nil {
		t.Fatalf("EnforceIsolation() error = %v", err)
	}

	if guarantee == nil {
		t.Fatal("expected non-nil guarantee")
	}

	if guarantee.WorktreeID != worktree.ID {
		t.Errorf("expected worktree ID %s, got %s", worktree.ID, guarantee.WorktreeID)
	}

	if guarantee.Level != IsolationLevelBasic {
		t.Errorf("expected level %v, got %v", IsolationLevelBasic, guarantee.Level)
	}

	if !guarantee.Enforced {
		t.Error("expected Enforced to be true")
	}

	if len(guarantee.Guarantees) == 0 {
		t.Error("expected non-empty guarantees")
	}
}

func TestEnforceIsolationStrict(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()

	m := NewManager(repoDir, worktreeBase)

	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}

	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)

	// Test enforce strict isolation with resource limits
	isolationConfig := &IsolationConfig{
		Level:          IsolationLevelStrict,
		PreventNetwork: true,
		PreventWrite:   false,
		ResourceLimits: &ResourceLimits{
			MaxMemoryMB:   1024,
			MaxCPUPercent: 50,
			MaxDiskMB:     10000,
		},
	}

	guarantee, err := m.EnforceIsolation(worktree.ID, isolationConfig)
	if err != nil {
		t.Fatalf("EnforceIsolation() error = %v", err)
	}

	if guarantee == nil {
		t.Fatal("expected non-nil guarantee")
	}

	if guarantee.Level != IsolationLevelStrict {
		t.Errorf("expected level %v, got %v", IsolationLevelStrict, guarantee.Level)
	}

	// Should have more guarantees for strict level
	basicGuarantees := GetIsolationGuarantees(IsolationLevelBasic)
	if len(guarantee.Guarantees) <= len(basicGuarantees) {
		t.Error("expected strict level to have more guarantees than basic")
	}
}

func TestEnforceIsolationFull(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()

	m := NewManager(repoDir, worktreeBase)

	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}

	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)

	// Test enforce full isolation
	isolationConfig := &IsolationConfig{
		Level:          IsolationLevelFull,
		PreventNetwork: true,
		PreventWrite:   true,
		ResourceLimits: &ResourceLimits{
			MaxMemoryMB:   2048,
			MaxCPUPercent: 100,
			MaxDiskMB:     50000,
		},
	}

	guarantee, err := m.EnforceIsolation(worktree.ID, isolationConfig)
	if err != nil {
		t.Fatalf("EnforceIsolation() error = %v", err)
	}

	if guarantee == nil {
		t.Fatal("expected non-nil guarantee")
	}

	if guarantee.Level != IsolationLevelFull {
		t.Errorf("expected level %v, got %v", IsolationLevelFull, guarantee.Level)
	}

	// Should have most guarantees for full level
	strictGuarantees := GetIsolationGuarantees(IsolationLevelStrict)
	if len(guarantee.Guarantees) <= len(strictGuarantees) {
		t.Error("expected full level to have more guarantees than strict")
	}
}

func TestEnforceIsolationInvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)

	// Test with nil config
	_, err := m.EnforceIsolation("test", nil)
	if err == nil {
		t.Error("expected error for nil config")
	}

	// Test with invalid level
	isolationConfig := &IsolationConfig{
		Level: IsolationLevel("invalid"),
	}
	_, err = m.EnforceIsolation("test", isolationConfig)
	if err == nil {
		t.Error("expected error for invalid isolation level")
	}
}

func TestEnforceIsolationInvalidWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)

	isolationConfig := &IsolationConfig{
		Level: IsolationLevelBasic,
	}

	_, err := m.EnforceIsolation("non-existent", isolationConfig)
	if err == nil {
		t.Error("expected error for non-existent worktree")
	}
}

func TestCleanupOnFailure(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()

	m := NewManager(repoDir, worktreeBase)

	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}

	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}

	// Manually add to tracking (simulate failure scenario)
	m.mu.Lock()
	m.worktrees[worktree.ID] = worktree
	m.mu.Unlock()

	// Test cleanup worktree
	err = m.CleanupWorktree(worktree.ID)
	if err != nil {
		t.Fatalf("CleanupWorktree() error = %v", err)
	}

	// Verify worktree directory is removed
	if _, err := os.Stat(worktree.Path); !os.IsNotExist(err) {
		t.Error("expected worktree directory to be removed")
	}

	// Verify worktree is removed from tracking
	_, err = m.Get(worktree.ID)
	if err == nil {
		t.Error("expected worktree to be removed from tracking")
	}
}

func TestCleanupWorktreeInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)

	// Test cleanup non-existent worktree
	err := m.CleanupWorktree("non-existent")
	if err == nil {
		t.Error("expected error for non-existent worktree")
	}
}

func TestVerifyIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()

	m := NewManager(repoDir, worktreeBase)

	config := WorktreeConfig{
		TaskID:     "test-task",
		BaseBranch: "main",
		WorkingDir: repoDir,
	}

	worktree, err := m.Create("test-task", config)
	if err != nil {
		t.Fatalf("failed to create worktree: %v", err)
	}
	defer m.Delete(worktree.ID)

	// Test verify isolation
	verification, err := m.VerifyIsolation(worktree.ID)
	if err != nil {
		t.Fatalf("VerifyIsolation() error = %v", err)
	}

	if verification == nil {
		t.Fatal("expected non-nil verification")
	}

	if verification.WorktreeID != worktree.ID {
		t.Errorf("expected worktree ID %s, got %s", worktree.ID, verification.WorktreeID)
	}

	if !verification.Passed {
		t.Error("expected verification to pass")
	}

	// Check that required checks exist
	if verification.Checks == nil {
		t.Fatal("expected non-nil checks map")
	}

	if !verification.Checks["directory_exists"] {
		t.Error("expected directory_exists check to pass")
	}

	if !verification.Checks["separate_from_repo"] {
		t.Error("expected separate_from_repo check to pass")
	}
}

func TestVerifyIsolationInvalidWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)

	_, err := m.VerifyIsolation("non-existent")
	if err == nil {
		t.Error("expected error for non-existent worktree")
	}
}

func TestVerifyIsolationFailed(t *testing.T) {
	tmpDir := t.TempDir()
	m := NewManager(tmpDir, tmpDir)

	// Add a worktree to tracking without creating directory
	worktree := &Worktree{
		ID:   "test-wt",
		Path: filepath.Join(tmpDir, "non-existent-path"),
	}
	m.mu.Lock()
	m.worktrees[worktree.ID] = worktree
	m.mu.Unlock()

	verification, err := m.VerifyIsolation(worktree.ID)
	if err != nil {
		t.Fatalf("VerifyIsolation() error = %v", err)
	}

	if verification.Passed {
		t.Error("expected verification to fail for non-existent directory")
	}

	if verification.Checks["directory_exists"] {
		t.Error("expected directory_exists check to fail")
	}
}

func TestConcurrentWorktrees(t *testing.T) {
	tmpDir := t.TempDir()
	repoDir := filepath.Join(tmpDir, "repo")
	worktreeBase := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(repoDir, 0755)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoDir
	if err := cmd.Run(); err != nil {
		t.Skipf("git not available: %v", err)
	}

	// Configure git
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(repoDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoDir
	cmd.Run()
	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = repoDir
	cmd.Run()

	m := NewManager(repoDir, worktreeBase)

	// Create multiple worktrees concurrently
	configs := []WorktreeConfig{
		{TaskID: "task-1", BaseBranch: "main", WorkingDir: repoDir},
		{TaskID: "task-2", BaseBranch: "main", WorkingDir: repoDir},
		{TaskID: "task-3", BaseBranch: "main", WorkingDir: repoDir},
	}

	worktrees := make([]*Worktree, len(configs))
	for i, config := range configs {
		wt, err := m.Create(config.TaskID, config)
		if err != nil {
			t.Fatalf("failed to create worktree %d: %v", i, err)
		}
		worktrees[i] = wt
	}

	// Verify all worktrees exist
	if len(m.List()) != len(configs) {
		t.Errorf("expected %d worktrees, got %d", len(configs), len(m.List()))
	}

	// Clean up all worktrees
	for _, wt := range worktrees {
		if err := m.Delete(wt.ID); err != nil {
			t.Errorf("failed to delete worktree %s: %v", wt.ID, err)
		}
	}

	// Verify all worktrees are removed
	if len(m.List()) != 0 {
		t.Errorf("expected 0 worktrees after cleanup, got %d", len(m.List()))
	}
}

func TestValidateWorktreeConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *WorktreeConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &WorktreeConfig{
				TaskID:     "task-1",
				BaseBranch: "main",
				WorkingDir: "/tmp/repo",
			},
			wantErr: false,
		},
		{
			name: "missing task ID",
			config: &WorktreeConfig{
				TaskID:     "",
				BaseBranch: "main",
				WorkingDir: "/tmp/repo",
			},
			wantErr: true,
		},
		{
			name: "missing working dir",
			config: &WorktreeConfig{
				TaskID:     "task-1",
				BaseBranch: "main",
				WorkingDir: "",
			},
			wantErr: true,
		},
		{
			name: "valid with isolation level",
			config: &WorktreeConfig{
				TaskID:         "task-1",
				BaseBranch:     "main",
				WorkingDir:     "/tmp/repo",
				IsolationLevel: "strict",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateWorktreeConfig(tt.config)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateWorktreeConfig() error = nil, wantErr %v", tt.wantErr)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateWorktreeConfig() unexpected error = %v", err)
				}
			}
		})
	}
}

func TestIsolationGuarantee(t *testing.T) {
	guarantee := &IsolationGuarantee{
		WorktreeID: "wt-1",
		Level:      IsolationLevelStrict,
		Guarantees: []string{"network_isolation", "process_isolation"},
		Enforced:   true,
	}

	if guarantee.WorktreeID != "wt-1" {
		t.Errorf("expected worktree ID wt-1, got %s", guarantee.WorktreeID)
	}

	if guarantee.Level != IsolationLevelStrict {
		t.Errorf("expected level strict, got %v", guarantee.Level)
	}

	if len(guarantee.Guarantees) != 2 {
		t.Errorf("expected 2 guarantees, got %d", len(guarantee.Guarantees))
	}

	if !guarantee.Enforced {
		t.Error("expected Enforced to be true")
	}
}

func TestIsolationVerification(t *testing.T) {
	verification := &IsolationVerification{
		WorktreeID: "wt-1",
		Checks: map[string]bool{
			"directory_exists":   true,
			"separate_from_repo": true,
			"permissions_ok":     false,
		},
		Passed: false,
	}

	if verification.WorktreeID != "wt-1" {
		t.Errorf("expected worktree ID wt-1, got %s", verification.WorktreeID)
	}

	if verification.Passed {
		t.Error("expected Passed to be false")
	}

	if len(verification.Checks) != 3 {
		t.Errorf("expected 3 checks, got %d", len(verification.Checks))
	}

	if !verification.Checks["directory_exists"] {
		t.Error("expected directory_exists to be true")
	}

	if verification.Checks["permissions_ok"] {
		t.Error("expected permissions_ok to be false")
	}
}
