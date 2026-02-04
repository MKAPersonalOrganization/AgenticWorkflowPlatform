package agent

import "context"

// Middleware wraps a Handler and returns a new Handler with additional behavior.
// Middlewares are applied in a chain, with the first middleware being the outermost.
//
// This pattern is compatible with Blades' middleware system.
type Middleware func(Handler) Handler

// Handler processes an invocation and returns a stream of events.
// This is the core processing interface used by middleware.
type Handler interface {
	Handle(context.Context, *Invocation) Stream
}

// HandlerFunc is an adapter to allow ordinary functions as Handlers.
type HandlerFunc func(context.Context, *Invocation) Stream

// Handle implements Handler for HandlerFunc.
func (f HandlerFunc) Handle(ctx context.Context, inv *Invocation) Stream {
	return f(ctx, inv)
}

// ChainMiddlewares composes multiple middlewares into one.
// The first middleware becomes the outermost wrapper.
//
// Example:
//
//	chain := ChainMiddlewares(logging, recovery, auth)
//	handler := chain(finalHandler)
func ChainMiddlewares(mws ...Middleware) Middleware {
	return func(next Handler) Handler {
		h := next
		for i := len(mws) - 1; i >= 0; i-- {
			h = mws[i](h)
		}
		return h
	}
}

// BeforeAgentCallback is called before an agent starts its run.
// If it returns non-nil content or error, the agent run is skipped.
type BeforeAgentCallback func(CallbackContext) (*Message, error)

// AfterAgentCallback is called after an agent completes its run.
// If it returns non-nil content or error, a new event is created.
type AfterAgentCallback func(CallbackContext) (*Message, error)

// BeforeToolCallback is called before a tool executes.
// If it returns non-nil result or error, the tool execution is skipped.
// To modify arguments and still run the tool, update args in place and return (nil, nil).
type BeforeToolCallback func(ctx context.Context, t Tool, args map[string]any) (map[string]any, error)

// AfterToolCallback is called after a tool completes.
// If it returns non-nil result or error, it replaces the actual result.
type AfterToolCallback func(ctx context.Context, t Tool, result *ToolResult, err error) (*ToolResult, error)
