package io_test

import (
	"aicompass/foundation/agent/io"
	"context"
	"errors"
	"testing"
)

// =============================================================================
// Handler context injection tests
// =============================================================================

func TestWithHandler_and_FromContext(t *testing.T) {
	ctx := context.Background()

	// No handler set → nil
	if h := io.FromContext(ctx); h != nil {
		t.Fatalf("expected nil handler, got %v", h)
	}

	// Set handler → retrieve it
	handler := io.NewNoopHandler()
	ctx = io.WithHandler(ctx, handler)
	got := io.FromContext(ctx)
	if got == nil {
		t.Fatal("expected handler, got nil")
	}
}

func TestConfirm_no_handler_auto_approves(t *testing.T) {
	ctx := context.Background()
	approved, err := io.Confirm(ctx, "do something?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected auto-approve when no handler is set")
	}
}

func TestRead_no_handler_returns_error(t *testing.T) {
	ctx := context.Background()
	_, err := io.Read(ctx, "enter value:", nil)
	if err == nil {
		t.Fatal("expected error when no handler is set")
	}
}

// =============================================================================
// NoopHandler tests
// =============================================================================

func TestNoopHandler_Confirm_auto_approves(t *testing.T) {
	h := io.NewNoopHandler()
	ctx := context.Background()

	approved, err := h.Confirm(ctx, "delete everything?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("NoopHandler should auto-approve")
	}
}

func TestNoopHandler_Read_returns_error(t *testing.T) {
	h := io.NewNoopHandler()
	ctx := context.Background()

	_, err := h.Read(ctx, "enter name:", nil)
	if err == nil {
		t.Fatal("NoopHandler.Read should return error")
	}

	var inputErr *io.ErrInputRequired
	if !errors.As(err, &inputErr) {
		t.Fatalf("expected ErrInputRequired, got %T", err)
	}
}

// =============================================================================
// DenyHandler tests
// =============================================================================

func TestDenyHandler_Confirm_denies(t *testing.T) {
	h := io.NewDenyHandler()
	ctx := context.Background()

	approved, err := h.Confirm(ctx, "delete everything?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Fatal("DenyHandler should deny")
	}
}

// =============================================================================
// PauseHandler tests
// =============================================================================

func TestPauseHandler_Confirm_returns_ErrAuthRequired(t *testing.T) {
	h := io.NewPauseHandler()
	ctx := context.Background()

	approved, err := h.Confirm(ctx, "Allow web_search?", "web_search")
	if approved {
		t.Fatal("PauseHandler.Confirm should not approve")
	}
	if err == nil {
		t.Fatal("PauseHandler.Confirm should return error")
	}

	var authErr *io.ErrAuthRequired
	if !errors.As(err, &authErr) {
		t.Fatalf("expected ErrAuthRequired, got %T: %v", err, err)
	}

	if authErr.Resource != "tool:web_search" {
		t.Errorf("expected resource 'tool:web_search', got %q", authErr.Resource)
	}
	if authErr.Action != "execute" {
		t.Errorf("expected action 'execute', got %q", authErr.Action)
	}
	if authErr.Reason != "Allow web_search?" {
		t.Errorf("expected reason 'Allow web_search?', got %q", authErr.Reason)
	}
}

func TestPauseHandler_Confirm_without_tool_name(t *testing.T) {
	h := io.NewPauseHandler()
	ctx := context.Background()

	_, err := h.Confirm(ctx, "Allow this?", nil)
	var authErr *io.ErrAuthRequired
	if !errors.As(err, &authErr) {
		t.Fatalf("expected ErrAuthRequired, got %T", err)
	}
	if authErr.Resource != "tool_execution" {
		t.Errorf("expected resource 'tool_execution', got %q", authErr.Resource)
	}
}

func TestPauseHandler_Read_returns_ErrInputRequired(t *testing.T) {
	h := io.NewPauseHandler()
	ctx := context.Background()

	schema := map[string]any{"type": "string"}
	_, err := h.Read(ctx, "What is your name?", schema)

	var inputErr *io.ErrInputRequired
	if !errors.As(err, &inputErr) {
		t.Fatalf("expected ErrInputRequired, got %T: %v", err, err)
	}

	if inputErr.Question != "What is your name?" {
		t.Errorf("expected question 'What is your name?', got %q", inputErr.Question)
	}
	if inputErr.Schema["type"] != "string" {
		t.Errorf("expected schema type 'string', got %v", inputErr.Schema["type"])
	}
}

// =============================================================================
// Convenience function tests with handlers
// =============================================================================

func TestConfirm_with_NoopHandler(t *testing.T) {
	ctx := io.WithHandler(context.Background(), io.NewNoopHandler())
	approved, err := io.Confirm(ctx, "do something?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected approval with NoopHandler")
	}
}

func TestConfirm_with_DenyHandler(t *testing.T) {
	ctx := io.WithHandler(context.Background(), io.NewDenyHandler())
	approved, err := io.Confirm(ctx, "do something?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approved {
		t.Fatal("expected denial with DenyHandler")
	}
}

func TestConfirm_with_PauseHandler(t *testing.T) {
	ctx := io.WithHandler(context.Background(), io.NewPauseHandler())
	_, err := io.Confirm(ctx, "do something?", nil)

	var authErr *io.ErrAuthRequired
	if !errors.As(err, &authErr) {
		t.Fatalf("expected ErrAuthRequired, got %T", err)
	}
}

// =============================================================================
// Error message tests
// =============================================================================

func TestErrInputRequired_Error(t *testing.T) {
	err := &io.ErrInputRequired{Question: "What color?"}
	expected := "io: input required: What color?"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestErrAuthRequired_Error(t *testing.T) {
	err := &io.ErrAuthRequired{
		Resource: "tool:web_search",
		Reason:   "Allow web search?",
	}
	expected := "io: authorization required: tool:web_search (Allow web search?)"
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

// =============================================================================
// Context cancellation tests
// =============================================================================

func TestConfirm_with_cancelled_context(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// NoopHandler should still work with cancelled context
	// (it doesn't check context)
	ctx = io.WithHandler(ctx, io.NewNoopHandler())
	approved, err := io.Confirm(ctx, "do something?", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !approved {
		t.Fatal("expected approval")
	}
}
