package domain

import (
	"context"
	"sync"
	"testing"
	"time"
)

// MockObserver tracks state changes
type MockObserver struct {
	transitions []string
}

func (m *MockObserver) OnToolStateChanged(
	toolCallID string,
	toolName string,
	oldState ToolState,
	newState ToolState,
	result *ToolExecutionResult,
	elicitation *ElicitationSpec,
) {
	m.transitions = append(m.transitions, oldState.String()+"->"+newState.String())
}

func TestToolStateMachine_DirectSuccess(t *testing.T) {
	ctx := context.Background()
	tc := &ToolCall{
		ID:   "tc-1",
		Name: "test_tool",
	}

	tsm := NewToolStateMachine(ctx, tc)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	if tsm.CurrentState() != ToolStateIdle {
		t.Errorf("Expected initial state Idle, got %s", tsm.CurrentState())
	}

	// Start
	err := tsm.Start()
	if err != nil {
		t.Fatalf("Failed to start TSM: %v", err)
	}

	// Success
	err = tsm.FireSuccess(&ToolExecutionResult{
		ToolCallID: "tc-1",
		Result:     "success",
	})
	if err != nil {
		t.Fatalf("Failed to fire success: %v", err)
	}

	if tsm.CurrentState() != ToolStateCompleted {
		t.Errorf("Expected final state Completed, got %s", tsm.CurrentState())
	}

	expectedTransitions := []string{"idle->executing", "executing->completed"}
	if len(observer.transitions) != len(expectedTransitions) {
		t.Errorf("Expected %d transitions, got %d", len(expectedTransitions), len(observer.transitions))
	}
	for i, tr := range observer.transitions {
		if tr != expectedTransitions[i] {
			t.Errorf("Transition %d: expected %s, got %s", i, expectedTransitions[i], tr)
		}
	}

	if tsm.Result().Result != "success" {
		t.Errorf("Expected result 'success', got '%s'", tsm.Result().Result)
	}
}

func TestToolStateMachine_ElicitationFlow(t *testing.T) {
	ctx := context.Background()
	tc := &ToolCall{
		ID:   "tc-2",
		Name: "elicitation_tool",
	}

	tsm := NewToolStateMachine(ctx, tc)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	// Start
	_ = tsm.Start()

	// Needs Input
	_ = tsm.FireNeedsInput(&ElicitationSpec{
		Question: "What is your name?",
	}, &ToolExecutionResult{ToolCallID: "tc-2"})

	if tsm.CurrentState() != ToolStateAwaitingInput {
		t.Errorf("Expected state AwaitingInput, got %s", tsm.CurrentState())
	}

	if tsm.Elicitation().Question != "What is your name?" {
		t.Errorf("Expected elicitation question 'What is your name?', got '%s'", tsm.Elicitation().Question)
	}

	// Provide input - should resume
	_ = tsm.ProvideInput("Alice")

	if tsm.CurrentState() != ToolStateExecuting {
		t.Errorf("Expected state Executing, got %s", tsm.CurrentState())
	}

	// Complete
	_ = tsm.FireSuccess(&ToolExecutionResult{ToolCallID: "tc-2", Result: "Hello Alice"})

	if tsm.CurrentState() != ToolStateCompleted {
		t.Errorf("Expected final state Completed, got %s", tsm.CurrentState())
	}

	expectedTransitions := []string{
		"idle->executing",
		"executing->awaiting_input",
		"awaiting_input->executing",
		"executing->completed",
	}
	if len(observer.transitions) != len(expectedTransitions) {
		t.Errorf("Expected %d transitions, got %d", len(expectedTransitions), len(observer.transitions))
	}
}

func TestToolStateMachine_AuthorizationFlow(t *testing.T) {
	ctx := context.Background()
	tc := &ToolCall{
		ID:   "tc-3",
		Name: "auth_tool",
	}

	tsm := NewToolStateMachine(ctx, tc)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	// Start
	_ = tsm.Start()

	// Needs Auth
	_ = tsm.FireNeedsAuth(&AuthorizationSpec{
		Resource: "secret",
		Action:   "read",
	}, &ToolExecutionResult{ToolCallID: "tc-3"})

	if tsm.CurrentState() != ToolStateAwaitingAuth {
		t.Errorf("Expected state AwaitingAuth, got %s", tsm.CurrentState())
	}

	// Grant auth - should resume
	_ = tsm.GrantAuth()

	if tsm.CurrentState() != ToolStateExecuting {
		t.Errorf("Expected state Executing, got %s", tsm.CurrentState())
	}

	// Complete
	_ = tsm.FireSuccess(&ToolExecutionResult{ToolCallID: "tc-3", Result: "secret data"})

	if tsm.CurrentState() != ToolStateCompleted {
		t.Errorf("Expected final state Completed, got %s", tsm.CurrentState())
	}
}

func TestToolStateMachine_Cancellation(t *testing.T) {
	ctx := context.Background()
	tc := &ToolCall{
		ID:   "tc-5",
		Name: "cancel_tool",
	}

	tsm := NewToolStateMachine(ctx, tc)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	_ = tsm.Start()
	_ = tsm.FireNeedsInput(&ElicitationSpec{Question: "test"}, &ToolExecutionResult{ToolCallID: "tc-5"})

	if tsm.CurrentState() != ToolStateAwaitingInput {
		t.Errorf("Expected state AwaitingInput, got %s", tsm.CurrentState())
	}

	err := tsm.Cancel()
	if err != nil {
		t.Fatalf("Failed to cancel: %v", err)
	}

	if tsm.CurrentState() != ToolStateError {
		t.Errorf("Expected state Error after cancellation, got %s", tsm.CurrentState())
	}

	expectedTransitions := []string{
		"idle->executing",
		"executing->awaiting_input",
		"awaiting_input->error",
	}
	if len(observer.transitions) != len(expectedTransitions) {
		t.Errorf("Expected %d transitions, got %d", len(expectedTransitions), len(observer.transitions))
	}
}

func TestToolStateMachine_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	tc := &ToolCall{
		ID:   "tc-concurrent",
		Name: "concurrent_tool",
	}

	tsm := NewToolStateMachine(ctx, tc)

	var wg sync.WaitGroup
	const numGoroutines = 10

	// Concurrently register observers
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			observer := &MockObserver{}
			tsm.RegisterObserver(observer)
		}()
	}
	wg.Wait()

	// Start execution
	_ = tsm.Start()

	// Concurrently read state, result, and elicitation
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = tsm.CurrentState()
				_ = tsm.Result()
				_ = tsm.Elicitation()
				_ = tsm.IsTerminal()
				_ = tsm.IsAwaitingInteraction()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Transition to success concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = tsm.FireSuccess(&ToolExecutionResult{ToolCallID: "tc-concurrent", Result: "done"})
	}()

	wg.Wait()

	if tsm.CurrentState() != ToolStateCompleted {
		t.Errorf("Expected final state Completed, got %s", tsm.CurrentState())
	}
}
