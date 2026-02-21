// Package io provides a pluggable IO abstraction for tool-user interaction.
//
// This is the Go equivalent of BeeAI Framework's IOHandlers system.
// Tools call io.Confirm() or io.Read() instead of returning metadata-based
// signaling. The underlying Handler decides how to satisfy the request:
//
//   - PauseHandler (web/SSE): returns sentinel errors that cause the engine to
//     pause and wait for user input via a separate HTTP request.
//   - StdinHandler (CLI): blocks on stdin until the user responds.
//   - NoopHandler (testing): auto-approves everything.
//
// Handler is injected via context.Context (like Go's context values), mirroring
// BeeAI's Python contextvars / ContextVar pattern.
//
// # Example (tool using IO)
//
//	func (t *DangerousTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
//	    approved, err := io.Confirm(ctx, "Delete 500 records?", args.Raw)
//	    if err != nil {
//	        return agent.Result{}, err  // sentinel error pauses the engine
//	    }
//	    if !approved {
//	        return agent.TextResult("Operation cancelled by user."), nil
//	    }
//	    // proceed with dangerous operation...
//	}
//
// # Example (engine setup)
//
//	ctx = io.WithHandler(ctx, io.NewPauseHandler())  // web
//	ctx = io.WithHandler(ctx, io.NewStdinHandler())   // CLI
package io

import (
	"context"
	"fmt"
)

// Handler defines the pluggable interface for tool-user interaction.
//
// This is the Go equivalent of BeeAI's IOHandlers dataclass with
// read() and confirm() callbacks.
//
// Implementations control the transport:
//   - PauseHandler: returns sentinel errors -> engine pauses -> SSE resumes
//   - StdinHandler: blocks on stdin -> synchronous CLI interaction
//   - NoopHandler:  auto-approves -> non-interactive testing
type Handler interface {
	// Confirm asks the user a yes/no approval question.
	//
	// Returns true if the user approves, false if denied.
	// For web contexts, the handler returns an ErrAuthRequired sentinel error
	// that causes the engine to pause and wait for user response.
	//
	// Parameters:
	//   - ctx: execution context (may be cancelled)
	//   - prompt: human-readable question shown to the user
	//   - data: optional structured data for rich UI rendering (tool args, etc.)
	Confirm(ctx context.Context, prompt string, data any) (bool, error)

	// Read asks the user for structured or free-form input.
	//
	// Returns the user's text response.
	// For web contexts, the handler returns an ErrInputRequired sentinel error
	// that causes the engine to pause and wait for user response.
	//
	// Parameters:
	//   - ctx: execution context (may be cancelled)
	//   - prompt: human-readable question shown to the user
	//   - schema: optional JSON Schema describing expected input shape
	Read(ctx context.Context, prompt string, schema map[string]any) (string, error)
}

// ---------------------------------------------------------------------------
// Context key injection (Go equivalent of BeeAI's Python contextvars)
// ---------------------------------------------------------------------------

// contextKey is unexported to prevent collisions with other packages.
type contextKey struct{}

// WithHandler returns a new context with the IO Handler set.
// This is the Go equivalent of BeeAI's setup_io_context().
//
//	ctx = io.WithHandler(ctx, io.NewPauseHandler())
func WithHandler(ctx context.Context, h Handler) context.Context {
	return context.WithValue(ctx, contextKey{}, h)
}

// FromContext retrieves the IO Handler from context.
// Returns nil if no handler has been set.
//
// This is the Go equivalent of BeeAI's _io_context.get().
func FromContext(ctx context.Context) Handler {
	h, _ := ctx.Value(contextKey{}).(Handler)
	return h
}

// ---------------------------------------------------------------------------
// Convenience functions (used by tools - mirrors BeeAI's io_confirm/io_read)
// ---------------------------------------------------------------------------

// Confirm is a convenience function that retrieves the Handler from context
// and calls Confirm on it.
//
// If no handler is configured, it returns (true, nil) - auto-approve.
// This fail-open default ensures tools work without explicit IO configuration.
//
// This is the Go equivalent of BeeAI's io_confirm().
func Confirm(ctx context.Context, prompt string, data any) (bool, error) {
	h := FromContext(ctx)
	if h == nil {
		return true, nil // fail-open: auto-approve when no handler
	}
	return h.Confirm(ctx, prompt, data)
}

// Read is a convenience function that retrieves the Handler from context
// and calls Read on it.
//
// If no handler is configured, it returns an error because user input
// cannot be synthesized.
//
// This is the Go equivalent of BeeAI's io_read().
func Read(ctx context.Context, prompt string, schema map[string]any) (string, error) {
	h := FromContext(ctx)
	if h == nil {
		return "", fmt.Errorf("io: no Handler configured; cannot read user input")
	}
	return h.Read(ctx, prompt, schema)
}
