package agent

import (
	"context"
)

// Context keys for distributed tracing.
// These are vendor-neutral — concrete tracing implementations (Langfuse,
// OpenTelemetry, etc.) map their IDs to/from these keys.
type traceContextKey string

const (
	traceIDKey      traceContextKey = "trace_id"
	parentSpanIDKey traceContextKey = "parent_span_id"
)

// WithTraceContext attaches trace identifiers to a context.
// traceID identifies the overall trace; parentSpanID identifies
// the parent span for nested observations.
func WithTraceContext(ctx context.Context, traceID, parentSpanID string) context.Context {
	ctx = context.WithValue(ctx, traceIDKey, traceID)
	ctx = context.WithValue(ctx, parentSpanIDKey, parentSpanID)
	return ctx
}

// ExtractTraceContext retrieves trace identifiers from a context.
func ExtractTraceContext(ctx context.Context) (traceID string, parentSpanID string) {
	if tid, ok := ctx.Value(traceIDKey).(string); ok {
		traceID = tid
	}
	if pid, ok := ctx.Value(parentSpanIDKey).(string); ok {
		parentSpanID = pid
	}
	return traceID, parentSpanID
}
