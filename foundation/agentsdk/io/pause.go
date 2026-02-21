package io

import (
	"context"
)

// PauseHandler implements Handler for web/SSE contexts.
//
// Instead of blocking (which would tie up the HTTP goroutine), it returns
// sentinel errors that cause the engine to pause. The execution state is
// persisted and the SSE connection signals the frontend to show an input
// dialog. When the user responds, a new HTTP request resumes execution.
//
// This is the Go equivalent of BeeAI's ACPIOContext, adapted for Go's
// pause-and-resume HTTP architecture instead of Python's async/await.
//
// Flow:
//  1. Tool calls io.Confirm(ctx, "Delete records?", data)
//  2. PauseHandler.Confirm() returns ErrAuthRequired
//  3. Engine catches error → tsm.NeedsAuth() → emits ToolNeedsAuth event
//  4. SSE pushes event to frontend → approval dialog
//  5. User approves → POST /resume → engine resumes with approval
//  6. Tool continues execution
type PauseHandler struct{}

// NewPauseHandler creates a new PauseHandler for web/SSE contexts.
func NewPauseHandler() *PauseHandler {
	return &PauseHandler{}
}

// Confirm always returns ErrAuthRequired, causing the engine to pause
// and request user authorization via the existing event system.
func (h *PauseHandler) Confirm(_ context.Context, prompt string, data any) (bool, error) {
	resource := "tool_execution"
	if toolName, ok := data.(string); ok && toolName != "" {
		resource = "tool:" + toolName
	}

	return false, &ErrAuthRequired{
		Resource: resource,
		Action:   "execute",
		Reason:   prompt,
	}
}

// Read always returns ErrInputRequired, causing the engine to pause
// and request user input via the existing event system.
func (h *PauseHandler) Read(_ context.Context, prompt string, schema map[string]any) (string, error) {
	return "", &ErrInputRequired{
		Question: prompt,
		Schema:   schema,
	}
}

// Verify PauseHandler implements Handler.
var _ Handler = (*PauseHandler)(nil)
