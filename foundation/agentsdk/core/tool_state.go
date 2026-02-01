package core

import (
	"context"
	"sync"

	"github.com/qmuntal/stateless"
)

// ToolState represents the state of a tool execution.
type ToolState int

const (
	ToolStateIdle          ToolState = iota // Initial state (not started)
	ToolStateExecuting                      // Tool is running
	ToolStateAwaitingInput                  // Waiting for human input (elicitation)
	ToolStateAwaitingAuth                   // Waiting for user authorization
	ToolStateCompleted                      // Successfully completed with result
	ToolStateError                          // Execution failed
)

// String provides human-readable state names for debugging and JSON serialization.
func (s ToolState) String() string {
	switch s {
	case ToolStateIdle:
		return "idle"
	case ToolStateExecuting:
		return "executing"
	case ToolStateAwaitingInput:
		return "awaiting_input"
	case ToolStateAwaitingAuth:
		return "awaiting_auth"
	case ToolStateCompleted:
		return "completed"
	case ToolStateError:
		return "error"
	default:
		return "unknown"
	}
}

// Tool state machine triggers
const (
	TrigToolStart      = "ToolStart"
	TrigToolSuccess    = "ToolSuccess"
	TrigToolError      = "ToolError"
	TrigToolNeedsInput = "ToolNeedsInput"
	TrigToolNeedsAuth  = "ToolNeedsAuth"
	TrigInputProvided  = "InputProvided"
	TrigAuthGranted    = "AuthGranted"
	TrigAuthDenied     = "AuthDenied"
	TrigToolCancelled  = "ToolCancelled"
)

// ToolExecutionResult represents the result of executing a tool
type ToolExecutionResult struct {
	ToolCallID    string   `json:"tool_call_id"`
	ToolName      string   `json:"tool_name"`
	Arguments     string   `json:"arguments"`
	Result        string   `json:"result"`
	IsError       bool     `json:"is_error"`
	StopIteration bool     `json:"stop_iteration"`
	Command       *Command `json:"command,omitempty"`
}

// ToolExecutionResults represents the outcome of executing multiple tool calls.
type ToolExecutionResults struct {
	Results    []ToolExecutionResult
	ShouldStop bool // true if any tool signaled to stop iteration
}

// ToolStateObserver is notified when a tool's state changes.
type ToolStateObserver interface {
	OnToolStateChanged(
		toolCallID string,
		toolName string,
		oldState ToolState,
		newState ToolState,
		result *ToolExecutionResult,
		elicitation *ElicitationSpec,
	)
}

// ToolStateMachine manages the lifecycle of a single tool execution.
// It is safe for concurrent use by multiple goroutines.
// The stateless.StateMachine library is internally thread-safe, so we only
// need to protect access to our own fields (result, elicitation, observers).
type ToolStateMachine struct {
	mu          sync.Mutex // Protects result, elicitation, and observers fields only
	sm          *stateless.StateMachine
	toolCall    *ToolCall
	result      *ToolExecutionResult
	elicitation *ElicitationSpec
	authSpec    *AuthorizationSpec
	observers   []ToolStateObserver
}

// NewToolStateMachine creates a new tool state machine for managing tool execution lifecycle.
func NewToolStateMachine(toolCall *ToolCall) *ToolStateMachine {
	tsm := &ToolStateMachine{
		toolCall:  toolCall,
		observers: make([]ToolStateObserver, 0),
	}

	// Create a new state machine
	tsm.sm = stateless.NewStateMachine(ToolStateIdle)

	// Configure state transitions and guards
	tsm.sm.Configure(ToolStateIdle).Permit(TrigToolStart, ToolStateExecuting)

	tsm.sm.Configure(ToolStateExecuting).
		Permit(TrigToolSuccess, ToolStateCompleted).
		Permit(TrigToolError, ToolStateError).
		Permit(TrigToolNeedsInput, ToolStateAwaitingInput).
		Permit(TrigToolNeedsAuth, ToolStateAwaitingAuth).
		Permit(TrigToolCancelled, ToolStateError)

	tsm.sm.Configure(ToolStateAwaitingInput).
		Permit(TrigInputProvided, ToolStateExecuting).
		Permit(TrigToolError, ToolStateError).
		Permit(TrigToolCancelled, ToolStateError)

	tsm.sm.Configure(ToolStateAwaitingAuth).
		Permit(TrigAuthGranted, ToolStateExecuting).
		Permit(TrigAuthDenied, ToolStateError).
		Permit(TrigToolCancelled, ToolStateError)

	// Configure entry/exit actions with proper ActionFunc signature
	tsm.sm.Configure(ToolStateExecuting).OnEntry(func(ctx context.Context, args ...any) error { tsm.onToolStarted(); return nil })
	tsm.sm.Configure(ToolStateCompleted).OnEntry(func(ctx context.Context, args ...any) error { tsm.onToolCompleted(); return nil })
	tsm.sm.Configure(ToolStateError).OnEntry(func(ctx context.Context, args ...any) error { tsm.onToolError(); return nil })
	tsm.sm.Configure(ToolStateAwaitingInput).OnEntry(func(ctx context.Context, args ...any) error { tsm.onNeedsInput(); return nil })
	tsm.sm.Configure(ToolStateAwaitingAuth).OnEntry(func(ctx context.Context, args ...any) error { tsm.onNeedsAuth(); return nil })

	return tsm
}

// ToolCall returns the tool call being managed.
func (tsm *ToolStateMachine) ToolCall() *ToolCall {
	return tsm.toolCall
}

// CurrentState returns the current state of the tool execution.
func (tsm *ToolStateMachine) CurrentState(ctx context.Context) ToolState {
	state, _ := tsm.sm.State(ctx) // ignore error for now
	return state.(ToolState)
}

// AddObserver adds a state change observer.
func (tsm *ToolStateMachine) AddObserver(observer ToolStateObserver) {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	tsm.observers = append(tsm.observers, observer)
}

// Start triggers tool execution.
func (tsm *ToolStateMachine) Start(ctx context.Context) error {
	return tsm.sm.Fire(TrigToolStart)
}

// Success signals successful tool completion.
func (tsm *ToolStateMachine) Success(result *ToolExecutionResult) error {
	tsm.mu.Lock()
	tsm.result = result
	tsm.mu.Unlock()
	return tsm.sm.Fire(TrigToolSuccess)
}

// Error signals tool execution failure.
func (tsm *ToolStateMachine) Error(result *ToolExecutionResult) error {
	tsm.mu.Lock()
	tsm.result = result
	tsm.mu.Unlock()
	return tsm.sm.Fire(TrigToolError)
}

// NeedsInput signals that the tool requires user input.
func (tsm *ToolStateMachine) NeedsInput(elicitation *ElicitationSpec, result *ToolExecutionResult) error {
	tsm.mu.Lock()
	tsm.elicitation = elicitation
	tsm.result = result
	tsm.mu.Unlock()
	return tsm.sm.Fire(TrigToolNeedsInput)
}

// NeedsAuth signals that the tool requires user authorization.
func (tsm *ToolStateMachine) NeedsAuth(authSpec *AuthorizationSpec, result *ToolExecutionResult) error {
	tsm.mu.Lock()
	tsm.authSpec = authSpec
	tsm.result = result
	tsm.mu.Unlock()
	return tsm.sm.Fire(TrigToolNeedsAuth)
}

// ProvideInput provides user input to continue execution.
func (tsm *ToolStateMachine) ProvideInput(input string) error {
	// Input is handled by the tool execution, we just transition back to executing
	return tsm.sm.Fire(TrigInputProvided)
}

// GrantAuth grants authorization to continue execution.
func (tsm *ToolStateMachine) GrantAuth() error {
	return tsm.sm.Fire(TrigAuthGranted)
}

// DenyAuth denies authorization and stops execution.
func (tsm *ToolStateMachine) DenyAuth() error {
	return tsm.sm.Fire(TrigAuthDenied)
}

// Cancel cancels tool execution.
func (tsm *ToolStateMachine) Cancel() error {
	return tsm.sm.Fire(TrigToolCancelled)
}

// Result returns the execution result (if available).
func (tsm *ToolStateMachine) Result() *ToolExecutionResult {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	return tsm.result
}

// Elicitation returns the current elicitation spec (if waiting for input).
func (tsm *ToolStateMachine) Elicitation() *ElicitationSpec {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	return tsm.elicitation
}

// AuthSpec returns the current authorization spec (if waiting for auth).
func (tsm *ToolStateMachine) AuthSpec() *AuthorizationSpec {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	return tsm.authSpec
}

// State change handlers
func (tsm *ToolStateMachine) onToolStarted() {
	tsm.notifyObservers(ToolStateIdle, ToolStateExecuting)
}

func (tsm *ToolStateMachine) onToolCompleted() {
	// Assume previous state was executing for simplicity
	// In a real implementation, we'd track the previous state
	tsm.notifyObservers(ToolStateExecuting, ToolStateCompleted)
}

func (tsm *ToolStateMachine) onToolError() {
	// Determine previous state - could be executing, awaiting input, or awaiting auth
	tsm.notifyObservers(ToolStateExecuting, ToolStateError)
}

func (tsm *ToolStateMachine) onNeedsInput() {
	tsm.notifyObservers(ToolStateExecuting, ToolStateAwaitingInput)
}

func (tsm *ToolStateMachine) onNeedsAuth() {
	tsm.notifyObservers(ToolStateExecuting, ToolStateAwaitingAuth)
}

func (tsm *ToolStateMachine) notifyObservers(oldState, newState ToolState) {
	tsm.mu.Lock()
	observers := make([]ToolStateObserver, len(tsm.observers))
	copy(observers, tsm.observers)
	result := tsm.result
	elicitation := tsm.elicitation
	tsm.mu.Unlock()

	for _, observer := range observers {
		observer.OnToolStateChanged(
			tsm.toolCall.ID,
			tsm.toolCall.Name,
			oldState,
			newState,
			result,
			elicitation,
		)
	}
}

// IsAwaitingInteraction returns true if the tool is awaiting user interaction.
func (tsm *ToolStateMachine) IsAwaitingInteraction(ctx context.Context) bool {
	state, _ := tsm.sm.State(ctx)
	return state.(ToolState) == ToolStateAwaitingInput || state.(ToolState) == ToolStateAwaitingAuth
}

// IsAwaitingInput returns true if the tool is specifically awaiting elicitation input.
func (tsm *ToolStateMachine) IsAwaitingInput(ctx context.Context) bool {
	state, _ := tsm.sm.State(ctx)
	return state.(ToolState) == ToolStateAwaitingInput
}

// IsAwaitingAuth returns true if the tool is specifically awaiting authorization.
func (tsm *ToolStateMachine) IsAwaitingAuth(ctx context.Context) bool {
	state, _ := tsm.sm.State(ctx)
	return state.(ToolState) == ToolStateAwaitingAuth
}

// IsTerminal returns true if the tool execution has reached a terminal state.
func (tsm *ToolStateMachine) IsTerminal(ctx context.Context) bool {
	state, _ := tsm.sm.State(ctx)
	return state.(ToolState) == ToolStateCompleted || state.(ToolState) == ToolStateError
}
