package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"



	"school-with-ai/foundation/agent"
)

// ErrToolNotFound is returned when a tool is not registered.
var ErrToolNotFound = errors.New("tool not found")

// ToolExecResult contains the result of tool execution.
// This struct exposes Command metadata to allow the engine to detect
// routing commands without checking tool names.
type ToolExecResult struct {
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
	Execute(toolCall ToolCall) (ToolExecResult, error)

	// Context returns the execution context.
	Context() context.Context
}

// ToolTracer defines the interface for tracking tool executions (e.g., Langfuse).
type ToolTracer interface {
	// TrackToolCall tracks a tool execution
	TrackToolCall(ctx context.Context, traceID, parentObservationID, toolName, input, output string, startTime, endTime time.Time, err error) error
}

// RegistryToolExecutor executes tools using a ToolRegistry.
type RegistryToolExecutor struct {
	registry   *ToolRegistry
	ctx        context.Context
	threadID   string
	userID     string
	messageID  string
	requestCtx *RequestContext
	tracer     ToolTracer // Optional tracer for observability
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
// The threadID, messageID, and requestCtx are optional and can be empty/nil.
// If provided, they will be made available to tools via ToolContext.
func NewRegistryToolExecutor(
	ctx context.Context,
	registry *ToolRegistry,
	threadID string,
	userID string,
	messageID string,
	requestCtx *RequestContext,
	opts ...ToolExecutorOption,
) *RegistryToolExecutor {
	executor := &RegistryToolExecutor{
		registry:   registry,
		ctx:        ctx,
		threadID:   threadID,
		userID:     userID,
		messageID:  messageID,
		requestCtx: requestCtx,
	}

	// Apply options
	for _, opt := range opts {
		opt(executor)
	}

	return executor
}

// Context returns the execution context.
func (e *RegistryToolExecutor) Context() context.Context {
	return e.ctx
}

// Execute runs a tool call using the registry.
func (e *RegistryToolExecutor) Execute(toolCall ToolCall) (ToolExecResult, error) {
	startTime := time.Now()

	tool, ok := e.registry.Get(toolCall.Name)
	if !ok {
		return ToolExecResult{}, fmt.Errorf("%w: %s", ErrToolNotFound, toolCall.Name)
	}

	// Create agent.Arguments from ToolCall
	args := agent.Arguments{
		Raw:    json.RawMessage(toolCall.Arguments),
		CallID: toolCall.ID,
	}

	// Populate standard fields
	args.SessionID = e.threadID
	args.UserID = e.userID


	// Populate legacy fields if requestCtx is available
	if e.requestCtx != nil {
		args.Metadata = e.requestCtx.State
	}

	// Execute tool with new spec-compliant interface
	result, err := tool.Execute(e.ctx, args)
	endTime := time.Now()

	// Build execution result
	execResult := ToolExecResult{
		StopIteration: false,
	}

	// Handle error from result (Execute returns error separately)
	if err == nil && result.Error != nil {
		err = result.Error
	}

	// Handle result content
	if len(result.Content) > 0 {
		// Check if the first content item is a ToolResult (for routing commands)
		if len(result.Content) == 1 {
			if tr, ok := result.Content[0].(ToolResult); ok {
				// Extract routing command metadata
				execResult.Command = tr.Command
				if tr.Command != nil {
					execResult.StopIteration = tr.Command.StopIteration

					// Apply state updates if RequestCtx is available
					if e.requestCtx != nil && tr.Command.Update != nil {
						if e.requestCtx.State == nil {
							e.requestCtx.State = make(map[string]any)
						}
						for k, v := range tr.Command.Update {
							e.requestCtx.State[k] = v
						}
					}
				}
				// Use the wrapped result
				if tr.Result != nil {
					if str, ok := tr.Result.(string); ok {
						execResult.Result = str
					} else {
						resultBytes, marshalErr := json.Marshal(tr.Result)
						if marshalErr != nil {
							return ToolExecResult{}, fmt.Errorf("failed to marshal tool result: %w", marshalErr)
						}
						execResult.Result = string(resultBytes)
					}
				}
			} else {
				// Regular content - use directly if it's already a string,
				// otherwise marshal the item itself if it's the only one
				if str, ok := result.Content[0].(string); ok {
					execResult.Result = str
				} else {
					resultBytes, marshalErr := json.Marshal(result.Content[0])
					if marshalErr != nil {
						return ToolExecResult{}, fmt.Errorf("failed to marshal tool result: %w", marshalErr)
					}
					execResult.Result = string(resultBytes)
				}
			}
		} else {
			// Multiple content items - marshal the array
			resultBytes, marshalErr := json.Marshal(result.Content)
			if marshalErr != nil {
				return ToolExecResult{}, fmt.Errorf("failed to marshal tool result: %w", marshalErr)
			}
			execResult.Result = string(resultBytes)
		}
	}

	// Track tool execution if tracer is available
	if e.tracer != nil {
		// Extract trace context from ctx if available
		traceID, parentObservationID := extractTraceContext(e.ctx)
		if traceID != "" {
			trackErr := e.tracer.TrackToolCall(e.ctx, traceID, parentObservationID, toolCall.Name, toolCall.Arguments, execResult.Result, startTime, endTime, err)
			if trackErr != nil {
				// Log tracking error but don't fail the tool execution
				// Note: In production, you'd want proper logging here
				_ = trackErr
			}
		}
	}

	if err != nil {
		return execResult, fmt.Errorf("failed to execute tool call %s: %w", toolCall.Name, err)
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
