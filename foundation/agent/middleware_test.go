package agent

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"
)

// =============================================================================
// Context Factory Tests
// =============================================================================

func TestNewAgentContext(t *testing.T) {
	ctx := context.Background()
	c := NewAgentContext(ctx, nil, nil, nil, true)

	if c.Kind != OperationAgent {
		t.Errorf("expected OperationAgent, got %v", c.Kind)
	}
	if !c.IsAgent() {
		t.Error("IsAgent() should return true")
	}
	if c.IsChat() {
		t.Error("IsChat() should return false")
	}
	if c.IsFunction() {
		t.Error("IsFunction() should return false")
	}
	if c.Agent == nil {
		t.Error("Agent should not be nil")
	}
	if c.Chat != nil {
		t.Error("Chat should be nil")
	}
	if c.Function != nil {
		t.Error("Function should be nil")
	}
}

func TestNewChatContext(t *testing.T) {
	ctx := context.Background()
	c := NewChatContext(ctx, nil, nil, nil)

	if c.Kind != OperationChat {
		t.Errorf("expected OperationChat, got %v", c.Kind)
	}
	if c.IsAgent() {
		t.Error("IsAgent() should return false")
	}
	if !c.IsChat() {
		t.Error("IsChat() should return true")
	}
	if c.Agent != nil {
		t.Error("Agent should be nil")
	}
	if c.Chat == nil {
		t.Error("Chat should not be nil")
	}
}

func TestNewFunctionContext(t *testing.T) {
	ctx := context.Background()
	c := NewFunctionContext(ctx, nil, map[string]any{"key": "value"})

	if c.Kind != OperationFunction {
		t.Errorf("expected OperationFunction, got %v", c.Kind)
	}
	if !c.IsFunction() {
		t.Error("IsFunction() should return true")
	}
	if c.Function == nil {
		t.Error("Function should not be nil")
	}
	if c.Function.Arguments["key"] != "value" {
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

	c := NewAgentContext(context.Background(), nil, nil, nil, false)
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

	c := NewAgentContext(context.Background(), nil, nil, nil, false)
	if err := handler.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Error("final handler should have been called")
	}
}

// =============================================================================
// Agent Middleware Tests
// =============================================================================

func TestRetry_PassesThroughNonAgentOps(t *testing.T) {
	called := 0
	mw := Retry(3)
	handler := mw(HandlerFunc(func(c *Context) error {
		called++
		return errors.New("fail")
	}))

	// Chat context - should pass through without retry
	c := NewChatContext(context.Background(), nil, nil, nil)
	err := handler.Handle(c)

	if err == nil {
		t.Error("expected error")
	}
	if called != 1 {
		t.Errorf("expected 1 call (passthrough), got %d", called)
	}
}

func TestRetry_RetriesAgentOps(t *testing.T) {
	attempts := 0
	mw := Retry(3)
	handler := mw(HandlerFunc(func(c *Context) error {
		attempts++
		if attempts < 3 {
			return errors.New("transient error")
		}
		return nil // Succeed on third attempt
	}))

	c := NewAgentContext(context.Background(), nil, nil, nil, false)
	err := handler.Handle(c)
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestTimeout_TimesOut(t *testing.T) {
	mw := Timeout(50 * time.Millisecond)
	handler := mw(HandlerFunc(func(c *Context) error {
		time.Sleep(200 * time.Millisecond)
		return nil
	}))

	c := NewAgentContext(context.Background(), nil, nil, nil, false)
	err := handler.Handle(c)

	if err == nil {
		t.Error("expected timeout error")
	}
}

func TestRecovery_RecoversPanic(t *testing.T) {
	mw := Recovery()
	handler := mw(HandlerFunc(func(c *Context) error {
		panic("test panic")
	}))

	c := NewAgentContext(context.Background(), nil, nil, nil, false)
	err := handler.Handle(c)

	if err == nil {
		t.Error("expected error from panic")
	}
}

func TestLogging_LogsOperations(t *testing.T) {
	logger := slog.Default()
	mw := Logging(logger)
	handler := mw(NoOpHandler)

	// Test agent op
	c := NewAgentContext(context.Background(), nil, nil, nil, false)
	if err := handler.Handle(c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test chat op
	c = NewChatContext(context.Background(), nil, nil, nil)
	if err := handler.Handle(c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Test function op
	c = NewFunctionContext(context.Background(), nil, nil)
	if err := handler.Handle(c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// =============================================================================
// Chat Middleware Tests
// =============================================================================

type mockTokenCounter struct {
	inputCalled  bool
	outputCalled bool
}

func (m *mockTokenCounter) CountInput(messages []Message) { m.inputCalled = true }
func (m *mockTokenCounter) CountOutput(result any)        { m.outputCalled = true }

func TestTokenCounting_PassesThroughNonChatOps(t *testing.T) {
	counter := &mockTokenCounter{}
	mw := TokenCounting(counter)
	handler := mw(NoOpHandler)

	c := NewAgentContext(context.Background(), nil, nil, nil, false)
	if err := handler.Handle(c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if counter.inputCalled || counter.outputCalled {
		t.Error("counter should not be called for non-chat ops")
	}
}

func TestTokenCounting_CountsTokensForChatOps(t *testing.T) {
	counter := &mockTokenCounter{}
	mw := TokenCounting(counter)
	handler := mw(NoOpHandler)

	c := NewChatContext(context.Background(), nil, nil, nil)
	if err := handler.Handle(c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !counter.inputCalled {
		t.Error("CountInput should be called")
	}
	if !counter.outputCalled {
		t.Error("CountOutput should be called")
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

func TestFunctionCaching_PassesThroughNonFunctionOps(t *testing.T) {
	cache := &mockCache{data: make(map[string]any)}
	keyFunc := func(tool Tool, args map[string]any) string { return "key" }
	mw := FunctionCaching(cache, keyFunc)
	handler := mw(NoOpHandler)

	c := NewAgentContext(context.Background(), nil, nil, nil, false)
	if err := handler.Handle(c); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(cache.data) != 0 {
		t.Error("cache should not be modified for non-function ops")
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

	c := NewFunctionContext(context.Background(), nil, nil)

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
	c2 := NewFunctionContext(context.Background(), nil, nil)
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

	c := NewFunctionContext(context.Background(), nil, nil)
	err := handler.Handle(c)

	if err == nil {
		t.Error("expected timeout error")
	}
}
