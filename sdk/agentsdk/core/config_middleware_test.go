package core

import (
	"context"
	"school-with-ai/sdk/agentsdk/llm"
	"testing"
)

// MockTool is a simple tool implementation for testing.
type MockTool struct {
	name        string
	description string
}

func NewMockTool(name, description string) *MockTool {
	return &MockTool{
		name:        name,
		description: description,
	}
}

func (m *MockTool) Name() string {
	return m.name
}

func (m *MockTool) Description() string {
	return m.description
}

func (m *MockTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{
				"type":        "string",
				"description": "Input parameter",
			},
		},
	}
}

func (m *MockTool) Invoke(ctx context.Context, arguments string) (string, error) {
	return "mock result", nil
}

// TestToChatOptionsWithMiddleware_WithoutMiddleware ensures backward compatibility.
func TestToChatOptionsWithMiddleware_WithoutMiddleware(t *testing.T) {
	config := AgentConfig{
		Name:         "test-agent",
		SystemPrompt: "You are a test assistant",
		Tools:        []Tool{NewMockTool("tool1", "description1")},
		Middleware:   []Middleware{}, // No middleware
	}

	runOpts := RunOptions{}

	opts, _, err := ToChatOptionsWithMiddleware(context.Background(), config, []llm.ChatMessage{}, "", runOpts)
	if err != nil {
		t.Fatalf("ToChatOptionsWithMiddleware failed: %v", err)
	}

	if opts.Instructions != "You are a test assistant" {
		t.Errorf("Expected system prompt, got %s", opts.Instructions)
	}

	if len(opts.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(opts.Tools))
	}
}

// TestApplyMiddleware_NoMiddleware ensures ApplyMiddleware works without middleware.
func TestApplyMiddleware_NoMiddleware(t *testing.T) {
	config := AgentConfig{
		Name:         "test-agent",
		SystemPrompt: "Original prompt",
		Tools:        []Tool{NewMockTool("tool1", "desc1")},
		Middleware:   []Middleware{}, // No middleware
	}

	messages := []llm.ChatMessage{
		llm.NewUserMessage("Hello"),
	}

	reqCtx, err := ApplyMiddleware(context.Background(), config, messages, "", RunOptions{})
	if err != nil {
		t.Fatalf("ApplyMiddleware failed: %v", err)
	}

	if reqCtx.SystemPrompt != "Original prompt" {
		t.Errorf("Expected original prompt, got %s", reqCtx.SystemPrompt)
	}

	if len(reqCtx.Tools) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(reqCtx.Tools))
	}

	if reqCtx.Terminate {
		t.Error("Expected Terminate=false without middleware")
	}
}

// TestApplyMiddleware_WithToolInjection tests middleware adding tools.
func TestApplyMiddleware_WithToolInjection(t *testing.T) {
	// Create middleware that adds a tool
	toolAdderMiddleware := MiddlewareFunc(func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
		reqCtx.AddTool(NewMockTool("injected_tool", "Middleware-injected tool"))

		return next(ctx, reqCtx)
	})

	config := AgentConfig{
		Name:         "test-agent",
		SystemPrompt: "Original prompt",
		Tools:        []Tool{NewMockTool("original_tool", "Original tool")},
		Middleware:   []Middleware{toolAdderMiddleware},
	}

	messages := []llm.ChatMessage{
		llm.NewUserMessage("Hello"),
	}

	reqCtx, err := ApplyMiddleware(context.Background(), config, messages, "", RunOptions{})
	if err != nil {
		t.Fatalf("ApplyMiddleware failed: %v", err)
	}

	// Should have both original and injected tool
	effectiveTools := reqCtx.GetEffectiveTools()
	if len(effectiveTools) != 2 {
		t.Errorf("Expected 2 tools (original + injected), got %d", len(effectiveTools))
	}

	// Verify tools are present
	toolNames := make(map[string]bool)
	for _, tool := range effectiveTools {
		toolNames[tool.Name()] = true
	}

	if !toolNames["original_tool"] {
		t.Error("Missing original_tool")
	}
	if !toolNames["injected_tool"] {
		t.Error("Missing injected_tool")
	}
}

// TestApplyMiddleware_WithSystemPromptAddition tests middleware adding to system prompt.
func TestApplyMiddleware_WithSystemPromptAddition(t *testing.T) {
	// Create middleware that adds to system prompt
	promptAdderMiddleware := MiddlewareFunc(func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
		reqCtx.AddSystemPrompt("IMPORTANT: Always be concise.")

		return next(ctx, reqCtx)
	})

	config := AgentConfig{
		Name:         "test-agent",
		SystemPrompt: "You are a helpful assistant.",
		Tools:        []Tool{},
		Middleware:   []Middleware{promptAdderMiddleware},
	}

	messages := []llm.ChatMessage{
		llm.NewUserMessage("Hello"),
	}

	reqCtx, err := ApplyMiddleware(context.Background(), config, messages, "", RunOptions{})
	if err != nil {
		t.Fatalf("ApplyMiddleware failed: %v", err)
	}

	effectivePrompt := reqCtx.GetEffectiveSystemPrompt()
	expected := "You are a helpful assistant.\n\nIMPORTANT: Always be concise."

	if effectivePrompt != expected {
		t.Errorf("Expected combined prompt, got:\n%s\n\nExpected:\n%s", effectivePrompt, expected)
	}
}

// TestApplyMiddleware_Termination tests middleware terminating early.
func TestApplyMiddleware_Termination(t *testing.T) {
	// Create middleware that terminates
	terminatorMiddleware := MiddlewareFunc(func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
		reqCtx.SetTerminate()
		reqCtx.Result = "Terminated by middleware"
		// Don't call next - terminate early
		return nil
	})

	config := AgentConfig{
		Name:       "test-agent",
		Middleware: []Middleware{terminatorMiddleware},
	}

	messages := []llm.ChatMessage{
		llm.NewUserMessage("Hello"),
	}

	reqCtx, err := ApplyMiddleware(context.Background(), config, messages, "", RunOptions{})
	if err != nil {
		t.Fatalf("ApplyMiddleware failed: %v", err)
	}

	if !reqCtx.Terminate {
		t.Error("Expected Terminate=true")
	}

	if reqCtx.Result != "Terminated by middleware" {
		t.Errorf("Expected result to be set, got %v", reqCtx.Result)
	}
}

// TestToChatOptionsWithMiddleware tests the integrated middleware flow.
func TestToChatOptionsWithMiddleware(t *testing.T) {
	// Create middleware that adds tool and system prompt
	enrichmentMiddleware := MiddlewareFunc(func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
		reqCtx.AddTool(NewMockTool("middleware_tool", "Added by middleware"))
		reqCtx.AddSystemPrompt("\n\nNote: Middleware is active.")

		return next(ctx, reqCtx)
	})

	config := AgentConfig{
		Name:         "test-agent",
		SystemPrompt: "Base prompt",
		Tools:        []Tool{NewMockTool("base_tool", "Base tool")},
		Middleware:   []Middleware{enrichmentMiddleware},
	}

	messages := []llm.ChatMessage{
		llm.NewUserMessage("Test message"),
	}

	opts, reqCtx, err := ToChatOptionsWithMiddleware(context.Background(), config, messages, "", RunOptions{})
	if err != nil {
		t.Fatalf("ToChatOptionsWithMiddleware failed: %v", err)
	}

	// Check system prompt was augmented
	// Note: Current implementation adds \n\n\n\n between base and additions
	expectedPrompt := "Base prompt\n\n\n\nNote: Middleware is active."
	if opts.Instructions != expectedPrompt {
		t.Errorf("Expected augmented prompt, got %s", opts.Instructions)
	}

	// Check tools were augmented
	if len(opts.Tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(opts.Tools))
	}

	// Verify reqCtx is returned correctly
	if reqCtx == nil {
		t.Fatal("Expected reqCtx to be returned")
	}

	if reqCtx.Terminate {
		t.Error("Expected Terminate=false")
	}
}

// TestToChatOptionsWithMiddleware_Termination tests early termination.
func TestToChatOptionsWithMiddleware_Termination(t *testing.T) {
	terminatorMiddleware := MiddlewareFunc(func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
		reqCtx.SetTerminate()

		return nil
	})

	config := AgentConfig{
		Name:       "test-agent",
		Middleware: []Middleware{terminatorMiddleware},
	}

	messages := []llm.ChatMessage{
		llm.NewUserMessage("Test"),
	}

	opts, reqCtx, err := ToChatOptionsWithMiddleware(context.Background(), config, messages, "", RunOptions{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if opts == nil {
		t.Fatal("Expected opts to be returned even with termination")
	}

	if !reqCtx.Terminate {
		t.Error("Expected Terminate=true in returned context")
	}
}

// TestApplyMiddleware_ConfigParameters tests that config parameters are copied to reqCtx.
func TestApplyMiddleware_ConfigParameters(t *testing.T) {
	temp := 0.7
	maxTokens := 1000
	topP := 0.9
	seed := int64(42)

	config := AgentConfig{
		Name:        "test-agent",
		Temperature: &temp,
		MaxTokens:   &maxTokens,
		TopP:        &topP,
		Seed:        &seed,
		ModelID:     "gpt-4",
		Middleware:  []Middleware{},
	}

	messages := []llm.ChatMessage{
		llm.NewUserMessage("Test"),
	}

	reqCtx, err := ApplyMiddleware(context.Background(), config, messages, "", RunOptions{})
	if err != nil {
		t.Fatalf("ApplyMiddleware failed: %v", err)
	}

	// Verify config parameters were copied
	if reqCtx.Config.Temperature == nil || *reqCtx.Config.Temperature != 0.7 {
		t.Error("Temperature not copied correctly")
	}
	if reqCtx.Config.MaxTokens == nil || *reqCtx.Config.MaxTokens != 1000 {
		t.Error("MaxTokens not copied correctly")
	}
	if reqCtx.Config.TopP == nil || *reqCtx.Config.TopP != 0.9 {
		t.Error("TopP not copied correctly")
	}
	if reqCtx.Config.Seed == nil || *reqCtx.Config.Seed != 42 {
		t.Error("Seed not copied correctly")
	}
	if reqCtx.Config.ModelID != "gpt-4" {
		t.Error("ModelID not copied correctly")
	}
}

// TestApplyMiddleware_RunOptionsOverride tests that run options override config.
func TestApplyMiddleware_RunOptionsOverride(t *testing.T) {
	configTemp := 0.5
	runTemp := 0.9
	configModel := "gpt-3.5"
	runModel := "gpt-4"

	config := AgentConfig{
		Name:        "test-agent",
		Temperature: &configTemp,
		ModelID:     configModel,
		Middleware:  []Middleware{},
	}

	runOpts := RunOptions{
		Temperature: &runTemp,
		ModelID:     runModel,
	}

	messages := []llm.ChatMessage{
		llm.NewUserMessage("Test"),
	}

	reqCtx, err := ApplyMiddleware(context.Background(), config, messages, "", runOpts)
	if err != nil {
		t.Fatalf("ApplyMiddleware failed: %v", err)
	}

	// Run options should override config
	if reqCtx.Config.Temperature == nil || *reqCtx.Config.Temperature != 0.9 {
		t.Error("Run options temperature should override config")
	}
	if reqCtx.Config.ModelID != "gpt-4" {
		t.Error("Run options ModelID should override config")
	}
}

// TestApplyMiddleware_MultipleMiddleware tests middleware chain execution order.
func TestApplyMiddleware_MultipleMiddleware(t *testing.T) {
	var executionOrder []string

	middleware1 := MiddlewareFunc(func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
		executionOrder = append(executionOrder, "m1-before")
		reqCtx.AddTool(NewMockTool("tool1", "From middleware 1"))
		err := next(ctx, reqCtx)
		executionOrder = append(executionOrder, "m1-after")

		return err
	})

	middleware2 := MiddlewareFunc(func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
		executionOrder = append(executionOrder, "m2-before")
		reqCtx.AddTool(NewMockTool("tool2", "From middleware 2"))
		err := next(ctx, reqCtx)
		executionOrder = append(executionOrder, "m2-after")

		return err
	})

	config := AgentConfig{
		Name:       "test-agent",
		Middleware: []Middleware{middleware1, middleware2},
	}

	messages := []llm.ChatMessage{
		llm.NewUserMessage("Test"),
	}

	reqCtx, err := ApplyMiddleware(context.Background(), config, messages, "", RunOptions{})
	if err != nil {
		t.Fatalf("ApplyMiddleware failed: %v", err)
	}

	// Verify execution order (first middleware is outermost)
	expectedOrder := []string{"m1-before", "m2-before", "m2-after", "m1-after"}
	if len(executionOrder) != len(expectedOrder) {
		t.Fatalf("Expected %d execution steps, got %d", len(expectedOrder), len(executionOrder))
	}

	for i, expected := range expectedOrder {
		if executionOrder[i] != expected {
			t.Errorf("At step %d, expected %s, got %s", i, expected, executionOrder[i])
		}
	}

	// Verify both tools were added
	tools := reqCtx.GetEffectiveTools()
	if len(tools) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(tools))
	}
}
