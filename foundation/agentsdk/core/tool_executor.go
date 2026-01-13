package core

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrToolNotFound is returned when a tool is not registered.
var ErrToolNotFound = errors.New("tool not found")

// ToolExecutor executes tool calls and returns results.
type ToolExecutor interface {
	// Execute runs a tool call and returns the result
	Execute(toolCall ToolCall) (string, error)
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
func (e *RegistryToolExecutor) Execute(toolCall ToolCall) (string, error) {
	startTime := time.Now()

	tool, ok := e.registry.Get(toolCall.Function.Name)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrToolNotFound, toolCall.Function.Name)
	}

	result, err := tool.Invoke(e.ctx, toolCall.Function.Arguments)
	endTime := time.Now()

	// Track tool execution if tracer is available
	if e.tracer != nil {
		// Extract trace context from ctx if available
		traceID, parentObservationID := extractTraceContext(e.ctx)
		if traceID != "" {
			trackErr := e.tracer.TrackToolCall(e.ctx, traceID, parentObservationID, toolCall.Function.Name, toolCall.Function.Arguments, result, startTime, endTime, err)
			if trackErr != nil {
				// Log tracking error but don't fail the tool execution
				// Note: In production, you'd want proper logging here
				_ = trackErr
			}
		}
	}

	if err != nil {
		return result, fmt.Errorf("failed to invoke tool %s: %w", toolCall.Function.Name, err)
	}

	return result, nil
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
