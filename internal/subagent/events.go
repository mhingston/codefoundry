package subagent

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// EventType defines the type of subagent event
type EventType string

const (
	// EventSpawned is emitted when a subagent is spawned
	EventSpawned EventType = "subagent_spawned"
	// EventStarted is emitted when a subagent starts execution
	EventStarted EventType = "subagent_started"
	// EventProgress is emitted periodically during execution
	EventProgress EventType = "subagent_progress"
	// EventCompleted is emitted when a subagent completes
	EventCompleted EventType = "subagent_completed"
	// EventFailed is emitted when a subagent fails
	EventFailed EventType = "subagent_failed"
	// EventAborted is emitted when a subagent is aborted
	EventAborted EventType = "subagent_aborted"
	// EventTimeout is emitted when a subagent times out
	EventTimeout EventType = "subagent_timeout"
	// EventLimitExceeded is emitted when a resource limit is exceeded
	EventLimitExceeded EventType = "subagent_limit_exceeded"
)

// Event represents a subagent lifecycle event
type Event struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	SubagentID string                `json:"subagent_id"`
	TaskID    string                 `json:"task_id"`
	Timestamp time.Time              `json:"timestamp"`
	Payload   map[string]interface{} `json:"payload"`
}

// NewEvent creates a new event
func NewEvent(eventType EventType, subagentID, taskID string, payload map[string]interface{}) *Event {
	return &Event{
		ID:         fmt.Sprintf("evt-%d-%s", time.Now().UnixNano(), subagentID),
		Type:       eventType,
		SubagentID: subagentID,
		TaskID:     taskID,
		Timestamp:  time.Now().UTC(),
		Payload:    payload,
	}
}

// Emitter manages event emission
type Emitter struct {
	handlers []EventHandler
	mu       sync.RWMutex
	history  []*Event
	maxHistory int
}

// EventHandler is a function that handles events
type EventHandler func(ctx context.Context, event *Event) error

// NewEmitter creates a new event emitter
func NewEmitter() *Emitter {
	return &Emitter{
		handlers:   make([]EventHandler, 0),
		history:    make([]*Event, 0),
		maxHistory: 1000,
	}
}

// RegisterHandler registers an event handler
func (e *Emitter) RegisterHandler(handler EventHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.handlers = append(e.handlers, handler)
}

// Emit emits an event to all registered handlers
func (e *Emitter) Emit(ctx context.Context, event *Event) error {
	// Add to history
	e.mu.Lock()
	e.history = append(e.history, event)
	if len(e.history) > e.maxHistory {
		e.history = e.history[len(e.history)-e.maxHistory:]
	}
	handlers := make([]EventHandler, len(e.handlers))
	copy(handlers, e.handlers)
	e.mu.Unlock()

	// Emit to all handlers
	var lastErr error
	for _, handler := range handlers {
		if err := handler(ctx, event); err != nil {
			lastErr = err
			// Continue to other handlers even if one fails
		}
	}

	return lastErr
}

// GetHistory returns event history
func (e *Emitter) GetHistory() []*Event {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	result := make([]*Event, len(e.history))
	copy(result, e.history)
	return result
}

// GetHistoryForSubagent returns event history for a specific subagent
func (e *Emitter) GetHistoryForSubagent(subagentID string) []*Event {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	var result []*Event
	for _, event := range e.history {
		if event.SubagentID == subagentID {
			result = append(result, event)
		}
	}
	return result
}

// ClearHistory clears event history
func (e *Emitter) ClearHistory() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = make([]*Event, 0)
}

// ConsoleEmitter creates an event handler that logs to console
func ConsoleEmitter() EventHandler {
	return func(ctx context.Context, event *Event) error {
		fmt.Printf("[%s] %s: subagent=%s task=%s\n",
			event.Timestamp.Format(time.RFC3339),
			event.Type,
			event.SubagentID,
			event.TaskID,
		)
		return nil
	}
}

// BufferedEmitter creates an event handler that buffers events
func BufferedEmitter(buffer chan *Event) EventHandler {
	return func(ctx context.Context, event *Event) error {
		select {
		case buffer <- event:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// FilteredEmitter creates an event handler that filters by type
func FilteredEmitter(handler EventHandler, types ...EventType) EventHandler {
	allowedTypes := make(map[EventType]bool)
	for _, t := range types {
		allowedTypes[t] = true
	}
	
	return func(ctx context.Context, event *Event) error {
		if len(allowedTypes) > 0 && !allowedTypes[event.Type] {
			return nil
		}
		return handler(ctx, event)
	}
}

// EventStore provides persistent event storage
type EventStore interface {
	Save(event *Event) error
	Get(subagentID string) ([]*Event, error)
	GetAll() ([]*Event, error)
}

// InMemoryEventStore is an in-memory implementation of EventStore
type InMemoryEventStore struct {
	mu     sync.RWMutex
	events map[string][]*Event
}

// NewInMemoryEventStore creates a new in-memory event store
func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events: make(map[string][]*Event),
	}
}

// Save saves an event
func (s *InMemoryEventStore) Save(event *Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	s.events[event.SubagentID] = append(s.events[event.SubagentID], event)
	return nil
}

// Get gets events for a subagent
func (s *InMemoryEventStore) Get(subagentID string) ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	events := s.events[subagentID]
	result := make([]*Event, len(events))
	copy(result, events)
	return result, nil
}

// GetAll gets all events
func (s *InMemoryEventStore) GetAll() ([]*Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*Event
	for _, events := range s.events {
		result = append(result, events...)
	}
	return result, nil
}

// StoreEmitter creates an event handler that persists events
func StoreEmitter(store EventStore) EventHandler {
	return func(ctx context.Context, event *Event) error {
		return store.Save(event)
	}
}
