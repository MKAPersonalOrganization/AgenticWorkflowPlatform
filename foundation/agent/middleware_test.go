package agent

import (
	"context"
	"testing"
)

// =============================================================================
// Context Factory Tests
// =============================================================================

func TestNewToolContext(t *testing.T) {
	ctx := context.Background()
	c := NewToolContext(ctx, nil, map[string]any{"key": "value"})

	if c.Kind != OperationTool {
		t.Errorf("expected OperationTool, got %v", c.Kind)
	}
	if !c.IsTool() {
		t.Error("IsTool() should return true")
	}
	if c.Tool == nil {
		t.Error("Tool should not be nil")
	}
	if c.Tool.Arguments["key"] != "value" {
		t.Error("Arguments should contain key=value")
	}
}

// =============================================================================
// Chain Tests
// =============================================================================

func TestChain_ExecutesInOrder(t *testing.T) {
	var order []string

	mw1 := func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			order = append(order, "mw1:before")
			err := next.Handle(c)
			order = append(order, "mw1:after")
			return err
		})
	}

	mw2 := func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			order = append(order, "mw2:before")
			err := next.Handle(c)
			order = append(order, "mw2:after")
			return err
		})
	}

	final := HandlerFunc(func(c *Context) error {
		order = append(order, "handler")
		return nil
	})

	chain := Chain(mw1, mw2)
	handler := chain(final)

	c := NewToolContext(context.Background(), nil, nil)
	if err := handler.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{
		"mw1:before",
		"mw2:before",
		"handler",
		"mw2:after",
		"mw1:after",
	}

	if len(order) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(order))
	}

	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d]: expected %s, got %s", i, v, order[i])
		}
	}
}

func TestChain_EmptyMiddleware(t *testing.T) {
	called := false
	final := HandlerFunc(func(c *Context) error {
		called = true
		return nil
	})

	chain := Chain()
	handler := chain(final)

	c := NewToolContext(context.Background(), nil, nil)
	if err := handler.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Error("final handler should have been called")
	}
}

