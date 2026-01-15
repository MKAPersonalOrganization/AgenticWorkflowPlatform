package middleware

import (
	"context"
	"school-with-ai/foundation/agentsdk/core"
)

// SubagentMiddleware automatically converts subagents from AgentConfig into tools.
type SubagentMiddleware struct{}

// NewSubagentMiddleware creates a new SubagentMiddleware.
func NewSubagentMiddleware() *SubagentMiddleware {
	return &SubagentMiddleware{}
}

// Process implements the Middleware interface.
func (m *SubagentMiddleware) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	if reqCtx.Config == nil {
		return next(ctx, reqCtx)
	}

	// If no runner is configured, we can't run subagents
	if reqCtx.Config.Runner == nil {
		return next(ctx, reqCtx)
	}

	// Add a SubagentTool for each subagent in the config
	for _, subAgentConfig := range reqCtx.Config.SubAgents {
		reqCtx.AddTool(core.NewSubagentTool(subAgentConfig, reqCtx.Config.Runner))
	}

	return next(ctx, reqCtx)
}

// Verify SubagentMiddleware implements Middleware.
var _ core.Middleware = (*SubagentMiddleware)(nil)
