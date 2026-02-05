package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"school-with-ai/domain"
)

func TestRouter(t *testing.T) {
	t.Parallel()
	router := NewRouter()
	ctx := context.Background()

	var handled bool
	router.Register(&domain.ThreadCreated{}, func(ctx context.Context, e domain.Event) error {
		handled = true
		return nil
	})

	var anyHandled bool
	router.RegisterAny(func(ctx context.Context, e domain.Event) error {
		anyHandled = true
		return nil
	})

	evt := &domain.ThreadCreated{ThreadID: "test-123", UserID: "user-456", Timestamp: time.Now()}
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
	router.Register(&domain.ThreadCreated{}, func(ctx context.Context, e domain.Event) error {
		return expectedErr
	})

	evt := &domain.ThreadCreated{ThreadID: "test-123", UserID: "user-456", Timestamp: time.Now()}
	err := router.Dispatch(ctx, evt)
	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}
}
