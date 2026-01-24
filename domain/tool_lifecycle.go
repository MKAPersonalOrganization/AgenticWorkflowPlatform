package domain

import (
	"context"
	"sync"
	"time"

	"school-with-ai/foundation/agentsdk/core"

	"github.com/qmuntal/stateless"
)

// -- States and Results --

// ToolExecutionResult represents the outcome of executing a single tool call.
type ToolExecutionResult struct {
	ToolCallID    string
	ToolName      string
	Arguments     string
	Result        string
	IsError       bool // True if the tool execution resulted in an error
	StopIteration bool
	Command       *core.Command
}

// ToolExecutionResults represents the outcome of executing multiple tool calls.
type ToolExecutionResults struct {
	Results    []ToolExecutionResult
	ShouldStop bool // true if any tool signaled to stop iteration
}

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

// -- Observer Interface --

// ToolStateObserver is notified when a tool's state changes.
type ToolStateObserver interface {
	OnToolStateChanged(
		toolCallID string,
		toolName string,
		oldState ToolState,
		newState ToolState,
		result *ToolExecutionResult,
		elicitation *core.ElicitationSpec,
	)
}

// -- State Machine --

// ToolStateMachine manages the lifecycle of a single tool execution.
// It is safe for concurrent use by multiple goroutines.
// The stateless.StateMachine library is internally thread-safe, so we only
// need to protect access to our own fields (result, elicitation, observers).
type ToolStateMachine struct {
	mu          sync.Mutex // Protects result, elicitation, and observers fields only
	sm          *stateless.StateMachine
	toolCall    *core.ToolCall
	result      *ToolExecutionResult
	elicitation *core.ElicitationSpec
	executor    core.ToolExecutor
	observers   []ToolStateObserver
	ctx         context.Context
}

// NewToolStateMachine creates a new tool state machine for a given tool call.
func NewToolStateMachine(ctx context.Context, tc *core.ToolCall, executor core.ToolExecutor) *ToolStateMachine {
	if tc == nil {
		panic("toolCall cannot be nil")
	}
	if executor == nil {
		panic("executor cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	tsm := &ToolStateMachine{
		toolCall: tc,
		executor: executor,
		ctx:      ctx,
	}
	tsm.setup()
	return tsm
}

// setup configures the state machine with all states and transitions.
func (t *ToolStateMachine) setup() {
	t.sm = stateless.NewStateMachine(ToolStateIdle)

	// Configure ToolStateIdle
	t.sm.Configure(ToolStateIdle).
		Permit(TrigToolStart, ToolStateExecuting)

	// Configure ToolStateExecuting
	t.sm.Configure(ToolStateExecuting).
		OnEntryFrom(TrigToolStart, func(_ context.Context, _ ...any) error {
			t.notifyStateChange(ToolStateIdle, ToolStateExecuting)
			return t.executeToolLogic(context.Background())
		}).
		OnEntryFrom(TrigInputProvided, func(_ context.Context, _ ...any) error {
			t.notifyStateChange(ToolStateAwaitingInput, ToolStateExecuting)
			return t.executeToolLogic(context.Background())
		}).
		OnEntryFrom(TrigAuthGranted, func(_ context.Context, _ ...any) error {
			t.notifyStateChange(ToolStateAwaitingAuth, ToolStateExecuting)
			return t.executeToolLogic(context.Background())
		}).
		Permit(TrigToolSuccess, ToolStateCompleted).
		Permit(TrigToolError, ToolStateError).
		Permit(TrigToolNeedsInput, ToolStateAwaitingInput).
		Permit(TrigToolNeedsAuth, ToolStateAwaitingAuth)

	// Configure ToolStateAwaitingInput
	t.sm.Configure(ToolStateAwaitingInput).
		OnEntry(t.onEnterAwaitingInput).
		Permit(TrigInputProvided, ToolStateExecuting).
		Permit(TrigToolCancelled, ToolStateError)

	// Configure ToolStateAwaitingAuth
	t.sm.Configure(ToolStateAwaitingAuth).
		OnEntry(t.onEnterAwaitingAuth).
		Permit(TrigAuthGranted, ToolStateExecuting).
		Permit(TrigAuthDenied, ToolStateError).
		Permit(TrigToolCancelled, ToolStateError)

	// Terminal states
	t.sm.Configure(ToolStateCompleted).
		OnEntry(t.onEnterCompleted)

	t.sm.Configure(ToolStateError).
		OnEntry(t.onEnterError)
}

// executeToolLogic executes the actual tool and determines the next trigger.
func (t *ToolStateMachine) executeToolLogic(_ context.Context, _ ...any) error {
	// Execute the tool via ToolExecutor
	execResult, err := t.executor.Execute(*t.toolCall)

	// Build result
	result := &ToolExecutionResult{
		ToolCallID:    t.toolCall.ID,
		ToolName:      t.toolCall.Function.Name,
		Arguments:     t.toolCall.Function.Arguments,
		StopIteration: execResult.StopIteration,
		Command:       execResult.Command,
	}

	if err != nil {
		result.Result = err.Error()
		result.IsError = true
		t.mu.Lock()
		t.result = result
		t.mu.Unlock()
		return t.sm.Fire(TrigToolError)
	}

	result.Result = execResult.Result

	// Check if tool returned a Command with special instructions
	if execResult.Command != nil {
		// Check for elicitation request
		if execResult.Command.Elicitation != nil {
			t.mu.Lock()
			t.elicitation = execResult.Command.Elicitation
			t.result = result
			t.mu.Unlock()
			return t.sm.Fire(TrigToolNeedsInput)
		}

		// Check for authorization request
		if execResult.Command.Authorization != nil {
			t.mu.Lock()
			t.result = result
			t.mu.Unlock()
			return t.sm.Fire(TrigToolNeedsAuth)
		}
	}

	// Normal successful completion
	t.mu.Lock()
	t.result = result
	t.mu.Unlock()
	return t.sm.Fire(TrigToolSuccess)
}

// onEnterAwaitingInput is called when entering the AwaitingInput state.
func (t *ToolStateMachine) onEnterAwaitingInput(_ context.Context, _ ...any) error {
	t.notifyStateChange(ToolStateExecuting, ToolStateAwaitingInput)
	return nil
}

// onEnterAwaitingAuth is called when entering the AwaitingAuth state.
func (t *ToolStateMachine) onEnterAwaitingAuth(_ context.Context, _ ...any) error {
	t.notifyStateChange(ToolStateExecuting, ToolStateAwaitingAuth)
	return nil
}

// onEnterCompleted is called when entering the Completed state.
func (t *ToolStateMachine) onEnterCompleted(_ context.Context, _ ...any) error {
	oldState := ToolStateExecuting
	if t.elicitation != nil {
		oldState = ToolStateAwaitingInput
	}
	t.notifyStateChange(oldState, ToolStateCompleted)
	return nil
}

// onEnterError is called when entering the Error state.
func (t *ToolStateMachine) onEnterError(_ context.Context, _ ...any) error {
	currentState := t.sm.MustState().(ToolState)
	// Determine old state based on current context
	oldState := ToolStateExecuting
	switch currentState {
	case ToolStateAwaitingInput:
		oldState = ToolStateAwaitingInput
	case ToolStateAwaitingAuth:
		oldState = ToolStateAwaitingAuth
	}
	t.notifyStateChange(oldState, ToolStateError)
	return nil
}

// Start begins the tool execution by firing the start trigger.
func (t *ToolStateMachine) Start() error {
	return t.sm.Fire(TrigToolStart)
}

// ProvideInput provides user input to resume execution from AwaitingInput state.
func (t *ToolStateMachine) ProvideInput(input string) error {
	// Store input for resumption
	// This would be handled by the tool executor on next execution
	return t.sm.Fire(TrigInputProvided)
}

// GrantAuth grants authorization to resume execution from AwaitingAuth state.
func (t *ToolStateMachine) GrantAuth() error {
	return t.sm.Fire(TrigAuthGranted)
}

// DenyAuth denials authorization, causing the tool to error.
func (t *ToolStateMachine) DenyAuth() error {
	return t.sm.Fire(TrigAuthDenied)
}

// Cancel cancels the tool execution.
func (t *ToolStateMachine) Cancel() error {
	return t.sm.Fire(TrigToolCancelled)
}

// CurrentState returns the current state of the tool.
func (t *ToolStateMachine) CurrentState() ToolState {
	return t.sm.MustState().(ToolState)
}

// Result returns the execution result (may be nil if not completed).
func (t *ToolStateMachine) Result() *ToolExecutionResult {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.result
}

// Elicitation returns the elicitation spec if the tool is awaiting input.
func (t *ToolStateMachine) Elicitation() *core.ElicitationSpec {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.elicitation
}

// RegisterObserver registers a state change observer.
func (t *ToolStateMachine) RegisterObserver(observer ToolStateObserver) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.observers = append(t.observers, observer)
}

// notifyStateChange notifies all observers of a state transition.
// Calls observers without holding the lock to prevent deadlocks.
func (t *ToolStateMachine) notifyStateChange(oldState, newState ToolState) {
	t.mu.Lock()

	// Copy result if needed
	var result *ToolExecutionResult
	if (newState == ToolStateCompleted || newState == ToolStateError || newState == ToolStateAwaitingInput || newState == ToolStateAwaitingAuth) && t.result != nil {
		result = t.result
	}

	// Make copies of data we need
	observersCopy := make([]ToolStateObserver, len(t.observers))
	copy(observersCopy, t.observers)
	elicitation := t.elicitation

	t.mu.Unlock()

	// Call observers without holding lock to prevent deadlocks
	toolCallID := t.toolCall.ID
	toolName := t.toolCall.Function.Name
	for _, observer := range observersCopy {
		observer.OnToolStateChanged(toolCallID, toolName, oldState, newState, result, elicitation)
	}
}

// IsTerminal returns true if the tool is in a terminal state.
func (t *ToolStateMachine) IsTerminal() bool {
	state := t.sm.MustState().(ToolState)
	return state == ToolStateCompleted || state == ToolStateError
}

// IsAwaitingInteraction returns true if the tool is awaiting user interaction.
func (t *ToolStateMachine) IsAwaitingInteraction() bool {
	state := t.sm.MustState().(ToolState)
	return state == ToolStateAwaitingInput || state == ToolStateAwaitingAuth
}

// IsAwaitingInput returns true if the tool is specifically awaiting elicitation input.
func (t *ToolStateMachine) IsAwaitingInput() bool {
	return t.sm.MustState().(ToolState) == ToolStateAwaitingInput
}

// IsAwaitingAuth returns true if the tool is specifically awaiting authorization.
func (t *ToolStateMachine) IsAwaitingAuth() bool {
	return t.sm.MustState().(ToolState) == ToolStateAwaitingAuth
}

// ToolStateChangedEvent is emitted when a tool transitions between states.
type ToolStateChangedEvent struct {
	ThreadID      string    `json:"threadId"`
	MessageID     string    `json:"messageId"`
	ToolCallID    string    `json:"toolCallId"`
	ToolName      string    `json:"toolName"`
	OldState      string    `json:"oldState"`
	NewState      string    `json:"newState"`
	Result        string    `json:"result,omitempty"`
	IsError       bool      `json:"isError,omitempty"`
	ToolArguments string    `json:"arguments,omitempty"`
	AgentName     string    `json:"agentName"`
	Timestamp     time.Time `json:"timestamp"`
}

func (e ToolStateChangedEvent) EventType() string         { return "ToolStateChanged" }
func (e ToolStateChangedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolStateChangedEvent) AggregateID() string       { return e.ThreadID }

// ToolAwaitingInputEvent is emitted when a tool requests user input (elicitation).
type ToolAwaitingInputEvent struct {
	ThreadID    string                `json:"threadId"`
	MessageID   string                `json:"messageId"`
	ToolCallID  string                `json:"toolCallId"`
	ToolName    string                `json:"toolName"`
	Elicitation *core.ElicitationSpec `json:"elicitation"`
	AgentName   string                `json:"agentName"`
	Timestamp   time.Time             `json:"timestamp"`
}

func (e ToolAwaitingInputEvent) EventType() string         { return "ToolAwaitingInput" }
func (e ToolAwaitingInputEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolAwaitingInputEvent) AggregateID() string       { return e.ThreadID }

// ToolAwaitingAuthEvent is emitted when a tool requests user authorization.
type ToolAwaitingAuthEvent struct {
	ThreadID   string                  `json:"threadId"`
	MessageID  string                  `json:"messageId"`
	ToolCallID string                  `json:"toolCallId"`
	ToolName   string                  `json:"toolName"`
	AuthSpec   *core.AuthorizationSpec `json:"authSpec"`
	AgentName  string                  `json:"agentName"`
	Timestamp  time.Time               `json:"timestamp"`
}

func (e ToolAwaitingAuthEvent) EventType() string         { return "ToolAwaitingAuth" }
func (e ToolAwaitingAuthEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolAwaitingAuthEvent) AggregateID() string       { return e.ThreadID }

// ToolInputProvidedEvent is emitted when user provides input to a waiting tool.
type ToolInputProvidedEvent struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	Input      string    `json:"input"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e ToolInputProvidedEvent) EventType() string         { return "ToolInputProvided" }
func (e ToolInputProvidedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolInputProvidedEvent) AggregateID() string       { return e.ThreadID }

// ToolAuthGrantedEvent is emitted when user grants authorization to a waiting tool.
type ToolAuthGrantedEvent struct {
	ThreadID   string    `json:"threadId"`
	MessageID  string    `json:"messageId"`
	ToolCallID string    `json:"toolCallId"`
	AgentName  string    `json:"agentName"`
	Timestamp  time.Time `json:"timestamp"`
}

func (e ToolAuthGrantedEvent) EventType() string         { return "ToolAuthGranted" }
func (e ToolAuthGrantedEvent) EventTimestamp() time.Time { return e.Timestamp }
func (e ToolAuthGrantedEvent) AggregateID() string       { return e.ThreadID }

// GetToolStateMachineGraph returns the DOT graph representation of the tool state machine.
func GetToolStateMachineGraph() string {
	tsm := &ToolStateMachine{}
	tsm.setup()
	return tsm.sm.ToGraph()
}
