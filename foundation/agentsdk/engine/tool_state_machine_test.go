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

func (m *MockToolExecutor) Context() context.Context {
	return context.Background()
}

// MockToolStateObserver records state transitions for testing.
type MockToolStateObserver struct {
	transitions []struct {
		toolCallID  string
		toolName    string
		arguments   string
		oldState    core.ToolState
		newState    core.ToolState
		result      *core.ToolExecutionResult
		elicitation *core.ElicitationSpec
	}
}

func (m *MockToolStateObserver) OnToolStateChanged(
	toolCallID string,
	toolName string,
	arguments string,
	oldState core.ToolState,
	newState core.ToolState,
	result *core.ToolExecutionResult,
	elicitation *core.ElicitationSpec,
) {
	m.transitions = append(m.transitions, struct {
		toolCallID  string
		toolName    string
		arguments   string
		oldState    core.ToolState
		newState    core.ToolState
		result      *core.ToolExecutionResult
		elicitation *core.ElicitationSpec
	}{
		toolCallID:  toolCallID,
		toolName:    toolName,
		arguments:   arguments,
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

	// Start execution (transitions to Executing)
	err := tsm.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Verify executing state
	if state := tsm.CurrentState(ctx); state != core.ToolStateExecuting {
		t.Errorf("Expected state Executing after Start, got %v", state)
	}

	// Signal success (transitions to Completed)
	result := &core.ToolExecutionResult{
		ToolCallID: "tc_123",
		ToolName:   "calculator",
		Result:     "3",
		IsError:    false,
	}
	err = tsm.Success(result)
	if err != nil {
		t.Fatalf("Success() failed: %v", err)
	}

	// Verify final state
	if state := tsm.CurrentState(ctx); state != core.ToolStateCompleted {
		t.Errorf("Expected state Completed, got %v", state)
	}

	// Verify result
	storedResult := tsm.Result()
	if storedResult == nil {
		t.Fatal("Expected result, got nil")
	}
	if storedResult.Result != "3" {
		t.Errorf("Expected result '3', got '%s'", storedResult.Result)
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

	// Start execution (transitions to Executing)
	err := tsm.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Signal needs input (transitions to AwaitingInput)
	result := &core.ToolExecutionResult{
		ToolCallID: "tc_456",
		ToolName:   "elicitation_tool",
		Result:     `{"status": "awaiting input"}`,
		IsError:    false,
	}
	err = tsm.NeedsInput(elicitation, result)
	if err != nil {
		t.Fatalf("NeedsInput() failed: %v", err)
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

	authSpec := &core.AuthorizationSpec{
		Resource: "/etc/passwd",
		Action:   "read",
		Reason:   "Tool requires access to system file",
	}

	observer := &MockToolStateObserver{}

	tsm := core.NewToolStateMachine(toolCall)
	tsm.AddObserver(observer)

	// Start execution (transitions to Executing)
	err := tsm.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Signal needs auth (transitions to AwaitingAuth)
	result := &core.ToolExecutionResult{
		ToolCallID: "tc_789",
		ToolName:   "file_access",
		Result:     `{"status": "awaiting authorization"}`,
		IsError:    false,
	}
	err = tsm.NeedsAuth(authSpec, result)
	if err != nil {
		t.Fatalf("NeedsAuth() failed: %v", err)
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

	// Start execution (transitions to Executing)
	err := tsm.Start(ctx)
	if err != nil {
		t.Fatalf("Start() failed: %v", err)
	}

	// Signal error (transitions to Error)
	errorResult := &core.ToolExecutionResult{
		ToolCallID: "tc_error",
		ToolName:   "failing_tool",
		Result:     "Tool execution failed",
		IsError:    true,
	}
	err = tsm.Error(errorResult)
	if err != nil {
		t.Fatalf("Error() failed: %v", err)
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

	// Initially in Idle - not terminal
	if tsm.IsTerminal(ctx) {
		t.Error("Expected Idle state to not be terminal")
	}

	// Start execution - now in Executing, not terminal
	tsm.Start(ctx)
	if tsm.IsTerminal(ctx) {
		t.Error("Expected Executing state to not be terminal")
	}

	// Complete execution - now in Completed, which is terminal
	result := &core.ToolExecutionResult{
		ToolCallID: "tc_term",
		ToolName:   "test_tool",
		Result:     "success",
	}
	tsm.Success(result)

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
