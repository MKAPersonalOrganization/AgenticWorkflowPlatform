package engine

import (
	"context"
	"encoding/json"

	"school-with-ai/foundation/agent"
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
	tools []agent.ExecutableTool,
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
	return s.ExecuteAllWithMiddleware(executor.Context(), toolCalls, executor, nil)
}

// ExecuteAllWithMiddleware executes all tool calls with middleware and returns their results.
func (s *ToolExecutionService) ExecuteAllWithMiddleware(
	ctx context.Context,
	toolCalls []core.ToolCall,
	executor core.ToolExecutor,
	middleware []agent.Middleware,
) ToolExecutionResults {
	results := ToolExecutionResults{
		Results:    make([]ToolExecutionResult, 0, len(toolCalls)),
		ShouldStop: false,
	}

	for _, tc := range toolCalls {
		result := s.ExecuteWithMiddleware(ctx, tc, executor, middleware)
		results.Results = append(results.Results, result)

		if result.StopIteration {
			results.ShouldStop = true
		}
	}

	return results
}

// Execute executes a single tool call and returns the result.
func (s *ToolExecutionService) Execute(toolCall core.ToolCall, executor core.ToolExecutor) ToolExecutionResult {
	return s.ExecuteWithMiddleware(executor.Context(), toolCall, executor, nil)
}

// ExecuteWithMiddleware executes a single tool call with middleware and returns the result.
func (s *ToolExecutionService) ExecuteWithMiddleware(
	ctx context.Context,
	toolCall core.ToolCall,
	executor core.ToolExecutor,
	middleware []agent.Middleware,
) ToolExecutionResult {
	var execResult core.ToolExecResult
	var execErr error

	// Core execution logic
	doExecute := func() error {
		execResult, execErr = executor.Execute(toolCall)
		if execErr != nil && execResult.Result == "" {
			execResult.Result = execErr.Error()
		}
		return nil // Always return nil - we capture errors in execErr
	}

	// Wrap with FunctionMiddleware if present
	if len(middleware) > 0 {
		// Parse arguments JSON string to map for middleware context
		var argsMap map[string]any
		if toolCall.Arguments != "" {
			_ = json.Unmarshal([]byte(toolCall.Arguments), &argsMap)
		}
		// We don't have access to the actual Tool interface here,
		// so we pass nil for the tool argument
		_ = ExecuteFunctionMiddleware(
			ctx,
			middleware,
			nil, // Tool (not easily available here)
			argsMap,
			nil, // Metadata (not available in this legacy path)
			func(c *agent.Context) error {
				err := doExecute()
				c.Result = execResult
				return err
			},
		)
	} else {
		doExecute()
	}

	return ToolExecutionResult{
		ToolCallID:    toolCall.ID,
		Result:        &execResult,
		Error:         execErr,
		StopIteration: execResult.StopIteration,
	}
}
