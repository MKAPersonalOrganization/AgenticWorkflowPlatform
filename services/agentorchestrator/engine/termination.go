package engine

import (
	"fmt"
	"time"
)

// TerminationReason describes why an agent execution ended.
type TerminationReason string

const (
	// TerminationGoalReached indicates the agent completed its task successfully.
	TerminationGoalReached TerminationReason = "goal_reached"

	// TerminationMaxIterations indicates the agent hit its iteration limit.
	TerminationMaxIterations TerminationReason = "max_iterations"

	// TerminationTimeout indicates the agent exceeded its time limit.
	TerminationTimeout TerminationReason = "timeout"

	// TerminationUserAborted indicates the user cancelled the execution.
	TerminationUserAborted TerminationReason = "user_aborted"

	// TerminationLoopDetected indicates a loop pattern was detected.
	TerminationLoopDetected TerminationReason = "loop_detected"

	// TerminationAwaitingInput indicates the agent is waiting for user input.
	TerminationAwaitingInput TerminationReason = "awaiting_input"

	// TerminationError indicates an error occurred during execution.
	TerminationError TerminationReason = "error"
)

// LoopType describes the type of loop detected.
type LoopType string

const (
	// LoopTypeToolCall indicates repeated identical tool calls.
	LoopTypeToolCall LoopType = "tool_call_loop"

	// LoopTypeToolError indicates repeated tool failures (same tool failing repeatedly).
	LoopTypeToolError LoopType = "tool_error_loop"

	// LoopTypeContent indicates repeated content patterns.
	LoopTypeContent LoopType = "content_repetition"

	// LoopTypePedagogical indicates unproductive teaching pattern.
	LoopTypePedagogical LoopType = "pedagogical_loop"
)

// ExecutionResult contains the outcome of an agent execution run.
type ExecutionResult struct {
	// Success indicates whether the execution completed without errors.
	Success bool

	// Reason explains why the execution terminated.
	Reason TerminationReason

	// FinalAgent is the name of the last agent that was executing.
	FinalAgent string

	// Iterations is the total number of LLM iterations across all agents.
	Iterations int

	// ToolsCalled is a list of all tools invoked during execution.
	ToolsCalled []string

	// Duration is how long the execution took.
	Duration time.Duration

	// LoopInfo contains details if a loop was detected.
	LoopInfo *LoopInfo

	// Error contains the error if Reason is TerminationError.
	Error error
}

// LoopInfo contains details about a detected loop.
type LoopInfo struct {
	Type        LoopType
	Description string
	Occurrences int
	Pattern     string // The repeated pattern (tool signature or content chunk)
}

// String returns a human-readable description of the execution result.
func (r *ExecutionResult) String() string {
	status := "failed"
	if r.Success {
		status = "succeeded"
	}

	msg := fmt.Sprintf("Execution %s: reason=%s, agent=%s, iterations=%d, duration=%s",
		status, r.Reason, r.FinalAgent, r.Iterations, r.Duration)

	if r.LoopInfo != nil {
		msg += fmt.Sprintf(", loop_type=%s, occurrences=%d", r.LoopInfo.Type, r.LoopInfo.Occurrences)
	}

	if r.Error != nil {
		msg += fmt.Sprintf(", error=%v", r.Error)
	}

	return msg
}

// IsRecoverable returns true if the termination reason allows resumption.
func (r *ExecutionResult) IsRecoverable() bool {
	switch r.Reason {
	case TerminationAwaitingInput:
		return true
	default:
		return false
	}
}

// runCompletedEvent is an internal event emitted at the end of Run()
// to communicate the termination reason to ProcessUserMessage.
// This event is NOT forwarded to the frontend.
type runCompletedEvent struct {
	Reason TerminationReason
}

func (e runCompletedEvent) EventType() string         { return "RunCompleted" }
func (e runCompletedEvent) EventTimestamp() time.Time { return time.Now() }
func (e runCompletedEvent) AggregateID() string       { return "" }
