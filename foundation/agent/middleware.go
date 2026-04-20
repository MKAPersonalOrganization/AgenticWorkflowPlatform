package agent

import "context"

// OperationKind identifies the type of operation being processed.
type OperationKind int

const (
	OperationAgent OperationKind = iota
	OperationTool
)

// Context carries all operation data through the middleware chain.
// Embeds context.Context for cancellation/timeout support.
// Exactly one of Agent or Tool is non-nil.
type Context struct {
	context.Context

	// Operation type
	Kind OperationKind

	// Exactly one is non-nil
	Agent *AgentOp
	Tool  *ToolOp

	// Shared across middleware
	Metadata map[string]any

	// DryRun indicates this is a configuration/preparation call, not actual execution.
	// Middlewares that have side effects (logging, etc.) should avoid processing dry runs.
	DryRun bool

	// Result (set by handler)
	Result any
}

// Helper methods
func (c *Context) IsAgent() bool { return c.Kind == OperationAgent }

// IsTool reports whether this context wraps a tool invocation.
func (c *Context) IsTool() bool { return c.Kind == OperationTool }

// AgentOp groups agent-level data.
type AgentOp struct {
	Agent     Agent
	Messages  []Message
	Streaming bool
}

// ToolOp groups tool call data for middleware execution.
// This is the primary type for tool-related middleware contexts,
// aligned with ADK-Go's "tool" terminology.
type ToolOp struct {
	Tool      Tool
	Arguments map[string]any
}

// Handler processes operations.
type Handler interface {
	Handle(*Context) error
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(*Context) error

func (f HandlerFunc) Handle(c *Context) error { return f(c) }

// Middleware wraps a Handler and returns a new Handler.
// Middlewares are applied in a chain, with the first being outermost.
type Middleware func(Handler) Handler

// Chain combines multiple middlewares into one.
// The first middleware becomes the outermost wrapper.
//
// Example:
//
//	chain := Chain(logging, recovery, auth)
//	handler := chain(finalHandler)
func Chain(mws ...Middleware) Middleware {
	return func(final Handler) Handler {
		for i := len(mws) - 1; i >= 0; i-- {
			final = mws[i](final)
		}
		return final
	}
}

// NewToolContext creates a Context for a tool invocation.
func NewToolContext(ctx context.Context, tool Tool, arguments map[string]any) *Context {
	return &Context{
		Context: ctx,
		Kind:    OperationTool,
		Tool: &ToolOp{
			Tool:      tool,
			Arguments: arguments,
		},
		Metadata: make(map[string]any),
	}
}
