package subagent

import (
	"testing"
	"time"
)

func TestDefaultLimits(t *testing.T) {
	limits := DefaultLimits()
	
	if limits.MaxTurns != 50 {
		t.Errorf("expected max turns 50, got %d", limits.MaxTurns)
	}
	
	if limits.MaxTokens != 100000 {
		t.Errorf("expected max tokens 100000, got %d", limits.MaxTokens)
	}
	
	if limits.Timeout != 30*time.Minute {
		t.Errorf("expected timeout 30m, got %v", limits.Timeout)
	}
	
	if limits.MemoryMB != 0 {
		t.Errorf("expected memory limit 0 (unlimited), got %d", limits.MemoryMB)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name   string
		limits Limits
		wantErr bool
	}{
		{
			name:    "valid limits",
			limits:  Limits{MaxTurns: 10, MaxTokens: 100, Timeout: time.Second, MemoryMB: 0},
			wantErr: false,
		},
		{
			name:    "zero turns",
			limits:  Limits{MaxTurns: 0, MaxTokens: 100, Timeout: time.Second},
			wantErr: true,
		},
		{
			name:    "negative turns",
			limits:  Limits{MaxTurns: -1, MaxTokens: 100, Timeout: time.Second},
			wantErr: true,
		},
		{
			name:    "zero tokens",
			limits:  Limits{MaxTurns: 10, MaxTokens: 0, Timeout: time.Second},
			wantErr: true,
		},
		{
			name:    "negative tokens",
			limits:  Limits{MaxTurns: 10, MaxTokens: -1, Timeout: time.Second},
			wantErr: true,
		},
		{
			name:    "zero timeout",
			limits:  Limits{MaxTurns: 10, MaxTokens: 100, Timeout: 0},
			wantErr: true,
		},
		{
			name:    "negative memory",
			limits:  Limits{MaxTurns: 10, MaxTokens: 100, Timeout: time.Second, MemoryMB: -1},
			wantErr: true,
		},
		{
			name:    "positive memory",
			limits:  Limits{MaxTurns: 10, MaxTokens: 100, Timeout: time.Second, MemoryMB: 1024},
			wantErr: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.limits.Validate()
			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMerge(t *testing.T) {
	base := Limits{
		MaxTurns:  50,
		MaxTokens: 100000,
		Timeout:   30 * time.Minute,
		MemoryMB:  0,
	}
	
	// Empty other - should keep base
	merged := base.Merge(Limits{})
	if merged.MaxTurns != 50 {
		t.Errorf("expected max turns 50, got %d", merged.MaxTurns)
	}
	if merged.MaxTokens != 100000 {
		t.Errorf("expected max tokens 100000, got %d", merged.MaxTokens)
	}
	
	// Override some values
	other := Limits{
		MaxTurns: 100,
		MemoryMB: 512,
	}
	merged = base.Merge(other)
	if merged.MaxTurns != 100 {
		t.Errorf("expected max turns 100, got %d", merged.MaxTurns)
	}
	if merged.MaxTokens != 100000 {
		t.Errorf("expected max tokens 100000 (unchanged), got %d", merged.MaxTokens)
	}
	if merged.MemoryMB != 512 {
		t.Errorf("expected memory 512, got %d", merged.MemoryMB)
	}
}

func TestIsExceeded(t *testing.T) {
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   time.Minute,
		MemoryMB:  512,
	}
	
	tests := []struct {
		name      string
		usage     Usage
		exceeded  bool
		reason    string
	}{
		{
			name:     "within limits",
			usage:    Usage{TurnsUsed: 5, TokensUsed: 500, Duration: 30 * time.Second, MemoryUsed: 256},
			exceeded: false,
			reason:   "",
		},
		{
			name:     "exceeds turns",
			usage:    Usage{TurnsUsed: 11, TokensUsed: 500, Duration: 30 * time.Second, MemoryUsed: 256},
			exceeded: true,
			reason:   "turn",
		},
		{
			name:     "exceeds tokens",
			usage:    Usage{TurnsUsed: 5, TokensUsed: 1001, Duration: 30 * time.Second, MemoryUsed: 256},
			exceeded: true,
			reason:   "token",
		},
		{
			name:     "exceeds timeout",
			usage:    Usage{TurnsUsed: 5, TokensUsed: 500, Duration: 61 * time.Second, MemoryUsed: 256},
			exceeded: true,
			reason:   "timeout",
		},
		{
			name:     "exceeds memory",
			usage:    Usage{TurnsUsed: 5, TokensUsed: 500, Duration: 30 * time.Second, MemoryUsed: 513},
			exceeded: true,
			reason:   "memory",
		},
		{
			name:     "exactly at limit",
			usage:    Usage{TurnsUsed: 10, TokensUsed: 1000, Duration: time.Minute, MemoryUsed: 512},
			exceeded: false,
			reason:   "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exceeded, reason := limits.IsExceeded(tt.usage)
			if exceeded != tt.exceeded {
				t.Errorf("expected exceeded=%v, got %v", tt.exceeded, exceeded)
			}
			if tt.exceeded && tt.reason != "" && !containsSubstring(reason, tt.reason) {
				t.Errorf("expected reason to contain %q, got %q", tt.reason, reason)
			}
		})
	}
}

func TestEnforcer(t *testing.T) {
	limits := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   time.Minute,
		MemoryMB:  512,
	}
	
	enforcer := NewEnforcer(limits)
	
	// Check initial state
	if enforcer.GetLimits().MaxTurns != 10 {
		t.Errorf("expected max turns 10, got %d", enforcer.GetLimits().MaxTurns)
	}
	
	// Check turn
	exceeded, _ := enforcer.CheckTurn()
	if exceeded {
		t.Error("should not be exceeded after first turn")
	}
	
	if enforcer.GetUsage().TurnsUsed != 1 {
		t.Errorf("expected 1 turn used, got %d", enforcer.GetUsage().TurnsUsed)
	}
	
	// Record tokens
	exceeded, _ = enforcer.RecordTokens(500)
	if exceeded {
		t.Error("should not be exceeded after 500 tokens")
	}
	
	if enforcer.GetUsage().TokensUsed != 500 {
		t.Errorf("expected 500 tokens used, got %d", enforcer.GetUsage().TokensUsed)
	}
	
	// Check remaining
	remaining := enforcer.Remaining()
	if remaining.TurnsUsed != 9 {
		t.Errorf("expected 9 turns remaining, got %d", remaining.TurnsUsed)
	}
	if remaining.TokensUsed != 500 {
		t.Errorf("expected 500 tokens remaining, got %d", remaining.TokensUsed)
	}
	
	// Check progress
	progress := enforcer.Progress()
	// Turn progress: 1/10 = 10%
	// Token progress: 500/1000 = 50%
	// Should return max (50%)
	if progress != 50.0 {
		t.Errorf("expected progress 50%%, got %f%%", progress)
	}
}

func TestEnforcerUpdateLimits(t *testing.T) {
	limits := Limits{MaxTurns: 10, MaxTokens: 1000, Timeout: time.Minute}
	enforcer := NewEnforcer(limits)
	
	newLimits := Limits{MaxTurns: 20, MaxTokens: 2000, Timeout: 2 * time.Minute}
	enforcer.UpdateLimits(newLimits)
	
	if enforcer.GetLimits().MaxTurns != 20 {
		t.Errorf("expected max turns 20, got %d", enforcer.GetLimits().MaxTurns)
	}
	if enforcer.GetLimits().MaxTokens != 2000 {
		t.Errorf("expected max tokens 2000, got %d", enforcer.GetLimits().MaxTokens)
	}
}

func TestEnforcerExceedTurns(t *testing.T) {
	limits := Limits{MaxTurns: 2, MaxTokens: 1000, Timeout: time.Minute}
	enforcer := NewEnforcer(limits)
	
	// First turn - OK
	exceeded, _ := enforcer.CheckTurn()
	if exceeded {
		t.Error("should not be exceeded after first turn")
	}
	
	// Second turn - OK
	exceeded, _ = enforcer.CheckTurn()
	if exceeded {
		t.Error("should not be exceeded after second turn")
	}
	
	// Third turn - Exceeds
	exceeded, reason := enforcer.CheckTurn()
	if !exceeded {
		t.Error("should be exceeded after third turn")
	}
	if !containsSubstring(reason, "turn") {
		t.Errorf("expected reason to contain 'turn', got %s", reason)
	}
}

func containsSubstring(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestLimitsExceeded(t *testing.T) {
	tests := []struct {
		name     string
		limits   Limits
		usage    Usage
		exceeded bool
		reason   string
	}{
		{
			name:     "turns exceeded",
			limits:   Limits{MaxTurns: 5, MaxTokens: 1000, Timeout: time.Minute},
			usage:    Usage{TurnsUsed: 6, TokensUsed: 100, Duration: time.Second},
			exceeded: true,
			reason:   "turn",
		},
		{
			name:     "tokens exceeded",
			limits:   Limits{MaxTurns: 10, MaxTokens: 100, Timeout: time.Minute},
			usage:    Usage{TurnsUsed: 5, TokensUsed: 101, Duration: time.Second},
			exceeded: true,
			reason:   "token",
		},
		{
			name:     "timeout exceeded",
			limits:   Limits{MaxTurns: 10, MaxTokens: 1000, Timeout: time.Second},
			usage:    Usage{TurnsUsed: 5, TokensUsed: 100, Duration: 2 * time.Second},
			exceeded: true,
			reason:   "timeout",
		},
		{
			name:     "memory exceeded",
			limits:   Limits{MaxTurns: 10, MaxTokens: 1000, Timeout: time.Minute, MemoryMB: 512},
			usage:    Usage{TurnsUsed: 5, TokensUsed: 100, Duration: time.Second, MemoryUsed: 513},
			exceeded: true,
			reason:   "memory",
		},
		{
			name:     "within all limits",
			limits:   Limits{MaxTurns: 10, MaxTokens: 1000, Timeout: time.Minute, MemoryMB: 512},
			usage:    Usage{TurnsUsed: 5, TokensUsed: 500, Duration: 30 * time.Second, MemoryUsed: 256},
			exceeded: false,
			reason:   "",
		},
		{
			name:     "exactly at turns limit",
			limits:   Limits{MaxTurns: 5, MaxTokens: 1000, Timeout: time.Minute},
			usage:    Usage{TurnsUsed: 5, TokensUsed: 100, Duration: time.Second},
			exceeded: false,
			reason:   "",
		},
		{
			name:     "exactly at tokens limit",
			limits:   Limits{MaxTurns: 10, MaxTokens: 100, Timeout: time.Minute},
			usage:    Usage{TurnsUsed: 5, TokensUsed: 100, Duration: time.Second},
			exceeded: false,
			reason:   "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exceeded, reason := tt.limits.IsExceeded(tt.usage)
			if exceeded != tt.exceeded {
				t.Errorf("IsExceeded() exceeded = %v, expected %v", exceeded, tt.exceeded)
			}
			if tt.exceeded && tt.reason != "" && !containsSubstring(reason, tt.reason) {
				t.Errorf("IsExceeded() reason = %q, expected to contain %q", reason, tt.reason)
			}
		})
	}
}

func TestLimitsZeroValues(t *testing.T) {
	// Test that zero limits are treated as unlimited (not exceeded)
	limits := Limits{
		MaxTurns:  0, // Unlimited
		MaxTokens: 0, // Unlimited
		Timeout:   0, // Unlimited
		MemoryMB:  0, // Unlimited
	}
	
	usage := Usage{
		TurnsUsed:  1000000,
		TokensUsed: 1000000,
		Duration:   24 * time.Hour,
		MemoryUsed: 1000000,
	}
	
	exceeded, _ := limits.IsExceeded(usage)
	if exceeded {
		t.Error("expected zero limits to be treated as unlimited")
	}
}

func TestLimitsNegative(t *testing.T) {
	tests := []struct {
		name   string
		limits Limits
	}{
		{
			name:   "negative turns",
			limits: Limits{MaxTurns: -1, MaxTokens: 100, Timeout: time.Second},
		},
		{
			name:   "negative tokens",
			limits: Limits{MaxTurns: 10, MaxTokens: -1, Timeout: time.Second},
		},
		{
			name:   "negative timeout",
			limits: Limits{MaxTurns: 10, MaxTokens: 100, Timeout: -time.Second},
		},
		{
			name:   "negative memory",
			limits: Limits{MaxTurns: 10, MaxTokens: 100, Timeout: time.Second, MemoryMB: -1},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.limits.Validate()
			if err == nil {
				t.Error("expected error for negative limits")
			}
		})
	}
}

func TestLimitsUpdate(t *testing.T) {
	// Test dynamic limit updates
	enforcer := NewEnforcer(Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   time.Minute,
	})
	
	// Record some usage
	enforcer.CheckTurn()
	enforcer.CheckTurn()
	enforcer.RecordTokens(500)
	
	// Update limits to be more restrictive
	enforcer.UpdateLimits(Limits{
		MaxTurns:  1,  // Now exceeded
		MaxTokens: 1000,
		Timeout:   time.Minute,
	})
	
	// Should now be exceeded
	exceeded, reason := enforcer.CheckTurn()
	if !exceeded {
		t.Error("expected to be exceeded after updating limits")
	}
	if !containsSubstring(reason, "turn") {
		t.Errorf("expected reason to contain 'turn', got %s", reason)
	}
}

func TestEnforcerProgressEdgeCases(t *testing.T) {
	// Test progress with zero limits (unlimited)
	enforcer := NewEnforcer(Limits{
		MaxTurns:  0,
		MaxTokens: 0,
		Timeout:   0,
	})
	
	progress := enforcer.Progress()
	if progress != 0 {
		t.Errorf("expected 0%% progress with unlimited limits, got %f%%", progress)
	}
	
	// Test progress at 100%
	enforcer2 := NewEnforcer(Limits{
		MaxTurns:  10,
		MaxTokens: 100,
		Timeout:   time.Minute,
	})
	
	// Use exactly the limit
	enforcer2.usage = Usage{
		TurnsUsed:  10,
		TokensUsed: 100,
		Duration:   time.Minute,
	}
	
	progress = enforcer2.Progress()
	if progress != 100.0 {
		t.Errorf("expected 100%% progress at limit, got %f%%", progress)
	}
}

func TestEnforcerRemaining(t *testing.T) {
	enforcer := NewEnforcer(Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   time.Minute,
		MemoryMB:  512,
	})
	
	// Record some usage
	enforcer.CheckTurn()
	enforcer.CheckTurn()
	enforcer.CheckTurn()
	enforcer.RecordTokens(300)
	
	remaining := enforcer.Remaining()
	
	if remaining.TurnsUsed != 7 {
		t.Errorf("expected 7 turns remaining, got %d", remaining.TurnsUsed)
	}
	
	if remaining.TokensUsed != 700 {
		t.Errorf("expected 700 tokens remaining, got %d", remaining.TokensUsed)
	}
}

func TestEnforcerRecordTokensExceeded(t *testing.T) {
	enforcer := NewEnforcer(Limits{
		MaxTurns:  10,
		MaxTokens: 100,
		Timeout:   time.Minute,
	})
	
	// Record tokens within limit
	exceeded, _ := enforcer.RecordTokens(50)
	if exceeded {
		t.Error("should not be exceeded after 50 tokens")
	}
	
	// Record more tokens to exceed limit
	exceeded, reason := enforcer.RecordTokens(60)
	if !exceeded {
		t.Error("should be exceeded after 110 tokens with limit of 100")
	}
	if !containsSubstring(reason, "token") {
		t.Errorf("expected reason to contain 'token', got %s", reason)
	}
}

func TestLimitsMergeEdgeCases(t *testing.T) {
	// Test merge with all fields overridden
	base := Limits{
		MaxTurns:  10,
		MaxTokens: 1000,
		Timeout:   time.Minute,
		MemoryMB:  0,
	}
	
	other := Limits{
		MaxTurns:  20,
		MaxTokens: 2000,
		Timeout:   2 * time.Minute,
		MemoryMB:  512,
	}
	
	merged := base.Merge(other)
	
	if merged.MaxTurns != 20 {
		t.Errorf("expected MaxTurns 20, got %d", merged.MaxTurns)
	}
	if merged.MaxTokens != 2000 {
		t.Errorf("expected MaxTokens 2000, got %d", merged.MaxTokens)
	}
	if merged.Timeout != 2*time.Minute {
		t.Errorf("expected Timeout 2m, got %v", merged.Timeout)
	}
	if merged.MemoryMB != 512 {
		t.Errorf("expected MemoryMB 512, got %d", merged.MemoryMB)
	}
	
	// Test merge with partial override
	partial := Limits{
		MaxTurns: 30,
	}
	
	merged = base.Merge(partial)
	if merged.MaxTurns != 30 {
		t.Errorf("expected MaxTurns 30, got %d", merged.MaxTurns)
	}
	if merged.MaxTokens != 1000 {
		t.Errorf("expected MaxTokens unchanged (1000), got %d", merged.MaxTokens)
	}
	if merged.Timeout != time.Minute {
		t.Errorf("expected Timeout unchanged (1m), got %v", merged.Timeout)
	}
}

func TestUsageStruct(t *testing.T) {
	usage := Usage{
		TurnsUsed:  10,
		TokensUsed: 1000,
		Duration:   5 * time.Minute,
		MemoryUsed: 512,
	}
	
	if usage.TurnsUsed != 10 {
		t.Errorf("expected TurnsUsed 10, got %d", usage.TurnsUsed)
	}
	if usage.TokensUsed != 1000 {
		t.Errorf("expected TokensUsed 1000, got %d", usage.TokensUsed)
	}
	if usage.Duration != 5*time.Minute {
		t.Errorf("expected Duration 5m, got %v", usage.Duration)
	}
	if usage.MemoryUsed != 512 {
		t.Errorf("expected MemoryUsed 512, got %d", usage.MemoryUsed)
	}
}
