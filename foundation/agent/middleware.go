package agent

import "context"

// OperationKind identifies the type of operation being processed.
type OperationKind int

const (
	OperationAgent OperationKind = iota
	OperationChat
	OperationFunction
)

// Context carries all operation data through the middleware chain.
// Embeds context.Context for cancellation/timeout support.
// Exactly one of Agent, Chat, or Function is non-nil.
type Context struct {
	context.Context

	// Operation type
	Kind OperationKind

	// Exactly one is non-nil
	Agent    *AgentOp
	Chat     *ChatOp
	Function *FunctionOp

	// Shared across middleware
	Metadata map[string]any

	// Result (set by handler)
	Result any
}

// Helper methods
func (c *Context) IsAgent() bool    { return c.Kind == OperationAgent }
func (c *Context) IsChat() bool     { return c.Kind == OperationChat }
func (c *Context) IsFunction() bool { return c.Kind == OperationFunction }

// AgentOp groups agent-level data.
type AgentOp struct {
	Agent     Agent
	Messages  []Message
	Session   Session
	Streaming bool
}

// ChatOp groups LLM call data.
type ChatOp struct {
	Model    Model
	Messages []Message
	Options  *ChatOptions
}

// ChatOptions holds LLM configuration.
type ChatOptions struct {
	Temperature   float64
	MaxTokens     int
	StopSequences []string
	Metadata      map[string]any
}

// FunctionOp groups tool call data.
type FunctionOp struct {
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

// NewAgentContext creates a Context for an agent operation.
func NewAgentContext(ctx context.Context, agent Agent, messages []Message, session Session, streaming bool) *Context {
	return &Context{
		Context: ctx,
		Kind:    OperationAgent,
		Agent: &AgentOp{
			Agent:     agent,
			Messages:  messages,
			Session:   session,
			Streaming: streaming,
		},
		Metadata: make(map[string]any),
	}
}

// NewChatContext creates a Context for a chat/LLM operation.
func NewChatContext(ctx context.Context, model Model, messages []Message, options *ChatOptions) *Context {
	return &Context{
		Context: ctx,
		Kind:    OperationChat,
		Chat: &ChatOp{
			Model:    model,
			Messages: messages,
			Options:  options,
		},
		Metadata: make(map[string]any),
	}
}

// NewFunctionContext creates a Context for a function/tool operation.
func NewFunctionContext(ctx context.Context, tool Tool, arguments map[string]any) *Context {
	return &Context{
		Context: ctx,
		Kind:    OperationFunction,
		Function: &FunctionOp{
			Tool:      tool,
			Arguments: arguments,
		},
		Metadata: make(map[string]any),
	}
}
