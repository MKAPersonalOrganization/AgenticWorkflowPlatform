package engine

import (
	"context"
	"errors"
	"testing"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// MockToolExecutor is a mock implementation of ToolExecutor for testing.
type MockToolExecutor struct {
	executeFunc func(toolCall core.ToolCall) (core.ToolExecutionResult, error)
}

func (m *MockToolExecutor) Execute(toolCall core.ToolCall) (core.ToolExecutionResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(toolCall)
	}
	return core.ToolExecutionResult{Result: "success"}, nil
}

// MockToolStateObserver records state transitions for testing.
type MockToolStateObserver struct {
	transitions []struct {
		toolCallID  string
		toolName    string
		oldState    domain.ToolState
		newState    domain.ToolState
		result      *domain.ToolExecutionResult
		elicitation *core.ElicitationSpec
	}
}

func (m *MockToolStateObserver) OnToolStateChanged(
	toolCallID string,
	toolName string,
	oldState domain.ToolState,
	newState domain.ToolState,
	result *domain.ToolExecutionResult,
	elicitation *core.ElicitationSpec,
) {
	m.transitions = append(m.transitions, struct {
		toolCallID  string
		toolName    string
		oldState    domain.ToolState
		newState    domain.ToolState
		result      *domain.ToolExecutionResult
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
		ID: "tc_123",
		Function: core.ToolFunction{
			Name:      "calculator",
			Arguments: `{"operation": "add", "a": 1, "b": 2}`,
		},
	}

	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{
				Result: "3",
			}, nil
		},
	}

	observer := &MockToolStateObserver{}

	tsm := domain.NewToolStateMachine(ctx, toolCall, executor)
	tsm.RegisterObserver(observer)

	// Start execution
	err := tsm.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify final state
	if state := tsm.CurrentState(); state != domain.ToolStateCompleted {
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
		if trans.newState != domain.ToolStateCompleted {
			t.Errorf("Expected transition to Completed, got %v", trans.newState)
		}
	}
}

func TestToolStateMachine_AwaitingInput(t *testing.T) {
	// Test: Tool requests elicitation
	// Expected: Idle → Executing → AwaitingInput

	ctx := context.Background()
	toolCall := &core.ToolCall{
		ID: "tc_456",
		Function: core.ToolFunction{
			Name:      "elicitation_tool",
			Arguments: `{}`,
		},
	}

	elicitation := &core.ElicitationSpec{
		Question: "What is your name?",
		Schema:   map[string]any{"type": "string"},
	}

	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{
				Result: "awaiting input",
				Command: &core.Command{
					StopIteration: true,
					Elicitation:   elicitation,
				},
				StopIteration: true,
			}, nil
		},
	}

	observer := &MockToolStateObserver{}

	tsm := domain.NewToolStateMachine(ctx, toolCall, executor)
	tsm.RegisterObserver(observer)

	// Start execution
	err := tsm.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify state
	if state := tsm.CurrentState(); state != domain.ToolStateAwaitingInput {
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
	if !tsm.IsAwaitingInteraction() {
		t.Error("Expected tool to be awaiting interaction")
	}

	// Verify state transitions: Idle -> Executing -> AwaitingInput
	if len(observer.transitions) != 2 {
		t.Errorf("Expected 2 transitions, got %d", len(observer.transitions))
	}
	if len(observer.transitions) >= 2 {
		trans := observer.transitions[1]
		if trans.newState != domain.ToolStateAwaitingInput {
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
		ID: "tc_789",
		Function: core.ToolFunction{
			Name:      "file_access",
			Arguments: `{"path": "/etc/passwd"}`,
		},
	}

	authSpec := &core.AuthorizationSpec{
		Resource: "/etc/passwd",
		Action:   "read",
		Reason:   "Need to check user configuration",
		Scopes:   []string{"file:read"},
	}

	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{
				Result: "awaiting authorization",
				Command: &core.Command{
					StopIteration: true,
					Authorization: authSpec,
				},
				StopIteration: true,
			}, nil
		},
	}

	observer := &MockToolStateObserver{}

	tsm := domain.NewToolStateMachine(ctx, toolCall, executor)
	tsm.RegisterObserver(observer)

	// Start execution
	err := tsm.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify state
	if state := tsm.CurrentState(); state != domain.ToolStateAwaitingAuth {
		t.Errorf("Expected state AwaitingAuth, got %v", state)
	}

	// Verify it's awaiting interaction
	if !tsm.IsAwaitingInteraction() {
		t.Error("Expected tool to be awaiting interaction")
	}

	// Verify state transitions: Idle -> Executing -> AwaitingAuth
	if len(observer.transitions) != 2 {
		t.Errorf("Expected 2 transitions, got %d", len(observer.transitions))
	}
	if len(observer.transitions) >= 2 {
		trans := observer.transitions[1]
		if trans.newState != domain.ToolStateAwaitingAuth {
			t.Errorf("Expected transition to AwaitingAuth, got %v", trans.newState)
		}
	}
}

func TestToolStateMachine_ErrorDuringExecution(t *testing.T) {
	// Test: Tool execution fails
	// Expected: Idle → Executing → Error

	ctx := context.Background()
	toolCall := &core.ToolCall{
		ID: "tc_error",
		Function: core.ToolFunction{
			Name:      "failing_tool",
			Arguments: `{}`,
		},
	}

	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{}, errors.New("tool execution failed")
		},
	}

	observer := &MockToolStateObserver{}

	tsm := domain.NewToolStateMachine(ctx, toolCall, executor)
	tsm.RegisterObserver(observer)

	// Start execution
	err := tsm.Start()
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify state
	if state := tsm.CurrentState(); state != domain.ToolStateError {
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
	if !tsm.IsTerminal() {
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
		ID: "tc_term",
		Function: core.ToolFunction{
			Name:      "test_tool",
			Arguments: `{}`,
		},
	}

	// Test completed state is terminal
	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{Result: "done"}, nil
		},
	}

	tsm := domain.NewToolStateMachine(ctx, toolCall, executor)
	tsm.Start()

	if !tsm.IsTerminal() {
		t.Error("Expected Completed state to be terminal")
	}

	// Test awaiting state is not terminal
	tsm2 := domain.NewToolStateMachine(ctx, toolCall, &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{
				Command: &core.Command{
					Elicitation: &core.ElicitationSpec{
						Question: "test?",
					},
					StopIteration: true,
				},
				StopIteration: true,
			}, nil
		},
	})
	tsm2.Start()

	if tsm2.IsTerminal() {
		t.Error("Expected AwaitingInput state to not be terminal")
	}
}
