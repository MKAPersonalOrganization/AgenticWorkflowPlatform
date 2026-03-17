package agent

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// Context Factory Tests
// =============================================================================

func TestNewAgentContext(t *testing.T) {
	ctx := context.Background()
	c := NewAgentContext(ctx, nil, nil, true)

	if c.Kind != OperationAgent {
		t.Errorf("expected OperationAgent, got %v", c.Kind)
	}
	if !c.IsAgent() {
		t.Error("IsAgent() should return true")
	}
	if c.IsTool() {
		t.Error("IsTool() should return false")
	}
	if c.Agent == nil {
		t.Error("Agent should not be nil")
	}
	if c.Tool != nil {
		t.Error("Tool should be nil")
	}
}

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

// =============================================================================
// Function Middleware Tests
// =============================================================================

type mockCache struct {
	data map[string]any
}

func (m *mockCache) Get(key string) (any, bool) {
	v, ok := m.data[key]
	return v, ok
}

func (m *mockCache) Set(key string, value any) {
	m.data[key] = value
}

func TestFunctionCaching_PassesThroughNonToolOps(t *testing.T) {
	cache := &mockCache{data: make(map[string]any)}
	keyFunc := func(tool Tool, args map[string]any) string { return "key" }
	mw := FunctionCaching(cache, keyFunc)
	handler := mw(NoOpHandler)

	// Context with nil Tool field — should pass through without caching.
	c := NewAgentContext(context.Background(), nil, nil, false)
	if err := handler.Handle(c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(cache.data) != 0 {
		t.Error("cache should not be modified for non-tool ops")
	}
}

func TestFunctionCaching_CachesFunctionResults(t *testing.T) {
	cache := &mockCache{data: make(map[string]any)}
	keyFunc := func(tool Tool, args map[string]any) string { return "test-key" }

	callCount := 0
	mw := FunctionCaching(cache, keyFunc)
	handler := mw(HandlerFunc(func(c *Context) error {
		callCount++
		c.Result = "computed-result"
		return nil
	}))

	c := NewToolContext(context.Background(), nil, nil)

	// First call - should execute handler
	if err := handler.Handle(c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if c.Result != "computed-result" {
		t.Errorf("expected computed-result, got %v", c.Result)
	}

	// Second call - should use cache
	c2 := NewToolContext(context.Background(), nil, nil)
	if err := handler.Handle(c2); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected still 1 call (cached), got %d", callCount)
	}
	if c2.Result != "computed-result" {
		t.Errorf("expected cached result, got %v", c2.Result)
	}
}

func TestFunctionTimeout_TimesOut(t *testing.T) {
	mw := FunctionTimeout(50 * time.Millisecond)
	handler := mw(HandlerFunc(func(c *Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}))

	c := NewToolContext(context.Background(), nil, nil)
	err := handler.Handle(c)

	if err == nil {
		t.Error("expected timeout error")
	}
}
