package engine

import "fmt"

// state represents the current state of the engine's execution.
type state int

const (
	// StateAwaitingExecution represents the initial state, ready to begin processing.
	// Replaces: StateAwaitingTurn
	StateAwaitingExecution state = iota

	// StateInvokingLLM represents the state where the LLM is generating a response.
	// Replaces: StateGeneratingResponse
	StateInvokingLLM

	// StateExecutingTools represents the state where tools are being executed.
	// Replaces: StatePerformingActions
	StateExecutingTools

	// StateAwaitingUserInput represents a paused state waiting for user interaction.
	// Unifies: StateAwaitingInput + StateAwaitingAuth
	StateAwaitingUserInput

	// StateCompleted represents a final state (success or error).
	// Unifies: StateDone + StateError
	StateCompleted
)

// String provides human-readable state names for debugging and persistence.
func (s state) String() string {
	switch s {
	case StateAwaitingExecution:
		return "AwaitingExecution"
	case StateInvokingLLM:
		return "InvokingLLM"
	case StateExecutingTools:
		return "ExecutingTools"
	case StateAwaitingUserInput:
		return "AwaitingUserInput"
	case StateCompleted:
		return "Completed"
	default:
		return fmt.Sprintf("Unknown(%d)", s)
	}
}

// PauseReason describes why execution is paused.
type PauseReason int

const (
	// PauseReasonNone indicates no pause (default value).
	PauseReasonNone PauseReason = iota

	// PauseReasonInput indicates paused for user input (elicitation).
	// Replaces: StateAwaitingInput
	PauseReasonInput

	// PauseReasonAuth indicates paused for authorization.
	// Replaces: StateAwaitingAuth
	PauseReasonAuth
)

// String provides human-readable pause reason names.
func (p PauseReason) String() string {
	switch p {
	case PauseReasonNone:
		return "None"
	case PauseReasonInput:
		return "Input"
	case PauseReasonAuth:
		return "Auth"
	default:
		return fmt.Sprintf("Unknown(%d)", p)
	}
}

// CompletionReason describes why execution reached the completed state.
// This provides context about the outcome without creating separate terminal states.
type CompletionReason int

const (
	// CompletionReasonNone indicates execution hasn't completed yet (default value).
	CompletionReasonNone CompletionReason = iota

	// CompletionReasonSuccess indicates successful goal completion.
	CompletionReasonSuccess

	// CompletionReasonError indicates a general execution error occurred.
	CompletionReasonError

	// CompletionReasonMaxIterations indicates iteration limit was reached.
	CompletionReasonMaxIterations

	// CompletionReasonLoopDetected indicates an infinite loop was detected.
	CompletionReasonLoopDetected

	// CompletionReasonTimeout indicates execution exceeded time limit.
	CompletionReasonTimeout

	// CompletionReasonUserAborted indicates user cancelled execution.
	CompletionReasonUserAborted
)

// String provides human-readable completion reason names.
func (c CompletionReason) String() string {
	switch c {
	case CompletionReasonNone:
		return "None"
	case CompletionReasonSuccess:
		return "Success"
	case CompletionReasonError:
		return "Error"
	case CompletionReasonMaxIterations:
		return "MaxIterations"
	case CompletionReasonLoopDetected:
		return "LoopDetected"
	case CompletionReasonTimeout:
		return "Timeout"
	case CompletionReasonUserAborted:
		return "UserAborted"
	default:
		return fmt.Sprintf("Unknown(%d)", c)
	}
}

// ToTerminationReason converts CompletionReason to TerminationReason for compatibility
// with the existing ExecutionResult API.
func (c CompletionReason) ToTerminationReason() TerminationReason {
	switch c {
	case CompletionReasonSuccess:
		return TerminationGoalReached
	case CompletionReasonError:
		return TerminationError
	case CompletionReasonMaxIterations:
		return TerminationMaxIterations
	case CompletionReasonLoopDetected:
		return TerminationLoopDetected
	case CompletionReasonTimeout:
		return TerminationTimeout
	case CompletionReasonUserAborted:
		return TerminationUserAborted
	default:
		return TerminationError
	}
}

// IsSuccess returns true if the completion was successful.
func (c CompletionReason) IsSuccess() bool {
	return c == CompletionReasonSuccess
}
