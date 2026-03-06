package engine

import (
	"context"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
)

// ToolExecutionService handles executor creation and tool call execution.
// It centralizes all tool execution logic - no persistence, no event emission.
type ToolExecutionService struct {
	tracer core.ToolTracer
}

// NewToolExecutionService creates a new ToolExecutionService.
func NewToolExecutionService(tracer core.ToolTracer) *ToolExecutionService {
	return &ToolExecutionService{
		tracer: tracer,
	}
}

// CreateExecutor creates a tool executor initialized with the given tools.
func (s *ToolExecutionService) CreateExecutor(
	ctx context.Context,
	tools []agent.ExecutableTool,
	threadID string,
	userID string,
	messageID string,
	reqCtx *core.RequestContext,
) core.ToolExecutor {
	registry := core.NewToolRegistry()
	for _, tool := range tools {
		registry.Register(tool)
	}

	var opts []core.ToolExecutorOption
	if s.tracer != nil {
		opts = append(opts, core.WithToolTracer(s.tracer))
	}

	return core.NewRegistryToolExecutor(
		ctx,
		registry,
		threadID,
		userID,
		messageID,
		reqCtx,
		opts...,
	)
}
