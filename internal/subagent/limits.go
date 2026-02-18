package subagent

import (
	"fmt"
	"time"
)

// Limits defines resource constraints for subagent execution
type Limits struct {
	MaxTurns  int           // Maximum LLM turns
	MaxTokens int           // Token budget
	Timeout   time.Duration // Execution timeout
	MemoryMB  int           // Memory limit (optional)
}

// DefaultLimits returns default resource limits
func DefaultLimits() Limits {
	return Limits{
		MaxTurns:  50,
		MaxTokens: 100000,
		Timeout:   30 * time.Minute,
		MemoryMB:  0, // No limit
	}
}

// Validate validates resource limits
func (l Limits) Validate() error {
	if l.MaxTurns <= 0 {
		return fmt.Errorf("max_turns must be positive")
	}
	if l.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive")
	}
	if l.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if l.MemoryMB < 0 {
		return fmt.Errorf("memory_mb must be non-negative")
	}
	return nil
}

// Merge combines limits, with non-zero values in other taking precedence
func (l Limits) Merge(other Limits) Limits {
	result := l

	if other.MaxTurns > 0 {
		result.MaxTurns = other.MaxTurns
	}
	if other.MaxTokens > 0 {
		result.MaxTokens = other.MaxTokens
	}
	if other.Timeout > 0 {
		result.Timeout = other.Timeout
	}
	if other.MemoryMB > 0 {
		result.MemoryMB = other.MemoryMB
	}

	return result
}

// Usage tracks actual resource consumption
type Usage struct {
	TurnsUsed  int           // Actual turns used
	TokensUsed int           // Actual tokens consumed
	Duration   time.Duration // Actual duration
	MemoryUsed int           // Peak memory in MB
}

// IsExceeded checks if usage exceeds limits
func (l Limits) IsExceeded(usage Usage) (bool, string) {
	if l.MaxTurns > 0 && usage.TurnsUsed > l.MaxTurns {
		return true, fmt.Sprintf("turn limit exceeded: %d > %d", usage.TurnsUsed, l.MaxTurns)
	}
	if l.MaxTokens > 0 && usage.TokensUsed > l.MaxTokens {
		return true, fmt.Sprintf("token limit exceeded: %d > %d", usage.TokensUsed, l.MaxTokens)
	}
	if l.Timeout > 0 && usage.Duration > l.Timeout {
		return true, fmt.Sprintf("timeout exceeded: %v > %v", usage.Duration, l.Timeout)
	}
	if l.MemoryMB > 0 && usage.MemoryUsed > l.MemoryMB {
		return true, fmt.Sprintf("memory limit exceeded: %d > %d", usage.MemoryUsed, l.MemoryMB)
	}
	return false, ""
}

// Enforcer manages resource limit enforcement
type Enforcer struct {
	limits Limits
	usage  Usage
}

// NewEnforcer creates a new limit enforcer
func NewEnforcer(limits Limits) *Enforcer {
	return &Enforcer{
		limits: limits,
		usage:  Usage{},
	}
}

// CheckTurn checks if a turn can be executed
func (e *Enforcer) CheckTurn() (bool, string) {
	e.usage.TurnsUsed++
	return e.limits.IsExceeded(e.usage)
}

// RecordTokens records token consumption
func (e *Enforcer) RecordTokens(tokens int) (bool, string) {
	e.usage.TokensUsed += tokens
	return e.limits.IsExceeded(e.usage)
}

// GetUsage returns current usage
func (e *Enforcer) GetUsage() Usage {
	return e.usage
}

// GetLimits returns current limits
func (e *Enforcer) GetLimits() Limits {
	return e.limits
}

// UpdateLimits updates limits (e.g., from hook overrides)
func (e *Enforcer) UpdateLimits(limits Limits) {
	e.limits = limits
}

// Remaining returns remaining resources
func (e *Enforcer) Remaining() Usage {
	return Usage{
		TurnsUsed:  e.limits.MaxTurns - e.usage.TurnsUsed,
		TokensUsed: e.limits.MaxTokens - e.usage.TokensUsed,
		Duration:   e.limits.Timeout - e.usage.Duration,
		MemoryUsed: e.limits.MemoryMB - e.usage.MemoryUsed,
	}
}

// Progress returns completion percentage
func (e *Enforcer) Progress() float64 {
	turnsProgress := 0.0
	tokensProgress := 0.0
	durationProgress := 0.0

	if e.limits.MaxTurns > 0 {
		turnsProgress = float64(e.usage.TurnsUsed) / float64(e.limits.MaxTurns)
	}
	if e.limits.MaxTokens > 0 {
		tokensProgress = float64(e.usage.TokensUsed) / float64(e.limits.MaxTokens)
	}
	if e.limits.Timeout > 0 {
		durationProgress = float64(e.usage.Duration) / float64(e.limits.Timeout)
	}

	// Return the highest progress (closest to limit)
	maxProgress := turnsProgress
	if tokensProgress > maxProgress {
		maxProgress = tokensProgress
	}
	if durationProgress > maxProgress {
		maxProgress = durationProgress
	}

	return maxProgress * 100
}
