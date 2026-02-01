package domain

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"school-with-ai/foundation/agentsdk/core"
)

// MockToolExecutor implements core.ToolExecutor for testing
type MockToolExecutor struct {
	executeFunc func(tc core.ToolCall) (core.ToolExecutionResult, error)
}

func (m *MockToolExecutor) Execute(tc core.ToolCall) (core.ToolExecutionResult, error) {
	if m.executeFunc != nil {
		return m.executeFunc(tc)
	}
	return core.ToolExecutionResult{}, nil
}

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
	elicitation *core.ElicitationSpec,
) {
	m.transitions = append(m.transitions, oldState.String()+"->"+newState.String())
}

func TestToolStateMachine_DirectSuccess(t *testing.T) {
	ctx := context.Background()
	tc := &core.ToolCall{
		ID:   "tc-1",
		Name: "test_tool",
	}

	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{
				Result: "success",
			}, nil
		},
	}

	tsm := NewToolStateMachine(ctx, tc, executor)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	if tsm.CurrentState() != ToolStateIdle {
		t.Errorf("Expected initial state Idle, got %s", tsm.CurrentState())
	}

	err := tsm.Start()
	if err != nil {
		t.Fatalf("Failed to start TSM: %v", err)
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
	tc := &core.ToolCall{
		ID:   "tc-2",
		Name: "elicitation_tool",
	}

	var callCount int
	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			callCount++
			if callCount == 1 {
				return core.ToolExecutionResult{
					Command: &core.Command{
						Elicitation: &core.ElicitationSpec{
							Question: "What is your name?",
						},
						StopIteration: true,
					},
				}, nil
			}
			return core.ToolExecutionResult{
				Result: "Hello User",
			}, nil
		},
	}

	tsm := NewToolStateMachine(ctx, tc, executor)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	// Start - should pause for elicitation
	err := tsm.Start()
	if err != nil {
		t.Fatalf("Failed to start TSM: %v", err)
	}

	if tsm.CurrentState() != ToolStateAwaitingInput {
		t.Errorf("Expected state AwaitingInput, got %s", tsm.CurrentState())
	}

	if tsm.Elicitation().Question != "What is your name?" {
		t.Errorf("Expected elicitation question 'What is your name?', got '%s'", tsm.Elicitation().Question)
	}

	// Provide input - should resume and complete
	err = tsm.ProvideInput("Alice")
	if err != nil {
		t.Fatalf("Failed to provide input: %v", err)
	}

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

	if tsm.Result().Result != "Hello User" {
		t.Errorf("Expected result 'Hello User', got '%s'", tsm.Result().Result)
	}
}

func TestToolStateMachine_AuthorizationFlow(t *testing.T) {
	ctx := context.Background()
	tc := &core.ToolCall{
		ID:   "tc-3",
		Name: "auth_tool",
	}

	var callCount int
	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			callCount++
			if callCount == 1 {
				return core.ToolExecutionResult{
					Command: &core.Command{
						Authorization: &core.AuthorizationSpec{
							Resource: "secret",
							Action:   "read",
						},
						StopIteration: true,
					},
				}, nil
			}
			return core.ToolExecutionResult{
				Result: "secret data",
			}, nil
		},
	}

	tsm := NewToolStateMachine(ctx, tc, executor)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	// Start - should pause for auth
	err := tsm.Start()
	if err != nil {
		t.Fatalf("Failed to start TSM: %v", err)
	}

	if tsm.CurrentState() != ToolStateAwaitingAuth {
		t.Errorf("Expected state AwaitingAuth, got %s", tsm.CurrentState())
	}

	// Grant auth - should resume and complete
	err = tsm.GrantAuth()
	if err != nil {
		t.Fatalf("Failed to grant auth: %v", err)
	}

	if tsm.CurrentState() != ToolStateCompleted {
		t.Errorf("Expected final state Completed, got %s", tsm.CurrentState())
	}

	expectedTransitions := []string{
		"idle->executing",
		"executing->awaiting_auth",
		"awaiting_auth->executing",
		"executing->completed",
	}
	if len(observer.transitions) != len(expectedTransitions) {
		t.Errorf("Expected %d transitions, got %d", len(expectedTransitions), len(observer.transitions))
	}
}

func TestToolStateMachine_ErrorPath(t *testing.T) {
	ctx := context.Background()
	tc := &core.ToolCall{
		ID:   "tc-4",
		Name: "error_tool",
	}

	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{}, errors.New("boom")
		},
	}

	tsm := NewToolStateMachine(ctx, tc, executor)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	err := tsm.Start()
	// TSM.Start() returns nil if Fire succeeds, even if entering Error state?
	// Actually t.sm.Fire(TrigToolError) is called inside executeToolLogic.
	// stateless.Fire returns error if trigger is not allowed.
	// In our case it should return nil but state becomes ToolStateError.
	if err != nil {
		t.Logf("Start returned error (as expected if Fire fails?): %v", err)
	}

	if tsm.CurrentState() != ToolStateError {
		t.Errorf("Expected state Error, got %s", tsm.CurrentState())
	}

	if tsm.Result().Result != "boom" {
		t.Errorf("Expected error message 'boom', got '%s'", tsm.Result().Result)
	}
	if !tsm.Result().IsError {
		t.Error("Expected IsError to be true")
	}
}

func TestToolStateMachine_Cancellation(t *testing.T) {
	ctx := context.Background()
	tc := &core.ToolCall{
		ID:   "tc-5",
		Name: "cancel_tool",
	}

	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{
				Command: &core.Command{
					Elicitation: &core.ElicitationSpec{Question: "test"},
				},
			}, nil
		},
	}

	tsm := NewToolStateMachine(ctx, tc, executor)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	_ = tsm.Start() // Enters AwaitingInput

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
	// Verify concurrent access to state machine methods is safe
	ctx := context.Background()
	tc := &core.ToolCall{
		ID:   "tc-concurrent",
		Name: "concurrent_tool",
	}

	// Use a slow executor to increase chance of race conditions
	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			time.Sleep(10 * time.Millisecond) // Simulate work
			return core.ToolExecutionResult{
				Result: "concurrent success",
			}, nil
		},
	}

	tsm := NewToolStateMachine(ctx, tc, executor)

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

	// Start execution in one goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = tsm.Start()
	}()

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

	wg.Wait()

	// Verify final state is consistent
	if tsm.CurrentState() != ToolStateCompleted {
		t.Errorf("Expected final state Completed, got %s", tsm.CurrentState())
	}
}

func TestToolStateMachine_ConcurrentObserverRegistration(t *testing.T) {
	// Verify observer registration is safe during state transitions
	ctx := context.Background()
	tc := &core.ToolCall{
		ID:   "tc-observer-race",
		Name: "observer_race_tool",
	}

	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			return core.ToolExecutionResult{Result: "done"}, nil
		},
	}

	tsm := NewToolStateMachine(ctx, tc, executor)

	var wg sync.WaitGroup
	const numObservers = 20

	// Register half the observers before starting
	for i := 0; i < numObservers/2; i++ {
		tsm.RegisterObserver(&MockObserver{})
	}

	// Start execution
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = tsm.Start()
	}()

	// Register remaining observers concurrently with execution
	wg.Add(numObservers / 2)
	for i := 0; i < numObservers/2; i++ {
		go func() {
			defer wg.Done()
			observer := &MockObserver{}
			tsm.RegisterObserver(observer)
		}()
	}

	wg.Wait()

	if tsm.CurrentState() != ToolStateCompleted {
		t.Errorf("Expected final state Completed, got %s", tsm.CurrentState())
	}
}

func TestToolStateMachine_ConcurrentElicitation(t *testing.T) {
	// Verify concurrent access during elicitation is safe
	ctx := context.Background()
	tc := &core.ToolCall{
		ID:   "tc-elicit-concurrent",
		Name: "elicit_concurrent_tool",
	}

	var callCount int
	var mu sync.Mutex
	executor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			mu.Lock()
			callCount++
			count := callCount
			mu.Unlock()

			if count == 1 {
				time.Sleep(5 * time.Millisecond)
				return core.ToolExecutionResult{
					Command: &core.Command{
						Elicitation: &core.ElicitationSpec{
							Question: "What?",
						},
						StopIteration: true,
					},
				}, nil
			}
			time.Sleep(5 * time.Millisecond)
			return core.ToolExecutionResult{Result: "final"}, nil
		},
	}

	tsm := NewToolStateMachine(ctx, tc, executor)
	observer := &MockObserver{}
	tsm.RegisterObserver(observer)

	var wg sync.WaitGroup

	// Start execution
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = tsm.Start()
	}()

	// Concurrently read state while transitioning to AwaitingInput
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				_ = tsm.CurrentState()
				_ = tsm.Elicitation()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// Wait for AwaitingInput state
	time.Sleep(20 * time.Millisecond)

	// Provide input while still reading state
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = tsm.ProvideInput("answer")
	}()

	wg.Wait()

	if tsm.CurrentState() != ToolStateCompleted {
		t.Errorf("Expected final state Completed, got %s", tsm.CurrentState())
	}
}
