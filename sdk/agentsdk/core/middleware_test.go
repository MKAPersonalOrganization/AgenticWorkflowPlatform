package core

import (
	"context"
	"errors"
	"school-with-ai/sdk/agentsdk/llm"
	"testing"
)

// Mock middleware for testing.
type mockMiddleware struct {
	beforeCalled bool
	afterCalled  bool
	beforeFunc   func(*RequestContext)
	afterFunc    func(*RequestContext)
	shouldError  bool
}

func (m *mockMiddleware) Process(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
	m.beforeCalled = true
	if m.beforeFunc != nil {
		m.beforeFunc(reqCtx)
	}

	if m.shouldError {
		return errors.New("middleware error")
	}

	err := next(ctx, reqCtx)

	m.afterCalled = true
	if m.afterFunc != nil {
		m.afterFunc(reqCtx)
	}

	return err
}

func TestNewRequestContext(t *testing.T) {
	messages := []llm.ChatMessage{
		{Role: llm.ChatRoleUser, Contents: []llm.AIContent{llm.TextContent{Text: "Hello"}}},
	}
	systemPrompt := "You are a helpful assistant"
	tools := []Tool{}
	config := &AgentConfig{Name: "test-agent"}

	ctx := NewRequestContext("", messages, systemPrompt, tools, config)

	if len(ctx.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(ctx.Messages))
	}
	if ctx.SystemPrompt != systemPrompt {
		t.Errorf("Expected system prompt %q, got %q", systemPrompt, ctx.SystemPrompt)
	}
	if ctx.Config.Name != "test-agent" {
		t.Errorf("Expected agent name 'test-agent', got %q", ctx.Config.Name)
	}
	if ctx.State == nil {
		t.Error("Expected State to be initialized")
	}
	if ctx.Terminate {
		t.Error("Expected Terminate to be false")
	}
}

func TestRequestContext_WithMetadata(t *testing.T) {
	ctx := NewRequestContext("", nil, "", nil, nil)

	ctx.WithMetadata("key1", "value1")
	ctx.WithMetadata("key2", 42)
	ctx.WithMetadata("key3", true)

	// Test retrieval
	val1, ok1 := ctx.GetMetadata("key1")
	if !ok1 || val1 != "value1" {
		t.Errorf("Expected key1='value1', got %v (exists=%v)", val1, ok1)
	}

	val2, ok2 := ctx.GetMetadata("key2")
	if !ok2 || val2 != 42 {
		t.Errorf("Expected key2=42, got %v (exists=%v)", val2, ok2)
	}

	val3, ok3 := ctx.GetMetadata("key3")
	if !ok3 || val3 != true {
		t.Errorf("Expected key3=true, got %v (exists=%v)", val3, ok3)
	}

	// Test non-existent key
	_, ok := ctx.GetMetadata("nonexistent")
	if ok {
		t.Error("Expected nonexistent key to return false")
	}
}

func TestRequestContext_SetTerminate(t *testing.T) {
	ctx := NewRequestContext("", nil, "", nil, nil)

	if ctx.Terminate {
		t.Error("Expected Terminate to be false initially")
	}

	ctx.SetTerminate()

	if !ctx.Terminate {
		t.Error("Expected Terminate to be true after SetTerminate()")
	}
}

func TestRequestContext_AddTool(t *testing.T) {
	ctx := NewRequestContext("", nil, "", nil, nil)

	tool1 := NewFunctionTool("tool1", "desc1", nil, nil)
	tool2 := NewFunctionTool("tool2", "desc2", nil, nil)

	ctx.AddTool(tool1)
	ctx.AddTool(tool2)

	if len(ctx.AdditionalTools) != 2 {
		t.Errorf("Expected 2 additional tools, got %d", len(ctx.AdditionalTools))
	}

	effectiveTools := ctx.GetEffectiveTools()
	if len(effectiveTools) != 2 {
		t.Errorf("Expected 2 effective tools, got %d", len(effectiveTools))
	}
}

func TestRequestContext_AddSystemPrompt(t *testing.T) {
	ctx := NewRequestContext("", nil, "Base prompt", nil, nil)

	ctx.AddSystemPrompt("Addition 1")
	ctx.AddSystemPrompt("Addition 2")
	ctx.AddSystemPrompt("") // Empty should be ignored

	// Empty strings are not added, so only 2 additions
	if len(ctx.SystemPromptAdditions) != 2 {
		t.Errorf("Expected 2 additions (empty ignored), got %d", len(ctx.SystemPromptAdditions))
	}

	effective := ctx.GetEffectiveSystemPrompt()
	expected := "Base prompt\n\nAddition 1\n\nAddition 2"
	if effective != expected {
		t.Errorf("Expected effective prompt %q, got %q", expected, effective)
	}
}

func TestRequestContext_GetEffectiveSystemPrompt_NoAdditions(t *testing.T) {
	ctx := NewRequestContext("", nil, "Base prompt only", nil, nil)

	effective := ctx.GetEffectiveSystemPrompt()
	if effective != "Base prompt only" {
		t.Errorf("Expected base prompt only, got %q", effective)
	}
}

func TestRequestContext_GetEffectiveTools(t *testing.T) {
	baseTool := NewFunctionTool("base", "base tool", nil, nil)
	ctx := NewRequestContext("", nil, "", []Tool{baseTool}, nil)

	// Initially, only base tool
	tools := ctx.GetEffectiveTools()
	if len(tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(tools))
	}

	// Add additional tools
	additionalTool := NewFunctionTool("additional", "additional tool", nil, nil)
	ctx.AddTool(additionalTool)

	tools = ctx.GetEffectiveTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
}

func TestRequestContext_Override(t *testing.T) {
	original := NewRequestContext(
		"thread-123",
		[]llm.ChatMessage{{Role: llm.ChatRoleUser, Contents: []llm.AIContent{llm.TextContent{Text: "Original"}}}},
		"Original prompt",
		nil,
		&AgentConfig{Name: "original"},
	)
	original.WithMetadata("key", "value")

	// Override messages
	newMessages := []llm.ChatMessage{
		{Role: llm.ChatRoleUser, Contents: []llm.AIContent{llm.TextContent{Text: "New"}}},
	}
	overridden := original.Override(WithMessages(newMessages))

	// Original should be unchanged
	if len(original.Messages) != 1 || original.Messages[0].Text() != "Original" {
		t.Error("Original context was modified")
	}

	// New context should have new messages
	if len(overridden.Messages) != 1 || overridden.Messages[0].Text() != "New" {
		t.Error("Overridden context doesn't have new messages")
	}

	// State should be shared
	if _, ok := overridden.GetMetadata("key"); !ok {
		t.Error("State was not shared between contexts")
	}

	// Override multiple fields
	multi := original.Override(
		WithSystemPrompt("New prompt"),
		WithResult("test result"),
	)

	if multi.SystemPrompt != "New prompt" {
		t.Errorf("Expected 'New prompt', got %q", multi.SystemPrompt)
	}
	if multi.Result != "test result" {
		t.Errorf("Expected 'test result', got %v", multi.Result)
	}
}

func TestMiddlewareFunc(t *testing.T) {
	called := false
	middlewareFunc := MiddlewareFunc(func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
		called = true

		return next(ctx, reqCtx)
	})

	ctx := NewRequestContext("", nil, "", nil, nil)
	err := middlewareFunc.Process(context.Background(), ctx, NoOpHandler)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !called {
		t.Error("Middleware function was not called")
	}
}

func TestChainMiddleware_Empty(t *testing.T) {
	baseHandlerCalled := false
	baseHandler := func(ctx context.Context, reqCtx *RequestContext) error {
		baseHandlerCalled = true

		return nil
	}

	handler := ChainMiddleware([]Middleware{}, baseHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	err := handler(context.Background(), ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !baseHandlerCalled {
		t.Error("Base handler was not called")
	}
}

func TestChainMiddleware_SingleMiddleware(t *testing.T) {
	m := &mockMiddleware{}
	baseHandlerCalled := false
	baseHandler := func(ctx context.Context, reqCtx *RequestContext) error {
		baseHandlerCalled = true

		return nil
	}

	handler := ChainMiddleware([]Middleware{m}, baseHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	err := handler(context.Background(), ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !m.beforeCalled {
		t.Error("Middleware before was not called")
	}
	if !m.afterCalled {
		t.Error("Middleware after was not called")
	}
	if !baseHandlerCalled {
		t.Error("Base handler was not called")
	}
}

func TestChainMiddleware_MultipleMiddleware_ExecutionOrder(t *testing.T) {
	executionOrder := []string{}

	m1 := &mockMiddleware{
		beforeFunc: func(rc *RequestContext) {
			executionOrder = append(executionOrder, "m1-before")
		},
		afterFunc: func(rc *RequestContext) {
			executionOrder = append(executionOrder, "m1-after")
		},
	}

	m2 := &mockMiddleware{
		beforeFunc: func(rc *RequestContext) {
			executionOrder = append(executionOrder, "m2-before")
		},
		afterFunc: func(rc *RequestContext) {
			executionOrder = append(executionOrder, "m2-after")
		},
	}

	m3 := &mockMiddleware{
		beforeFunc: func(rc *RequestContext) {
			executionOrder = append(executionOrder, "m3-before")
		},
		afterFunc: func(rc *RequestContext) {
			executionOrder = append(executionOrder, "m3-after")
		},
	}

	baseHandler := func(ctx context.Context, reqCtx *RequestContext) error {
		executionOrder = append(executionOrder, "base")

		return nil
	}

	handler := ChainMiddleware([]Middleware{m1, m2, m3}, baseHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	err := handler(context.Background(), ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	expected := []string{
		"m1-before", "m2-before", "m3-before",
		"base",
		"m3-after", "m2-after", "m1-after",
	}

	if len(executionOrder) != len(expected) {
		t.Fatalf("Expected %d execution steps, got %d: %v", len(expected), len(executionOrder), executionOrder)
	}

	for i, step := range expected {
		if executionOrder[i] != step {
			t.Errorf("Step %d: expected %q, got %q", i, step, executionOrder[i])
		}
	}
}

func TestChainMiddleware_MiddlewareCanModifyContext(t *testing.T) {
	m1 := &mockMiddleware{
		beforeFunc: func(rc *RequestContext) {
			rc.WithMetadata("m1", "set by m1")
		},
	}

	m2 := &mockMiddleware{
		beforeFunc: func(rc *RequestContext) {
			rc.WithMetadata("m2", "set by m2")
			// Can read what m1 set
			if val, ok := rc.GetMetadata("m1"); !ok || val != "set by m1" {
				panic("m2 could not read m1's metadata")
			}
		},
	}

	baseHandler := func(ctx context.Context, reqCtx *RequestContext) error {
		// Base handler can see all metadata
		val1, ok1 := reqCtx.GetMetadata("m1")
		val2, ok2 := reqCtx.GetMetadata("m2")
		if !ok1 || val1 != "set by m1" {
			return errors.New("base handler could not read m1 metadata")
		}
		if !ok2 || val2 != "set by m2" {
			return errors.New("base handler could not read m2 metadata")
		}

		return nil
	}

	handler := ChainMiddleware([]Middleware{m1, m2}, baseHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	err := handler(context.Background(), ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestChainMiddleware_MiddlewareCanShortCircuit(t *testing.T) {
	m1 := &mockMiddleware{}
	m2 := &mockMiddleware{
		beforeFunc: func(rc *RequestContext) {
			rc.SetTerminate()
		},
	}
	m3 := &mockMiddleware{}

	baseHandlerCalled := false
	baseHandler := func(ctx context.Context, reqCtx *RequestContext) error {
		baseHandlerCalled = true

		return nil
	}

	handler := ChainMiddleware([]Middleware{m1, m2, m3}, baseHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	err := handler(context.Background(), ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// m1 and m2 should be called (before), but m3 and base should not
	if !m1.beforeCalled {
		t.Error("m1 before should have been called")
	}
	if !m2.beforeCalled {
		t.Error("m2 before should have been called")
	}
	if m3.beforeCalled {
		t.Error("m3 before should not have been called (terminated)")
	}
	if baseHandlerCalled {
		t.Error("Base handler should not have been called (terminated)")
	}
}

func TestChainMiddleware_ErrorPropagation(t *testing.T) {
	m1 := &mockMiddleware{}
	m2 := &mockMiddleware{shouldError: true}
	m3 := &mockMiddleware{}

	baseHandlerCalled := false
	baseHandler := func(ctx context.Context, reqCtx *RequestContext) error {
		baseHandlerCalled = true

		return nil
	}

	handler := ChainMiddleware([]Middleware{m1, m2, m3}, baseHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	err := handler(context.Background(), ctx)
	if err == nil {
		t.Error("Expected error from middleware")
	}
	if err.Error() != "middleware error" {
		t.Errorf("Expected 'middleware error', got %v", err)
	}

	// m1 before should be called, but m2 error should stop the chain
	if !m1.beforeCalled {
		t.Error("m1 before should have been called")
	}
	if !m2.beforeCalled {
		t.Error("m2 before should have been called")
	}
	if m3.beforeCalled {
		t.Error("m3 should not have been called (error in m2)")
	}
	if baseHandlerCalled {
		t.Error("Base handler should not have been called (error in m2)")
	}
}

func TestChainMiddleware_BaseHandlerError(t *testing.T) {
	m := &mockMiddleware{}
	baseError := errors.New("base handler error")
	baseHandler := ErrorHandler(baseError)

	handler := ChainMiddleware([]Middleware{m}, baseHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	err := handler(context.Background(), ctx)
	if !errors.Is(err, baseError) {
		t.Errorf("Expected base handler error, got %v", err)
	}

	// Middleware before and after should both be called
	if !m.beforeCalled {
		t.Error("Middleware before should have been called")
	}
	if !m.afterCalled {
		t.Error("Middleware after should have been called even with base error")
	}
}

func TestNoOpHandler(t *testing.T) {
	ctx := NewRequestContext("", nil, "", nil, nil)
	err := NoOpHandler(context.Background(), ctx)
	if err != nil {
		t.Errorf("NoOpHandler should not return error, got %v", err)
	}
}

func TestErrorHandler(t *testing.T) {
	expectedError := errors.New("test error")
	handler := ErrorHandler(expectedError)

	ctx := NewRequestContext("", nil, "", nil, nil)
	err := handler(context.Background(), ctx)
	if !errors.Is(err, expectedError) {
		t.Errorf("Expected error %v, got %v", expectedError, err)
	}
}

func TestResultHandler(t *testing.T) {
	expectedResult := "test result"
	handler := ResultHandler(expectedResult)

	ctx := NewRequestContext("", nil, "", nil, nil)
	err := handler(context.Background(), ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if ctx.Result != expectedResult {
		t.Errorf("Expected result %q, got %v", expectedResult, ctx.Result)
	}
}

func TestMiddleware_CanAccessResultAfterNext(t *testing.T) {
	expectedResult := "final result"
	resultSeen := false

	m := &mockMiddleware{
		afterFunc: func(rc *RequestContext) {
			if rc.Result == expectedResult {
				resultSeen = true
			}
		},
	}

	handler := ChainMiddleware([]Middleware{m}, ResultHandler(expectedResult))
	ctx := NewRequestContext("", nil, "", nil, nil)

	err := handler(context.Background(), ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if !resultSeen {
		t.Error("Middleware did not see the result after next()")
	}
	if ctx.Result != expectedResult {
		t.Errorf("Expected final result %q, got %v", expectedResult, ctx.Result)
	}
}

func TestMiddleware_CanAddToolsDynamically(t *testing.T) {
	tool1 := NewFunctionTool("tool1", "Tool 1", nil, nil)
	tool2 := NewFunctionTool("tool2", "Tool 2", nil, nil)

	m := &mockMiddleware{
		beforeFunc: func(rc *RequestContext) {
			rc.AddTool(tool1)
			rc.AddTool(tool2)
		},
	}

	handler := ChainMiddleware([]Middleware{m}, NoOpHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	err := handler(context.Background(), ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	effectiveTools := ctx.GetEffectiveTools()
	if len(effectiveTools) != 2 {
		t.Errorf("Expected 2 tools after middleware, got %d", len(effectiveTools))
	}
}

func TestMiddleware_CanAddSystemPromptDynamically(t *testing.T) {
	m := &mockMiddleware{
		beforeFunc: func(rc *RequestContext) {
			rc.AddSystemPrompt("Additional instructions from middleware")
		},
	}

	handler := ChainMiddleware([]Middleware{m}, NoOpHandler)
	ctx := NewRequestContext("", nil, "Base instructions", nil, nil)

	err := handler(context.Background(), ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	effective := ctx.GetEffectiveSystemPrompt()
	expected := "Base instructions\n\nAdditional instructions from middleware"
	if effective != expected {
		t.Errorf("Expected %q, got %q", expected, effective)
	}
}

// Benchmark middleware chain performance.
func BenchmarkChainMiddleware_NoMiddleware(b *testing.B) {
	handler := ChainMiddleware([]Middleware{}, NoOpHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	for b.Loop() {
		_ = handler(context.Background(), ctx)
	}
}

func BenchmarkChainMiddleware_SingleMiddleware(b *testing.B) {
	m := &mockMiddleware{}
	handler := ChainMiddleware([]Middleware{m}, NoOpHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	for b.Loop() {
		_ = handler(context.Background(), ctx)
	}
}

func BenchmarkChainMiddleware_TenMiddleware(b *testing.B) {
	middlewares := make([]Middleware, 10)
	for i := range 10 {
		middlewares[i] = &mockMiddleware{}
	}
	handler := ChainMiddleware(middlewares, NoOpHandler)
	ctx := NewRequestContext("", nil, "", nil, nil)

	for b.Loop() {
		_ = handler(context.Background(), ctx)
	}
}
