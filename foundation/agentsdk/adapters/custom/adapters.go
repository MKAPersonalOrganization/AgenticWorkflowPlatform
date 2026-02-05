package custom

import (
	"context"
	"fmt"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
)

// ExecutableToolAdapter adapts agent.Tool to agent.ExecutableTool.
// This is used when a tool only implements the metadata interface (agent.Tool)
// but needs to be used where execution is required (agent.ExecutableTool).
type ExecutableToolAdapter struct {
	tool     agent.Tool
	executor agent.Executor
}

// NewExecutableToolAdapter creates an adapter from agent.Tool and agent.Executor.
// If the tool already implements agent.ExecutableTool, it returns it directly.
func NewExecutableToolAdapter(tool agent.Tool) agent.ExecutableTool {
	// Check if tool already implements ExecutableTool
	if et, ok := tool.(agent.ExecutableTool); ok {
		return et
	}

	// Check if tool implements Executor separately
	var executor agent.Executor
	if ex, ok := tool.(agent.Executor); ok {
		executor = ex
	}

	return &ExecutableToolAdapter{
		tool:     tool,
		executor: executor,
	}
}

func (a *ExecutableToolAdapter) Name() string {
	return a.tool.Name()
}

func (a *ExecutableToolAdapter) Description() string {
	return a.tool.Description()
}

func (a *ExecutableToolAdapter) Schema() *agent.Schema {
	return a.tool.Schema()
}

func (a *ExecutableToolAdapter) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	if a.executor != nil {
		return a.executor.Execute(ctx, args)
	}
	// If no executor, return error
	return agent.Result{}, fmt.Errorf("tool %s does not implement Execute method", a.tool.Name())
}

// MiddlewareAdapter adapts agent.RequestMiddleware to core.Middleware.
type MiddlewareAdapter struct {
	middleware agent.RequestMiddleware
}

// NewMiddlewareAdapter creates an adapter from agent.RequestMiddleware.
func NewMiddlewareAdapter(middleware agent.RequestMiddleware) core.Middleware {
	return &MiddlewareAdapter{
		middleware: middleware,
	}
}

func (a *MiddlewareAdapter) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Convert core.RequestContext to agent.RequestContext
	// Note: agent.RequestContext has different structure, we'll pass through what we can
	agentReqCtx := &agent.RequestContext{}

	// Create adapter for next handler
	agentNext := func(ctx context.Context, agentCtx *agent.RequestContext) error {
		// Call the core next handler
		return next(ctx, reqCtx)
	}

	// Call the agent middleware
	return a.middleware.Process(ctx, agentReqCtx, agentNext)
}
