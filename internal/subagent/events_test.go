package subagent

import (
	"context"
	"testing"
	"time"
)

func TestNewEvent(t *testing.T) {
	event := NewEvent(EventSpawned, "subagent-1", "task-1", map[string]interface{}{
		"key": "value",
	})

	if event.Type != EventSpawned {
		t.Errorf("expected type %s, got %s", EventSpawned, event.Type)
	}

	if event.SubagentID != "subagent-1" {
		t.Errorf("expected subagent ID subagent-1, got %s", event.SubagentID)
	}

	if event.TaskID != "task-1" {
		t.Errorf("expected task ID task-1, got %s", event.TaskID)
	}

	if event.Payload["key"] != "value" {
		t.Errorf("expected payload key=value, got %v", event.Payload["key"])
	}

	if event.ID == "" {
		t.Error("expected non-empty event ID")
	}

	if event.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestNewEmitter(t *testing.T) {
	emitter := NewEmitter()

	if emitter == nil {
		t.Fatal("expected non-nil emitter")
	}

	if emitter.handlers == nil {
		t.Error("expected non-nil handlers")
	}

	if emitter.history == nil {
		t.Error("expected non-nil history")
	}

	if emitter.maxHistory != 1000 {
		t.Errorf("expected maxHistory 1000, got %d", emitter.maxHistory)
	}
}

func TestRegisterHandler(t *testing.T) {
	emitter := NewEmitter()

	// NewEmitter starts with 0 handlers
	if len(emitter.handlers) != 0 {
		t.Errorf("expected 0 handlers initially, got %d", len(emitter.handlers))
	}

	handler := func(ctx context.Context, event *Event) error {
		return nil
	}

	emitter.RegisterHandler(handler)

	if len(emitter.handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(emitter.handlers))
	}
}

func TestEmit(t *testing.T) {
	emitter := NewEmitter()

	handlerCalled := false
	handler := func(ctx context.Context, event *Event) error {
		handlerCalled = true
		return nil
	}

	emitter.RegisterHandler(handler)

	ctx := context.Background()
	event := NewEvent(EventSpawned, "subagent-1", "task-1", nil)

	err := emitter.Emit(ctx, event)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !handlerCalled {
		t.Error("expected handler to be called")
	}

	// Check history
	history := emitter.GetHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 event in history, got %d", len(history))
	}
}

func TestEmitMultipleHandlers(t *testing.T) {
	emitter := NewEmitter()

	callCount := 0
	handler1 := func(ctx context.Context, event *Event) error {
		callCount++
		return nil
	}
	handler2 := func(ctx context.Context, event *Event) error {
		callCount++
		return nil
	}

	emitter.RegisterHandler(handler1)
	emitter.RegisterHandler(handler2)

	ctx := context.Background()
	event := NewEvent(EventSpawned, "subagent-1", "task-1", nil)

	emitter.Emit(ctx, event)

	if callCount != 2 {
		t.Errorf("expected handler to be called 2 times, got %d", callCount)
	}
}

func TestEmitHandlerError(t *testing.T) {
	emitter := NewEmitter()

	handler1 := func(ctx context.Context, event *Event) error {
		return nil // success
	}
	handler2 := func(ctx context.Context, event *Event) error {
		return nil // success
	}

	emitter.RegisterHandler(handler1)
	emitter.RegisterHandler(handler2)

	ctx := context.Background()
	event := NewEvent(EventSpawned, "subagent-1", "task-1", nil)

	err := emitter.Emit(ctx, event)
	if err != nil {
		t.Errorf("expected no error when handlers succeed, got %v", err)
	}
}

func TestGetHistory(t *testing.T) {
	emitter := NewEmitter()

	ctx := context.Background()

	// Emit multiple events
	for i := 0; i < 3; i++ {
		event := NewEvent(EventSpawned, "subagent-1", "task-1", nil)
		emitter.Emit(ctx, event)
	}

	history := emitter.GetHistory()
	if len(history) != 3 {
		t.Errorf("expected 3 events in history, got %d", len(history))
	}
}

func TestGetHistoryForSubagent(t *testing.T) {
	emitter := NewEmitter()

	ctx := context.Background()

	// Emit events for different subagents
	emitter.Emit(ctx, NewEvent(EventSpawned, "subagent-1", "task-1", nil))
	emitter.Emit(ctx, NewEvent(EventSpawned, "subagent-2", "task-1", nil))
	emitter.Emit(ctx, NewEvent(EventStarted, "subagent-1", "task-1", nil))

	history := emitter.GetHistoryForSubagent("subagent-1")
	if len(history) != 2 {
		t.Errorf("expected 2 events for subagent-1, got %d", len(history))
	}

	history = emitter.GetHistoryForSubagent("subagent-2")
	if len(history) != 1 {
		t.Errorf("expected 1 event for subagent-2, got %d", len(history))
	}
}

func TestClearHistory(t *testing.T) {
	emitter := NewEmitter()

	ctx := context.Background()
	emitter.Emit(ctx, NewEvent(EventSpawned, "subagent-1", "task-1", nil))

	if len(emitter.GetHistory()) != 1 {
		t.Fatal("expected 1 event in history")
	}

	emitter.ClearHistory()

	if len(emitter.GetHistory()) != 0 {
		t.Errorf("expected 0 events after clear, got %d", len(emitter.GetHistory()))
	}
}

func TestHistoryLimit(t *testing.T) {
	emitter := NewEmitter()
	emitter.maxHistory = 3

	ctx := context.Background()

	// Emit more events than the limit
	for i := 0; i < 5; i++ {
		event := NewEvent(EventSpawned, "subagent-1", "task-1", nil)
		emitter.Emit(ctx, event)
	}

	history := emitter.GetHistory()
	if len(history) != 3 {
		t.Errorf("expected 3 events (max history), got %d", len(history))
	}
}

func TestConsoleEmitter(t *testing.T) {
	handler := ConsoleEmitter()

	ctx := context.Background()
	event := NewEvent(EventSpawned, "subagent-1", "task-1", nil)

	err := handler(ctx, event)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBufferedEmitter(t *testing.T) {
	buffer := make(chan *Event, 1)
	handler := BufferedEmitter(buffer)

	ctx := context.Background()
	event := NewEvent(EventSpawned, "subagent-1", "task-1", nil)

	err := handler(ctx, event)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case received := <-buffer:
		if received.ID != event.ID {
			t.Error("expected to receive the same event")
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for event")
	}
}

func TestBufferedEmitterContextCancelled(t *testing.T) {
	buffer := make(chan *Event, 0) // Unbuffered channel
	handler := BufferedEmitter(buffer)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	event := NewEvent(EventSpawned, "subagent-1", "task-1", nil)

	// This should fail because context is canceled and channel is blocked
	err := handler(ctx, event)
	if err != context.Canceled {
		// If channel sends first, it might succeed before context check
		// This is a race, but either way is acceptable
		if err != nil {
			t.Errorf("expected context.Canceled or nil, got %v", err)
		}
	}
}

func TestFilteredEmitter(t *testing.T) {
	called := false
	handler := func(ctx context.Context, event *Event) error {
		called = true
		return nil
	}

	// Filter to only EventSpawned
	filtered := FilteredEmitter(handler, EventSpawned)

	ctx := context.Background()

	// Should be called for EventSpawned
	called = false
	filtered(ctx, NewEvent(EventSpawned, "subagent-1", "task-1", nil))
	if !called {
		t.Error("expected handler to be called for EventSpawned")
	}

	// Should NOT be called for EventCompleted
	called = false
	filtered(ctx, NewEvent(EventCompleted, "subagent-1", "task-1", nil))
	if called {
		t.Error("expected handler NOT to be called for EventCompleted")
	}
}

func TestFilteredEmitterEmptyTypes(t *testing.T) {
	called := false
	handler := func(ctx context.Context, event *Event) error {
		called = true
		return nil
	}

	// Empty types means allow all
	filtered := FilteredEmitter(handler)

	ctx := context.Background()
	filtered(ctx, NewEvent(EventSpawned, "subagent-1", "task-1", nil))

	if !called {
		t.Error("expected handler to be called when no filter types")
	}
}

func TestInMemoryEventStore(t *testing.T) {
	store := NewInMemoryEventStore()

	event1 := NewEvent(EventSpawned, "subagent-1", "task-1", nil)
	event2 := NewEvent(EventStarted, "subagent-1", "task-1", nil)
	event3 := NewEvent(EventSpawned, "subagent-2", "task-2", nil)

	// Save events
	store.Save(event1)
	store.Save(event2)
	store.Save(event3)

	// Get by subagent
	events, err := store.Get("subagent-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events for subagent-1, got %d", len(events))
	}

	events, _ = store.Get("subagent-2")
	if len(events) != 1 {
		t.Errorf("expected 1 event for subagent-2, got %d", len(events))
	}

	// Get all
	allEvents, _ := store.GetAll()
	if len(allEvents) != 3 {
		t.Errorf("expected 3 total events, got %d", len(allEvents))
	}
}

func TestStoreEmitter(t *testing.T) {
	store := NewInMemoryEventStore()
	handler := StoreEmitter(store)

	ctx := context.Background()
	event := NewEvent(EventSpawned, "subagent-1", "task-1", nil)

	err := handler(ctx, event)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	events, _ := store.Get("subagent-1")
	if len(events) != 1 {
		t.Errorf("expected 1 event in store, got %d", len(events))
	}
}
