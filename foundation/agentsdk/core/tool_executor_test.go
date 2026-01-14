package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"school-with-ai/foundation/agentsdk/core"
	"testing"
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

func (m *MockToolExecutor) Execute(toolCall core.ToolCall) (core.ToolExecutionResult, error) {
	m.calls = append(m.calls, toolCall)

	if err, ok := m.errors[toolCall.Function.Name]; ok {
		return core.ToolExecutionResult{}, err
	}

	var result string
	if res, ok := m.results[toolCall.Function.Name]; ok {
		result = res
	} else {
		result = `{"status": "ok"}`
	}

	return core.ToolExecutionResult{Result: result}, nil
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
		ID:   "call_123",
		Type: "function",
		Function: core.ToolFunction{
			Name:      "get_weather",
			Arguments: `{"city": "London"}`,
		},
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
		ID:   "call_456",
		Type: "function",
		Function: core.ToolFunction{
			Name:      "unknown_tool",
			Arguments: `{}`,
		},
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
		ID:   "call_789",
		Type: "function",
		Function: core.ToolFunction{
			Name:      "any_tool",
			Arguments: `{}`,
		},
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
		func(ctx context.Context, arguments string) (core.ToolResult, error) {
			return core.NewToolResult(`{"result": "4"}`), nil
		},
	))

	executor := core.NewRegistryToolExecutor(context.Background(), toolRegistry)

	toolCall := core.ToolCall{
		ID:   "call_calc",
		Type: "function",
		Function: core.ToolFunction{
			Name:      "calculate",
			Arguments: `{"expression": "2 + 2"}`,
		},
	}

	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBasicToolExecutor_WriteWhiteboard(t *testing.T) {
	t.Parallel()
	// Create tool registry and register a mock whiteboard tool
	toolRegistry := core.NewToolRegistry()
	toolRegistry.Register(core.NewFunctionTool(
		"write_whiteboard",
		"Write to whiteboard",
		core.ObjectSchema(map[string]any{
			"content": core.StringProperty("Content"),
			"action":  core.StringProperty("Action"),
		}, []string{"content", "action"}),
		func(ctx context.Context, arguments string) (core.ToolResult, error) {
			return core.NewToolResult(`{"status": "success"}`), nil
		},
	))

	executor := core.NewRegistryToolExecutor(context.Background(), toolRegistry)

	toolCall := core.ToolCall{
		ID:   "call_whiteboard",
		Type: "function",
		Function: core.ToolFunction{
			Name:      "write_whiteboard",
			Arguments: `{"content": "test content", "action": "write"}`,
		},
	}

	// write_whiteboard is an instructor tool
	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Result == "" {
		t.Error("expected non-empty result from instructor tool")
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
		func(ctx context.Context, arguments string) (core.ToolResult, error) {
			var args struct {
				Name string `json:"name"`
			}
			err := json.Unmarshal([]byte(arguments), &args)
			if err != nil {
				return core.ToolResult{}, fmt.Errorf("failed to unmarshal arguments: %w", err)
			}

			return core.NewToolResult(`{"message": "Hello, ` + args.Name + `!"}`), nil
		},
	))

	executor := core.NewRegistryToolExecutor(ctx, registry)

	// Test successful execution
	result, err := executor.Execute(core.ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: core.ToolFunction{
			Name:      "greet",
			Arguments: `{"name": "World"}`,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Result != `{"message": "Hello, World!"}` {
		t.Errorf("unexpected result: %s", result.Result)
	}
}

func TestRegistryToolExecutor_ToolNotFound(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	registry := core.NewToolRegistry()
	executor := core.NewRegistryToolExecutor(ctx, registry)

	_, err := executor.Execute(core.ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: core.ToolFunction{
			Name:      "nonexistent",
			Arguments: `{}`,
		},
	})

	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
}
