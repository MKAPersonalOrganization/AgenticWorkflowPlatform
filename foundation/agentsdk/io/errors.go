package io

import (
	"fmt"
)

// ---------------------------------------------------------------------------
// Sentinel errors for the pause-and-resume flow
// ---------------------------------------------------------------------------
//
// When a Handler cannot satisfy a request inline (e.g., in a web/SSE context),
// it returns one of these sentinel errors. The engine's tool processor detects
// them (via errors.As) and routes to the appropriate ToolStateMachine state:
//
//   ErrInputRequired → tsm.NeedsInput() → AwaitingInput → ToolNeedsInput event
//   ErrAuthRequired  → tsm.NeedsAuth()  → AwaitingAuth  → ToolNeedsAuth event
//
// This bridges BeeAI's blocking IO model to Go's pause-and-resume HTTP model.

// ErrInputRequired signals the engine to pause and request user input.
// The engine translates this to a ToolStateMachine.NeedsInput() transition,
// which emits a ToolNeedsInput event and pauses the execution.
//
// This is the Go equivalent of BeeAI's io_read() blocking call, adapted
// for the pause-and-resume pattern used in HTTP/SSE contexts.
type ErrInputRequired struct {
	// Question is the prompt shown to the user.
	Question string

	// Schema is an optional JSON Schema describing the expected input shape.
	// For example: {"type": "choose_multiple", "enum": ["A", "B", "C"]}
	Schema map[string]any
}

func (e *ErrInputRequired) Error() string {
	return fmt.Sprintf("io: input required: %s", e.Question)
}

// ErrAuthRequired signals the engine to pause and request user authorization.
// The engine translates this to a ToolStateMachine.NeedsAuth() transition,
// which emits a ToolNeedsAuth event and pauses the execution.
//
// This is the Go equivalent of BeeAI's AskPermissionRequirement calling
// io_confirm() which blocks, adapted for the pause-and-resume pattern.
type ErrAuthRequired struct {
	// Resource identifies what is being accessed (e.g., "tool:web_search").
	Resource string

	// Action describes what the tool wants to do (e.g., "execute").
	Action string

	// Reason is a human-readable explanation of why approval is needed.
	Reason string

	// Scopes are optional permission scopes required.
	Scopes []string
}

func (e *ErrAuthRequired) Error() string {
	return fmt.Sprintf("io: authorization required: %s (%s)", e.Resource, e.Reason)
}
