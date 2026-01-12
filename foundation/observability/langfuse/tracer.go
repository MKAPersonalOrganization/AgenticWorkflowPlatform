package langfuse

import (
	"context"
	"time"
)

// Tracer provides high-level tracing functionality
type Tracer struct {
	client *Client
}

// NewTracer creates a new Tracer instance
func NewTracer(client *Client) *Tracer {
	return &Tracer{
		client: client,
	}
}

// TraceContext holds trace and span information
type TraceContext struct {
	TraceID             string
	ParentObservationID string
}

// StartTrace begins a new trace
func (t *Tracer) StartTrace(ctx context.Context, name string, opts ...TraceOption) (*TraceContext, error) {
	trace := NewTrace(name)

	// Apply options
	for _, opt := range opts {
		opt(trace)
	}

	// Create trace in Langfuse
	if err := t.client.CreateTrace(ctx, trace); err != nil {
		return nil, err
	}

	return &TraceContext{
		TraceID: trace.ID,
	}, nil
}

// StartGeneration begins tracking an LLM generation
func (t *Tracer) StartGeneration(ctx context.Context, traceCtx *TraceContext, name, model string, opts ...GenerationOption) (*GenerationTracker, error) {
	gen := NewGeneration(traceCtx.TraceID, name, model)
	gen.ParentObservationID = traceCtx.ParentObservationID

	// Apply options
	for _, opt := range opts {
		opt(gen)
	}

	// Create generation in Langfuse
	if err := t.client.CreateGeneration(ctx, gen); err != nil {
		return nil, err
	}

	return &GenerationTracker{
		client:     t.client,
		generation: gen,
	}, nil
}

// StartSpan begins a new span within a trace
func (t *Tracer) StartSpan(ctx context.Context, traceCtx *TraceContext, name string, opts ...SpanOption) (*SpanTracker, error) {
	span := NewSpan(traceCtx.TraceID, name)
	span.ParentObservationID = traceCtx.ParentObservationID

	// Apply options
	for _, opt := range opts {
		opt(span)
	}

	// Create span in Langfuse
	if err := t.client.CreateSpan(ctx, span); err != nil {
		return nil, err
	}

	return &SpanTracker{
		client: t.client,
		span:   span,
	}, nil
}

// LogEvent logs a discrete event within a trace
func (t *Tracer) LogEvent(ctx context.Context, traceCtx *TraceContext, name string, opts ...EventOption) error {
	event := NewEvent(traceCtx.TraceID, name)
	event.ParentObservationID = traceCtx.ParentObservationID

	// Apply options
	for _, opt := range opts {
		opt(event)
	}

	return t.client.CreateEvent(ctx, event)
}

// GenerationTracker tracks an ongoing LLM generation
type GenerationTracker struct {
	client     *Client
	generation *Generation
}

// End completes the generation with output and usage
func (g *GenerationTracker) End(ctx context.Context, output any, usage *Usage) error {
	now := time.Now()
	g.generation.EndTime = &now
	g.generation.Output = output
	g.generation.Usage = usage

	return g.client.UpdateGeneration(ctx, g.generation)
}

// EndWithError completes the generation with an error
func (g *GenerationTracker) EndWithError(ctx context.Context, err error) error {
	now := time.Now()
	g.generation.EndTime = &now
	g.generation.Level = "ERROR"
	g.generation.StatusMessage = err.Error()

	return g.client.UpdateGeneration(ctx, g.generation)
}

// GetObservationID returns the generation's ID for use as a parent
func (g *GenerationTracker) GetObservationID() string {
	return g.generation.ID
}

// SpanTracker tracks an ongoing span
type SpanTracker struct {
	client *Client
	span   *Span
}

// End completes the span
func (s *SpanTracker) End(ctx context.Context, output any) error {
	now := time.Now()
	s.span.EndTime = &now
	s.span.Output = output

	return s.client.CreateSpan(ctx, s.span)
}

// EndWithError completes the span with an error
func (s *SpanTracker) EndWithError(ctx context.Context, err error) error {
	now := time.Now()
	s.span.EndTime = &now
	s.span.Level = "ERROR"
	s.span.StatusMessage = err.Error()

	return s.client.CreateSpan(ctx, s.span)
}

// GetObservationID returns the span's ID for use as a parent
func (s *SpanTracker) GetObservationID() string {
	return s.span.ID
}

// Option types

// TraceOption is a functional option for configuring a trace
type TraceOption func(*Trace)

// WithTraceUserID sets the user ID for the trace
func WithTraceUserID(userID string) TraceOption {
	return func(t *Trace) {
		t.UserID = userID
	}
}

// WithTraceSessionID sets the session ID for the trace
func WithTraceSessionID(sessionID string) TraceOption {
	return func(t *Trace) {
		t.SessionID = sessionID
	}
}

// WithTraceMetadata sets metadata for the trace
func WithTraceMetadata(metadata map[string]any) TraceOption {
	return func(t *Trace) {
		t.Metadata = metadata
	}
}

// WithTraceTags sets tags for the trace
func WithTraceTags(tags []string) TraceOption {
	return func(t *Trace) {
		t.Tags = tags
	}
}

// WithTraceRelease sets the release version for the trace
func WithTraceRelease(release string) TraceOption {
	return func(t *Trace) {
		t.Release = release
	}
}

// WithTraceVersion sets the component/model version for the trace
func WithTraceVersion(version string) TraceOption {
	return func(t *Trace) {
		t.Version = version
	}
}

// GenerationOption is a functional option for configuring a generation
type GenerationOption func(*Generation)

// WithGenerationInput sets the input for the generation
func WithGenerationInput(input any) GenerationOption {
	return func(g *Generation) {
		g.Input = input
	}
}

// WithGenerationMetadata sets metadata for the generation
func WithGenerationMetadata(metadata map[string]any) GenerationOption {
	return func(g *Generation) {
		g.Metadata = metadata
	}
}

// WithGenerationModelParameters sets model parameters
func WithGenerationModelParameters(params map[string]any) GenerationOption {
	return func(g *Generation) {
		g.ModelParameters = params
	}
}

// SpanOption is a functional option for configuring a span
type SpanOption func(*Span)

// WithSpanInput sets the input for the span
func WithSpanInput(input any) SpanOption {
	return func(s *Span) {
		s.Input = input
	}
}

// WithSpanMetadata sets metadata for the span
func WithSpanMetadata(metadata map[string]any) SpanOption {
	return func(s *Span) {
		s.Metadata = metadata
	}
}

// EventOption is a functional option for configuring an event
type EventOption func(*Event)

// WithEventInput sets the input for the event
func WithEventInput(input any) EventOption {
	return func(e *Event) {
		e.Input = input
	}
}

// WithEventOutput sets the output for the event
func WithEventOutput(output any) EventOption {
	return func(e *Event) {
		e.Output = output
	}
}

// WithEventMetadata sets metadata for the event
func WithEventMetadata(metadata map[string]any) EventOption {
	return func(e *Event) {
		e.Metadata = metadata
	}
}

// WithEventLevel sets the level for the event
func WithEventLevel(level string) EventOption {
	return func(e *Event) {
		e.Level = level
	}
}
