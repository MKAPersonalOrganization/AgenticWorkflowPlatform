package event

import (
	"context"
	"errors"
	"testing"
	"time"

	es "school-with-ai/foundation/eventsourcing"
)

// testEvent is a test-local event type used to avoid importing module types
// (modules/chat/thread) from the foundation layer. See AC-13 / TODO-11.
type testEvent struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Timestamp time.Time `json:"timestamp"`
}

func (e *testEvent) EventType() string         { return "test.EventCreated" }
func (e *testEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e *testEvent) AggregateID() string       { return e.ID }

func TestRouter(t *testing.T) {
	t.Parallel()
	router := NewRouter()
	ctx := context.Background()

	var handled bool
	router.Register(&testEvent{}, func(ctx context.Context, e es.Event) error {
		handled = true
		return nil
	})

	var anyHandled bool
	router.RegisterAny(func(ctx context.Context, e es.Event) error {
		anyHandled = true
		return nil
	})

	evt := &testEvent{ID: "test-123", Name: "example", Timestamp: time.Now()}
	if err := router.Dispatch(ctx, evt); err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if !handled {
		t.Error("Specific handler not called")
	}
	if !anyHandled {
		t.Error("Any handler not called")
	}
}

func TestRouter_Error(t *testing.T) {
	t.Parallel()
	router := NewRouter()
	ctx := context.Background()

	expectedErr := errors.New("test error")
	router.Register(&testEvent{}, func(ctx context.Context, e es.Event) error {
		return expectedErr
	})

	evt := &testEvent{ID: "test-123", Name: "example", Timestamp: time.Now()}
	err := router.Dispatch(ctx, evt)
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}
