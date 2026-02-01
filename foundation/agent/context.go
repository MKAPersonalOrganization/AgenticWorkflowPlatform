package agent

import (
	"context"
)

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

// ExtractTraceContext extracts trace information from context.
func ExtractTraceContext(ctx context.Context) (traceID string, parentObservationID string) {
	if tid, ok := ctx.Value(traceIDKey).(string); ok {
		traceID = tid
	}
	if pid, ok := ctx.Value(parentObservationIDKey).(string); ok {
		parentObservationID = pid
	}
	return traceID, parentObservationID
}
