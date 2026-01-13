package langfuse

import (
	"context"
	"school-with-ai/sdk/agentsdk/core"
	"time"
)

// LangfuseToolTracer implements core.ToolTracer for tracking tool calls in
type LangfuseToolTracer struct {
	tracer *Tracer
}

// NewLangfuseToolTracer creates a new Langfuse tool tracer.
func NewLangfuseToolTracer(tracer *Tracer) *LangfuseToolTracer {
	return &LangfuseToolTracer{
		tracer: tracer,
	}
}

// TrackToolCall tracks a tool execution in Langfuse as an event.
func (t *LangfuseToolTracer) TrackToolCall(
	ctx context.Context,
	traceID string,
	parentObservationID string,
	toolName string,
	input string,
	output string,
	startTime time.Time,
	endTime time.Time,
	err error,
) error {
	if t.tracer == nil {
		return nil
	}

	traceCtx := &TraceContext{
		TraceID:             traceID,
		ParentObservationID: parentObservationID,
	}

	// Prepare event options
	eventOpts := []EventOption{
		WithEventInput(map[string]any{
			"tool":      toolName,
			"arguments": input,
		}),
		WithEventMetadata(map[string]any{
			"startTime": startTime,
			"endTime":   endTime,
			"duration":  endTime.Sub(startTime).String(),
		}),
	}

	// Set output or error
	if err != nil {
		eventOpts = append(eventOpts, WithEventLevel("ERROR"))
		eventOpts = append(eventOpts, WithEventOutput(map[string]any{
			"error": err.Error(),
		}))
	} else {
		eventOpts = append(eventOpts, WithEventOutput(map[string]any{
			"result": output,
		}))
	}

	// Log the tool call as an event
	return t.tracer.LogEvent(ctx, traceCtx, "tool-call", eventOpts...)
}

// Verify interface implementation
var _ core.ToolTracer = (*LangfuseToolTracer)(nil)
