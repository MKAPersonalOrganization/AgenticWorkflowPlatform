package engine

import (
	"context"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
)

// MockToolExecutor is a mock implementation of ToolExecutor for testing.
type MockToolExecutor struct {
	executeFunc func(toolCall core.ToolCall) (core.ToolExecResult, error)
}

func (m *MockToolExecutor) Execute(toolCall core.ToolCall) (core.ToolExecResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(toolCall)
	}
	return core.ToolExecResult{Result: "success"}, nil
}

// MockToolStateObserver records state transitions for testing.
type MockToolStateObserver struct {
	transitions []struct {
		toolCallID  string
		toolName    string
		oldState    core.ToolState
		newState    core.ToolState
		result      *core.ToolExecutionResult
		elicitation *core.ElicitationSpec
	}
}

func (m *MockToolStateObserver) OnToolStateChanged(
	toolCallID string,
	toolName string,
	oldState core.ToolState,
	newState core.ToolState,
	result *core.ToolExecutionResult,
	elicitation *core.ElicitationSpec,
) {
	m.transitions = append(m.transitions, struct {
		toolCallID  string
		toolName    string
		oldState    core.ToolState
		newState    core.ToolState
		result      *core.ToolExecutionResult
		elicitation *core.ElicitationSpec
	}{
		toolCallID:  toolCallID,
		toolName:    toolName,
		oldState:    oldState,
		newState:    newState,
		result:      result,
		elicitation: elicitation,
	})
}

func TestToolStateMachine_NormalExecution(t *testing.T) {
	// Test: Tool executes successfully without interaction
	// Expected: Idle → Executing → Completed

	ctx := context.Background()
	toolCall := &core.ToolCall{
		ID:        "tc_123",
		Type:      "function",
		Name:      "calculator",
		Arguments: `{"operation": "add", "a": 1, "b": 2}`,
	}

	observer := &MockToolStateObserver{}

	tsm := core.NewToolStateMachine(toolCall)
	tsm.AddObserver(observer)

	// Start execution
	err := tsm.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify final state
	if state := tsm.CurrentState(ctx); state != core.ToolStateCompleted {
		t.Errorf("Expected state Completed, got %v", state)
	}

	// Verify result
	result := tsm.Result()
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Result != "3" {
		t.Errorf("Expected result '3', got '%s'", result.Result)
	}

	// Verify state transitions: Idle -> Executing -> Completed
	if len(observer.transitions) != 2 {
		t.Errorf("Expected 2 transitions, got %d", len(observer.transitions))
	}
	if len(observer.transitions) >= 2 {
		trans := observer.transitions[1]
		if trans.newState != core.ToolStateCompleted {
			t.Errorf("Expected transition to Completed, got %v", trans.newState)
		}
	}
}

func TestToolStateMachine_AwaitingInput(t *testing.T) {
	// Test: Tool requests elicitation
	// Expected: Idle → Executing → AwaitingInput

	ctx := context.Background()
	toolCall := &core.ToolCall{
		ID:        "tc_456",
		Type:      "function",
		Name:      "elicitation_tool",
		Arguments: `{}`,
	}

	elicitation := &core.ElicitationSpec{
		Question: "What is your name?",
		Schema:   map[string]any{"type": "string"},
	}

	observer := &MockToolStateObserver{}

	tsm := core.NewToolStateMachine(toolCall)
	tsm.AddObserver(observer)

	// Start execution
	err := tsm.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify state
	if state := tsm.CurrentState(ctx); state != core.ToolStateAwaitingInput {
		t.Errorf("Expected state AwaitingInput, got %v", state)
	}

	// Verify elicitation
	if tsm.Elicitation() == nil {
		t.Fatal("Expected elicitation spec, got nil")
	}
	if tsm.Elicitation().Question != elicitation.Question {
		t.Errorf("Expected question '%s', got '%s'", elicitation.Question, tsm.Elicitation().Question)
	}

	// Verify it's awaiting interaction
	if !tsm.IsAwaitingInteraction(ctx) {
		t.Error("Expected tool to be awaiting interaction")
	}

	// Verify state transitions: Idle -> Executing -> AwaitingInput
	if len(observer.transitions) != 2 {
		t.Errorf("Expected 2 transitions, got %d", len(observer.transitions))
	}
	if len(observer.transitions) >= 2 {
		trans := observer.transitions[1]
		if trans.newState != core.ToolStateAwaitingInput {
			t.Errorf("Expected transition to AwaitingInput, got %v", trans.newState)
		}
		if trans.elicitation == nil {
			t.Error("Expected elicitation in transition")
		}
	}
}

func TestToolStateMachine_AwaitingAuth(t *testing.T) {
	// Test: Tool requests authorization
	// Expected: Idle → Executing → AwaitingAuth

	ctx := context.Background()
	toolCall := &core.ToolCall{
		ID:        "tc_789",
		Type:      "function",
		Name:      "file_access",
		Arguments: `{"path": "/etc/passwd"}`,
	}

	observer := &MockToolStateObserver{}

	tsm := core.NewToolStateMachine(toolCall)
	tsm.AddObserver(observer)

	// Start execution
	err := tsm.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify state
	if state := tsm.CurrentState(ctx); state != core.ToolStateAwaitingAuth {
		t.Errorf("Expected state AwaitingAuth, got %v", state)
	}

	// Verify it's awaiting interaction
	if !tsm.IsAwaitingInteraction(ctx) {
		t.Error("Expected tool to be awaiting interaction")
	}

	// Verify state transitions: Idle -> Executing -> AwaitingAuth
	if len(observer.transitions) != 2 {
		t.Errorf("Expected 2 transitions, got %d", len(observer.transitions))
	}
	if len(observer.transitions) >= 2 {
		trans := observer.transitions[1]
		if trans.newState != core.ToolStateAwaitingAuth {
			t.Errorf("Expected transition to AwaitingAuth, got %v", trans.newState)
		}
	}
}

func TestToolStateMachine_ErrorDuringExecution(t *testing.T) {
	// Test: Tool execution fails
	// Expected: Idle → Executing → Error

	ctx := context.Background()
	toolCall := &core.ToolCall{
		ID:        "tc_error",
		Type:      "function",
		Name:      "failing_tool",
		Arguments: `{}`,
	}

	observer := &MockToolStateObserver{}

	tsm := core.NewToolStateMachine(toolCall)
	tsm.AddObserver(observer)

	// Start execution
	err := tsm.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify state
	if state := tsm.CurrentState(ctx); state != core.ToolStateError {
		t.Errorf("Expected state Error, got %v", state)
	}

	// Verify result shows error
	result := tsm.Result()
	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if !result.IsError {
		t.Error("Expected IsError to be true")
	}

	// Verify it's terminal
	if !tsm.IsTerminal(ctx) {
		t.Error("Expected tool to be in terminal state")
	}

	// Verify state transitions: Idle -> Executing -> Error
	if len(observer.transitions) != 2 {
		t.Errorf("Expected 2 transitions, got %d", len(observer.transitions))
	}
}

func TestToolStateMachine_IsTerminal(t *testing.T) {
	ctx := context.Background()
	toolCall := &core.ToolCall{
		ID:        "tc_term",
		Type:      "function",
		Name:      "test_tool",
		Arguments: `{}`,
	}

	tsm := core.NewToolStateMachine(toolCall)
	tsm.Start(ctx)

	if !tsm.IsTerminal(ctx) {
		t.Error("Expected Completed state to be terminal")
	}

	// Test awaiting state is not terminal
	tsm2 := core.NewToolStateMachine(toolCall)
	tsm2.Start(ctx)

	if tsm2.IsTerminal(ctx) {
		t.Error("Expected AwaitingInput state to not be terminal")
	}
}
