package io_test

import (
	"aicompass/foundation/agent/io"
	"context"
	"errors"
	"testing"
)

func TestCachedReadHandler_Read_returns_cached_value(t *testing.T) {
	t.Parallel()

	h := io.NewCachedReadHandler("John Doe")
	ctx := context.Background()

	result, err := h.Read(ctx, "What is your name?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "John Doe" {
		t.Errorf("expected %q, got %q", "John Doe", result)
	}
}

func TestCachedReadHandler_Read_second_call_returns_ErrInputRequired(t *testing.T) {
	t.Parallel()

	h := io.NewCachedReadHandler("first answer")
	ctx := context.Background()

	// First call: returns cached value
	_, err := h.Read(ctx, "question 1", nil)
	if err != nil {
		t.Fatalf("first Read: unexpected error: %v", err)
	}

	// Second call: falls through to PauseHandler → ErrInputRequired
	_, err = h.Read(ctx, "question 2", nil)
	var inputErr *io.ErrInputRequired
	if !errors.As(err, &inputErr) {
		t.Fatalf("second Read: expected ErrInputRequired, got %T: %v", err, err)
	}
	if inputErr.Question != "question 2" {
		t.Errorf("expected question %q, got %q", "question 2", inputErr.Question)
	}
}

func TestCachedReadHandler_Confirm_delegates_to_PauseHandler(t *testing.T) {
	t.Parallel()

	h := io.NewCachedReadHandler("some input")
	ctx := context.Background()

	approved, err := h.Confirm(ctx, "allow?", "my_tool")
	if approved {
		t.Fatal("CachedReadHandler.Confirm should not approve (delegates to PauseHandler)")
	}
	var authErr *io.ErrAuthRequired
	if !errors.As(err, &authErr) {
		t.Fatalf("expected ErrAuthRequired, got %T: %v", err, err)
	}
}

func TestCachedReadHandler_via_context(t *testing.T) {
	t.Parallel()

	ctx := io.WithHandler(context.Background(), io.NewCachedReadHandler("cached text"))

	result, err := io.Read(ctx, "prompt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "cached text" {
		t.Errorf("expected %q, got %q", "cached text", result)
	}
}
