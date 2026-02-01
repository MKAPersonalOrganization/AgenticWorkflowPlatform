package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"school-with-ai/domain"
)

type mockEvent struct {
	eventType string
	timestamp time.Time
	aggID     string
}

func (m *mockEvent) EventType() string         { return m.eventType }
func (m *mockEvent) EventTimestamp() time.Time { return m.timestamp }
func (m *mockEvent) AggregateID() string       { return m.aggID }

func TestRouter(t *testing.T) {
	t.Parallel()
	router := NewRouter()
	ctx := context.Background()

	var handled bool
	router.Register(&mockEvent{}, func(ctx context.Context, e domain.Event) error {
		handled = true
		return nil
	})

	var anyHandled bool
	router.RegisterAny(func(ctx context.Context, e domain.Event) error {
		anyHandled = true
		return nil
	})

	evt := &mockEvent{eventType: "MockEvent", timestamp: time.Now(), aggID: "test-123"}
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
	router.Register(&mockEvent{}, func(ctx context.Context, e domain.Event) error {
		return expectedErr
	})

	evt := &mockEvent{eventType: "MockEvent", timestamp: time.Now(), aggID: "test-123"}
	err := router.Dispatch(ctx, evt)
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}
