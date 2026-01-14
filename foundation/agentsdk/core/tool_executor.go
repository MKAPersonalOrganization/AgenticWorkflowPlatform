package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrToolNotFound is returned when a tool is not registered.
var ErrToolNotFound = errors.New("tool not found")

// ToolExecutionResult contains the result of tool execution.
// This struct exposes Command metadata to allow the engine to detect
// routing commands without checking tool names.
type ToolExecutionResult struct {
	// Result is the string representation for adding to thread
	Result string

	// Command is the routing command if tool returned one (nil otherwise)
	Command *Command

	// StopIteration indicates the engine should stop iterating
	// Extracted from Command.StopIteration for convenience
	StopIteration bool
}

// ToolExecutor executes tool calls and returns results.
type ToolExecutor interface {
	// Execute runs a tool call and returns the result with Command metadata
	Execute(toolCall ToolCall) (ToolExecutionResult, error)
}

// ToolTracer defines the interface for tracking tool executions (e.g., Langfuse).
type ToolTracer interface {
	// TrackToolCall tracks a tool execution
	TrackToolCall(ctx context.Context, traceID, parentObservationID, toolName, input, output string, startTime, endTime time.Time, err error) error
}

// RegistryToolExecutor executes tools using a ToolRegistry.
type RegistryToolExecutor struct {
	registry *ToolRegistry
	ctx      context.Context
	tracer   ToolTracer // Optional tracer for observability
}

// ToolExecutorOption is a functional option for configuring the tool executor.
type ToolExecutorOption func(*RegistryToolExecutor)

// WithToolTracer sets a tracer for tracking tool executions.
func WithToolTracer(tracer ToolTracer) ToolExecutorOption {
	return func(e *RegistryToolExecutor) {
		e.tracer = tracer
	}
}

// NewRegistryToolExecutor creates a new executor with the given registry.
func NewRegistryToolExecutor(ctx context.Context, registry *ToolRegistry, opts ...ToolExecutorOption) *RegistryToolExecutor {
	executor := &RegistryToolExecutor{
		registry: registry,
		ctx:      ctx,
	}

	// Apply options
	for _, opt := range opts {
		opt(executor)
	}

	return executor
}

// Execute runs a tool call using the registry.
func (e *RegistryToolExecutor) Execute(toolCall ToolCall) (ToolExecutionResult, error) {
	startTime := time.Now()

	tool, ok := e.registry.Get(toolCall.Function.Name)
	if !ok {
		return ToolExecutionResult{}, fmt.Errorf("%w: %s", ErrToolNotFound, toolCall.Function.Name)
	}

	result, err := tool.Invoke(e.ctx, toolCall.Function.Arguments)
	endTime := time.Now()

	// Build execution result with Command detection
	var execResult ToolExecutionResult

	if result != nil {
		// Check if result is a Command
		if cmd, isCmd := IsCommand(result); isCmd {
			execResult.Command = cmd
			execResult.StopIteration = cmd.StopIteration
			execResult.Result = fmt.Sprintf("Successfully transferred to %s", cmd.Goto)
		} else if str, ok := result.(string); ok {
			execResult.Result = str
		} else {
			execResult.Result = fmt.Sprintf("%v", result)
		}
	}

	// Track tool execution if tracer is available
	if e.tracer != nil {
		// Extract trace context from ctx if available
		traceID, parentObservationID := extractTraceContext(e.ctx)
		if traceID != "" {
			trackErr := e.tracer.TrackToolCall(e.ctx, traceID, parentObservationID, toolCall.Function.Name, toolCall.Function.Arguments, execResult.Result, startTime, endTime, err)
			if trackErr != nil {
				// Log tracking error but don't fail the tool execution
				// Note: In production, you'd want proper logging here
				_ = trackErr
			}
		}
	}

	if err != nil {
		return execResult, fmt.Errorf("failed to execute tool call %s: %w", toolCall.Function.Name, err)
	}

	return execResult, nil
}

// Context keys for trace information
type traceContextKey string

const (
	traceIDKey             traceContextKey = "langfuse_trace_id"
	parentObservationIDKey traceContextKey = "langfuse_parent_observation_id"
)

// WithTraceContext adds trace context to a context.
func WithTraceContext(ctx context.Context, traceID, parentObservationID string) context.Context {
	ctx = context.WithValue(ctx, traceIDKey, traceID)
	ctx = context.WithValue(ctx, parentObservationIDKey, parentObservationID)
	return ctx
}

// extractTraceContext extracts trace information from context.
func extractTraceContext(ctx context.Context) (traceID string, parentObservationID string) {
	if tid, ok := ctx.Value(traceIDKey).(string); ok {
		traceID = tid
	}
	if pid, ok := ctx.Value(parentObservationIDKey).(string); ok {
		parentObservationID = pid
	}
	return traceID, parentObservationID
}

// Verify RegistryToolExecutor implements ToolExecutor.
var _ ToolExecutor = (*RegistryToolExecutor)(nil)
