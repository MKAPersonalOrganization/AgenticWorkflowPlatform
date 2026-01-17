package event

import (
	"context"
	"errors"
	"school-with-ai/foundation/agentsdk/core"
	"testing"
)

type mockEvent struct {
	core.BaseEvent
}

func (m *mockEvent) EventType() string { return "MockEvent" }

func TestRouter(t *testing.T) {
	router := NewRouter()
	ctx := context.Background()

	var handled bool
	router.Register(&mockEvent{}, func(ctx context.Context, e core.Event) error {
		handled = true
		return nil
	})

	var anyHandled bool
	router.RegisterAny(func(ctx context.Context, e core.Event) error {
		anyHandled = true
		return nil
	})

	evt := &mockEvent{}
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
	router := NewRouter()
	ctx := context.Background()

	expectedErr := errors.New("test error")
	router.Register(&mockEvent{}, func(ctx context.Context, e core.Event) error {
		return expectedErr
	})

	evt := &mockEvent{}
	err := router.Dispatch(ctx, evt)
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}
