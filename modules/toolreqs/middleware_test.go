package toolreqs_test

import (
	"context"
	"testing"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agent/io"
	"school-with-ai/modules/toolreqs"
)

// =============================================================================
// Middleware tests
// =============================================================================

func TestMiddleware_NoPolicy_PassesThrough(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	mw := toolreqs.ConfirmationMiddleware(cr, "agent1")

	called := false
	handler := mw(agent.HandlerFunc(func(c *agent.Context) error {
		called = true
		return nil
	}))

	tool := &mockTool{name: "calculator"}
	ctx := agent.NewFunctionContext(context.Background(), tool, map[string]any{"x": 1})
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected handler to be called when no confirmation policy")
	}
}

func TestMiddleware_WithPolicy_NoopHandler_AutoApproves(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("dangerous_tool", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
		Hint:          "Confirm dangerous action?",
	})
	mw := toolreqs.ConfirmationMiddleware(cr, "agent1")

	called := false
	handler := mw(agent.HandlerFunc(func(c *agent.Context) error {
		called = true
		return nil
	}))

	// NoopHandler auto-approves all confirmations.
	goCtx := io.WithHandler(context.Background(), &io.NoopHandler{})
	tool := &mockTool{name: "dangerous_tool"}
	ctx := agent.NewFunctionContext(goCtx, tool, nil)

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected handler to be called after auto-approval")
	}
}

func TestMiddleware_WithPolicy_DenyHandler_Blocks(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("dangerous_tool", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
	})
	mw := toolreqs.ConfirmationMiddleware(cr, "agent1")

	called := false
	handler := mw(agent.HandlerFunc(func(c *agent.Context) error {
		called = true
		return nil
	}))

	// DenyHandler rejects all confirmations.
	goCtx := io.WithHandler(context.Background(), &io.DenyHandler{})
	tool := &mockTool{name: "dangerous_tool"}
	ctx := agent.NewFunctionContext(goCtx, tool, nil)

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("handler should NOT be called when user denies")
	}
	// Should have set a denial result.
	result, ok := ctx.Result.(map[string]any)
	if !ok {
		t.Fatal("expected result to be set")
	}
	if denied, _ := result["denied"].(bool); !denied {
		t.Error("expected denied flag in result")
	}
}

func TestMiddleware_WithPolicy_PauseHandler_ReturnsAuthError(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("file_delete", &toolreqs.ConfirmationPolicy{
		AlwaysConfirm: true,
	})
	mw := toolreqs.ConfirmationMiddleware(cr, "agent1")

	handler := mw(agent.HandlerFunc(func(c *agent.Context) error {
		t.Error("handler should NOT be called when paused")
		return nil
	}))

	// PauseHandler returns ErrAuthRequired to pause execution.
	goCtx := io.WithHandler(context.Background(), &io.PauseHandler{})
	tool := &mockTool{name: "file_delete"}
	ctx := agent.NewFunctionContext(goCtx, tool, nil)

	err := handler.Handle(ctx)
	if err == nil {
		t.Fatal("expected an error from PauseHandler")
	}
	// The error should be an ErrAuthRequired (sentinel for pause).
	if _, ok := err.(*io.ErrAuthRequired); !ok {
		t.Errorf("expected *io.ErrAuthRequired, got %T: %v", err, err)
	}
}

func TestMiddleware_NonFunctionContext_PassesThrough(t *testing.T) {
	cr := toolreqs.NewConfirmationRequirement()
	cr.SetGlobalPolicy("any_tool", &toolreqs.ConfirmationPolicy{AlwaysConfirm: true})
	mw := toolreqs.ConfirmationMiddleware(cr, "agent1")

	called := false
	handler := mw(agent.HandlerFunc(func(c *agent.Context) error {
		called = true
		return nil
	}))

	// Agent context (not a function call) — middleware should pass through.
	ctx := agent.NewAgentContext(context.Background(), nil, nil, false)
	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected handler to be called for non-function context")
	}
}

// =============================================================================
// Test helpers
// =============================================================================

type mockTool struct {
	name string
}

func (t *mockTool) Name() string          { return t.name }
func (t *mockTool) Description() string   { return "mock tool" }
func (t *mockTool) Schema() *agent.Schema { return nil }
