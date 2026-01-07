package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents/shared/tools"
)

// MockToolExecutor is a test double for ToolExecutor
type MockToolExecutor struct {
	results map[string]string
	errors  map[string]error
	calls   []domain.ToolCall
}

func NewMockToolExecutor() *MockToolExecutor {
	return &MockToolExecutor{
		results: make(map[string]string),
		errors:  make(map[string]error),
		calls:   []domain.ToolCall{},
	}
}

func (m *MockToolExecutor) Execute(toolCall domain.ToolCall) (string, error) {
	m.calls = append(m.calls, toolCall)

	if err, ok := m.errors[toolCall.Function.Name]; ok {
		return "", err
	}

	if result, ok := m.results[toolCall.Function.Name]; ok {
		return result, nil
	}

	return `{"status": "ok"}`, nil
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

func (m *MockToolExecutor) LastCall() domain.ToolCall {
	if len(m.calls) == 0 {
		return domain.ToolCall{}
	}
	return m.calls[len(m.calls)-1]
}

// Verify MockToolExecutor implements ToolExecutor
var _ core.ToolExecutor = (*MockToolExecutor)(nil)

func TestMockToolExecutor_Execute(t *testing.T) {
	executor := NewMockToolExecutor()
	executor.SetResult("get_weather", `{"temperature": 72}`)

	toolCall := domain.ToolCall{
		ID:   "call_123",
		Type: "function",
		Function: domain.ToolFunction{
			Name:      "get_weather",
			Arguments: `{"city": "London"}`,
		},
	}

	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != `{"temperature": 72}` {
		t.Errorf("expected temperature result, got %s", result)
	}

	if executor.CallCount() != 1 {
		t.Errorf("expected 1 call, got %d", executor.CallCount())
	}
}

func TestMockToolExecutor_ExecuteWithError(t *testing.T) {
	executor := NewMockToolExecutor()
	expectedErr := errors.New("tool not found")
	executor.SetError("unknown_tool", expectedErr)

	toolCall := domain.ToolCall{
		ID:   "call_456",
		Type: "function",
		Function: domain.ToolFunction{
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
	executor := NewMockToolExecutor()

	toolCall := domain.ToolCall{
		ID:   "call_789",
		Type: "function",
		Function: domain.ToolFunction{
			Name:      "any_tool",
			Arguments: `{}`,
		},
	}

	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != `{"status": "ok"}` {
		t.Errorf("expected default result, got %s", result)
	}
}

func TestBasicToolExecutor_Calculate(t *testing.T) {
	// Create tool registry and register the calculator tool
	toolRegistry := core.NewToolRegistry()
	toolRegistry.Register(tools.NewCalculatorTool())

	executor := core.NewRegistryToolExecutor(context.Background(), toolRegistry)

	toolCall := domain.ToolCall{
		ID:   "call_calc",
		Type: "function",
		Function: domain.ToolFunction{
			Name:      "calculate",
			Arguments: `{"expression": "2 + 2"}`,
		},
	}

	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestBasicToolExecutor_WriteWhiteboard(t *testing.T) {
	// Create tool registry and register instructor tools
	toolRegistry := core.NewToolRegistry()
	// Register whiteboard tool
	toolRegistry.Register(tools.NewWriteWhiteboardTool())

	executor := core.NewRegistryToolExecutor(context.Background(), toolRegistry)

	toolCall := domain.ToolCall{
		ID:   "call_whiteboard",
		Type: "function",
		Function: domain.ToolFunction{
			Name:      "write_whiteboard",
			Arguments: `{"content": "test content", "action": "write"}`,
		},
	}

	// write_whiteboard is an instructor tool
	result, err := executor.Execute(toolCall)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == "" {
		t.Error("expected non-empty result from instructor tool")
	}
}

func TestRegistryToolExecutor(t *testing.T) {
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
		func(ctx context.Context, arguments string) (string, error) {
			var args struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal([]byte(arguments), &args); err != nil {
				return "", err
			}
			return `{"message": "Hello, ` + args.Name + `!"}`, nil
		},
	))

	executor := core.NewRegistryToolExecutor(ctx, registry)

	// Test successful execution
	result, err := executor.Execute(domain.ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: domain.ToolFunction{
			Name:      "greet",
			Arguments: `{"name": "World"}`,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result != `{"message": "Hello, World!"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestRegistryToolExecutor_ToolNotFound(t *testing.T) {
	ctx := context.Background()
	registry := core.NewToolRegistry()
	executor := core.NewRegistryToolExecutor(ctx, registry)

	_, err := executor.Execute(domain.ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: domain.ToolFunction{
			Name:      "nonexistent",
			Arguments: `{}`,
		},
	})

	if err == nil {
		t.Fatal("expected error for nonexistent tool")
	}
}
