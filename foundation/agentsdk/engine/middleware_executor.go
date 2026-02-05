package engine

import (
	"context"

	"school-with-ai/foundation/agent"
)

// ExecuteWithMiddleware runs a handler through the middleware chain.
// This is the integration point for the unified middleware system.
func ExecuteWithMiddleware(
	middlewares []agent.Middleware,
	c *agent.Context,
	final agent.Handler,
) error {
	if len(middlewares) == 0 {
		return final.Handle(c)
	}
	chain := agent.Chain(middlewares...)
	return chain(final).Handle(c)
}

// ExecuteAgentMiddleware executes middleware for an agent operation.
func ExecuteAgentMiddleware(
	ctx context.Context,
	middlewares []agent.Middleware,
	agentInstance agent.Agent,
	messages []agent.Message,
	session agent.Session,
	streaming bool,
	handler func() error,
) error {
	c := agent.NewAgentContext(ctx, agentInstance, messages, session, streaming)
	return ExecuteWithMiddleware(middlewares, c, agent.HandlerFunc(func(*agent.Context) error {
		return handler()
	}))
}

// ExecuteChatMiddleware executes middleware for a chat/LLM operation.
func ExecuteChatMiddleware(
	ctx context.Context,
	middlewares []agent.Middleware,
	model agent.Model,
	messages []agent.Message,
	options *agent.ChatOptions,
	handler func() error,
) error {
	c := agent.NewChatContext(ctx, model, messages, options)
	return ExecuteWithMiddleware(middlewares, c, agent.HandlerFunc(func(*agent.Context) error {
		return handler()
	}))
}

// ExecuteFunctionMiddleware executes middleware for a function/tool operation.
func ExecuteFunctionMiddleware(
	ctx context.Context,
	middlewares []agent.Middleware,
	tool agent.Tool,
	arguments map[string]any,
	handler func() error,
) error {
	c := agent.NewFunctionContext(ctx, tool, arguments)
	return ExecuteWithMiddleware(middlewares, c, agent.HandlerFunc(func(*agent.Context) error {
		return handler()
	}))
}
