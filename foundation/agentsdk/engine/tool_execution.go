package engine

import (
	"context"

	"school-with-ai/foundation/agentsdk/core"
)

// ToolExecutionResult represents the result of executing a single tool
type ToolExecutionResult struct {
	ToolCallID    string               `json:"tool_call_id"`
	Result        *core.ToolExecResult `json:"result,omitempty"`
	Error         error                `json:"error,omitempty"`
	StopIteration bool                 `json:"stop_iteration"`
}

// ToolExecutionResults represents the results of executing multiple tools
type ToolExecutionResults struct {
	Results    []ToolExecutionResult `json:"results"`
	HasErrors  bool                  `json:"has_errors"`
	ErrorCount int                   `json:"error_count"`
	ShouldStop bool                  `json:"should_stop"`
}

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
	tools []core.Tool,
	threadID string,
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
		messageID,
		reqCtx,
		opts...,
	)
}

// ExecuteAll executes all tool calls and returns their results.
// It tracks whether any tool signals to stop iteration.
func (s *ToolExecutionService) ExecuteAll(toolCalls []core.ToolCall, executor core.ToolExecutor) ToolExecutionResults {
	results := ToolExecutionResults{
		Results:    make([]ToolExecutionResult, 0, len(toolCalls)),
		ShouldStop: false,
	}

	for _, tc := range toolCalls {
		result := s.Execute(tc, executor)
		results.Results = append(results.Results, result)

		if result.StopIteration {
			results.ShouldStop = true
		}
	}

	return results
}

// Execute executes a single tool call and returns the result.
func (s *ToolExecutionService) Execute(toolCall core.ToolCall, executor core.ToolExecutor) ToolExecutionResult {
	execResult, err := executor.Execute(toolCall)

	return ToolExecutionResult{
		ToolCallID:    toolCall.ID,
		Result:        &execResult,
		Error:         err,
		StopIteration: execResult.StopIteration,
	}
}
