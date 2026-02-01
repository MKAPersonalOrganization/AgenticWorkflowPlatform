package core_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
)

var errTestToolNotFound = errors.New("tool not found")

// MockToolExecutor is a test double for ToolExecutor.
type MockToolExecutor struct {
	results map[string]string
	errors  map[string]error
	calls   []core.ToolCall
}

func NewMockToolExecutor() *MockToolExecutor {
	return &MockToolExecutor{
		results: make(map[string]string),
		errors:  make(map[string]error),
		calls:   []core.ToolCall{},
	}
}

func (m *MockToolExecutor) Execute(toolCall core.ToolCall) (core.ToolExecResult, error) {
	m.calls = append(m.calls, toolCall)

	if err, ok := m.errors[toolCall.Name]; ok {
		return core.ToolExecResult{}, err
	}

	var result string
	if res, ok := m.results[toolCall.Name]; ok {
		result = res
	} else {
		result = `{"status": "ok"}`
	}

	return core.ToolExecResult{Result: result}, nil
}

func (m *MockToolExecutor) SetResult(toolName, result string) {
	m.results[toolName] = result
}

func (m *MockToolExecutor) SetError(toolName string, err error) {
	m.errors[toolName] = err
}

func (m *MockToolExecutor) CallCount() int {
	return len(m.calls)
}

func (m *MockToolExecutor) LastCall() core.ToolCall {
	if len(m.calls) == 0 {
		return core.ToolCall{}
	}

	return m.calls[len(m.calls)-1]
}

// Verify MockToolExecutor implements ToolExecutor.
var _ core.ToolExecutor = (*MockToolExecutor)(nil)

func TestMockToolExecutor_Execute(t *testing.T) {
	t.Parallel()
	executor := NewMockToolExecutor()
	executor.SetResult("get_weather", `{"temperature": 72}`)

	toolCall := core.ToolCall{
		ID:        "call_123",
		Type:      "function",
		Name:      "get_weather",
		Arguments: `{"city": "London"}`,
	}

	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Result != `{"temperature": 72}` {
		t.Errorf("expected temperature result, got %s", result.Result)
	}

	if executor.CallCount() != 1 {
		t.Errorf("expected 1 call, got %d", executor.CallCount())
	}
}

func TestMockToolExecutor_ExecuteWithError(t *testing.T) {
	t.Parallel()
	executor := NewMockToolExecutor()
	expectedErr := errTestToolNotFound
	executor.SetError("unknown_tool", expectedErr)

	toolCall := core.ToolCall{
		ID:        "call_456",
		Type:      "function",
		Name:      "unknown_tool",
		Arguments: `{}`,
	}

	_, err := executor.Execute(toolCall)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("expected %v, got %v", expectedErr, err)
	}
}

func TestMockToolExecutor_DefaultResult(t *testing.T) {
	t.Parallel()
	executor := NewMockToolExecutor()

	toolCall := core.ToolCall{
		ID:        "call_789",
		Type:      "function",
		Name:      "any_tool",
		Arguments: `{}`,
	}

	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Result != `{"status": "ok"}` {
		t.Errorf("expected default result, got %s", result.Result)
	}
}

func TestBasicToolExecutor_Calculate(t *testing.T) {
	t.Parallel()
	// Create tool registry and register a mock calculator tool
	toolRegistry := core.NewToolRegistry()
	toolRegistry.Register(core.NewFunctionTool(
		"calculate",
		"Calculate an expression",
		core.ObjectSchema(map[string]any{"expression": core.StringProperty("Expression")}, []string{"expression"}),
		func(ctx core.ToolContext, args any) (any, error) {
			return core.NewToolResult(`{"result": "4"}`), nil
		},
	))

	executor := core.NewRegistryToolExecutor(context.Background(), toolRegistry, "", "", nil)

	toolCall := core.ToolCall{
		ID:        "call_calc",
		Type:      "function",
		Name:      "calculate",
		Arguments: `{"expression": "2 + 2"}`,
	}

	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBasicToolExecutor_CustomTool(t *testing.T) {
	t.Parallel()
	// Create tool registry and register a mock custom tool
	toolRegistry := core.NewToolRegistry()
	toolRegistry.Register(core.NewFunctionTool(
		"save_note",
		"Save a note",
		core.ObjectSchema(map[string]any{
			"content": core.StringProperty("Content"),
			"action":  core.StringProperty("Action"),
		}, []string{"content", "action"}),
		func(ctx core.ToolContext, args any) (any, error) {
			return core.NewToolResult(`{"status": "success"}`), nil
		},
	))

	executor := core.NewRegistryToolExecutor(context.Background(), toolRegistry, "", "", nil)

	toolCall := core.ToolCall{
		ID:        "call_note",
		Type:      "function",
		Name:      "save_note",
		Arguments: `{"content": "test content", "action": "write"}`,
	}

	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Result == "" {
		t.Error("expected non-empty result from custom tool")
	}
}

func TestRegistryToolExecutor(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	registry := core.NewToolRegistry()

	// Register a test tool
	registry.Register(core.NewFunctionTool(
		"greet",
		"Greet someone",
		core.ObjectSchema(
			map[string]any{"name": core.StringProperty("The name")},
			[]string{"name"},
		),
		func(ctx core.ToolContext, args any) (any, error) {
			var params struct {
				Name string `json:"name"`
			}
			err := core.UnmarshalArgs(args, &params)
			if err != nil {
				return nil, fmt.Errorf("failed to unmarshal arguments: %w", err)
			}

			return map[string]any{"message": "Hello, " + params.Name + "!"}, nil
		},
	))

	executor := core.NewRegistryToolExecutor(ctx, registry, "", "", nil)

	// Test successful execution
	result, err := executor.Execute(core.ToolCall{
		ID:        "call_1",
		Type:      "function",
		Name:      "greet",
		Arguments: `{"name": "World"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Result != `{"message":"Hello, World!"}` {
		t.Errorf("unexpected result: %s", result.Result)
	}
}

func TestRegistryToolExecutor_ToolNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	registry := core.NewToolRegistry()
	executor := core.NewRegistryToolExecutor(ctx, registry, "", "", nil)

	_, err := executor.Execute(core.ToolCall{
		ID:        "call_1",
		Type:      "function",
		Name:      "nonexistent",
		Arguments: `{}`,
	})

	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
}
