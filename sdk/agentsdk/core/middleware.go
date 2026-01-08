package core

import (
	"context"
	"errors"
	"fmt"
	"school-with-ai/sdk/agentsdk/llm"
	"strings"
)

var (
	ErrNoEventNotifier     = errors.New("no event notifier configured")
	ErrEventEmissionFailed = errors.New("failed to handle domain event")
)

// Middleware provides a way to intercept and modify agent execution.
// Middleware can inspect and modify the request context before and after
// the next handler executes, enabling cross-cutting concerns like logging,
// retry logic, task management, and request/response transformation.
//
// Example:
//
//	type LoggingMiddleware struct{}
//
//	func (m *LoggingMiddleware) Process(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
//	    log.Printf("Before: %d messages", len(reqCtx.Messages))
//	    err := next(ctx, reqCtx)
//	    log.Printf("After: result=%v", reqCtx.Result)
//	    return err
//	}
type Middleware interface {
	// Process executes the middleware logic.
	// It receives the request context and a next handler to continue the chain.
	// The middleware can:
	// - Inspect and modify reqCtx before calling next()
	// - Call next() to continue execution (or skip it to short-circuit)
	// - Inspect and modify reqCtx after next() returns
	// - Return an error to halt execution
	Process(ctx context.Context, reqCtx *RequestContext, next NextHandler) error
}

// NextHandler is a function that continues middleware chain execution.
// It processes the request context and may modify it.
type NextHandler func(ctx context.Context, reqCtx *RequestContext) error

// RequestContext carries all state through the middleware chain.
// It contains the request data, agent configuration, and execution state.
type RequestContext struct {
	// ThreadID is the identifier for the conversation thread
	ThreadID string

	// Messages are the conversation messages being sent to the LLM
	Messages []llm.ChatMessage

	// SystemPrompt is the system-level instruction for the agent
	SystemPrompt string

	// Tools are the available tools/functions for this request
	Tools []Tool

	// Config is the agent configuration
	Config *AgentConfig

	// State is a key-value store for sharing data between middleware
	// Middleware can use this to store and retrieve arbitrary data
	State map[string]any

	// Result contains the execution result after the base handler runs
	// For LLM calls, this will be the chat response
	Result any

	// Terminate indicates whether to stop execution after current middleware
	// When true, the middleware chain will not continue
	Terminate bool

	// AdditionalTools are tools added by middleware
	// These will be merged with the base Tools
	AdditionalTools []Tool

	// SystemPromptAdditions are system prompt fragments added by middleware
	// These will be appended to the base SystemPrompt
	SystemPromptAdditions []string
}

// NewRequestContext creates a new RequestContext with the given parameters.
func NewRequestContext(threadID string, messages []llm.ChatMessage, systemPrompt string, tools []Tool, config *AgentConfig) *RequestContext {
	return &RequestContext{
		ThreadID:              threadID,
		Messages:              messages,
		SystemPrompt:          systemPrompt,
		Tools:                 tools,
		Config:                config,
		State:                 make(map[string]any),
		AdditionalTools:       []Tool{},
		SystemPromptAdditions: []string{},
		Terminate:             false,
	}
}

// WithMetadata adds a key-value pair to the context state.
// This is useful for middleware to share information.
func (rc *RequestContext) WithMetadata(key string, value any) *RequestContext {
	rc.State[key] = value

	return rc
}

// GetMetadata retrieves a value from the context state.
// Returns the value and a boolean indicating if the key exists.
func (rc *RequestContext) GetMetadata(key string) (any, bool) {
	val, ok := rc.State[key]

	return val, ok
}

// SetTerminate marks the context for termination.
// When called, the middleware chain will stop after the current middleware completes.
func (rc *RequestContext) SetTerminate() {
	rc.Terminate = true
}

// AddTool adds a tool that will be available for this request.
// This allows middleware to dynamically inject tools.
func (rc *RequestContext) AddTool(tool Tool) {
	rc.AdditionalTools = append(rc.AdditionalTools, tool)
}

// AddSystemPrompt appends additional text to the system prompt.
// This allows middleware to inject instructions without replacing the base prompt.
func (rc *RequestContext) AddSystemPrompt(text string) {
	if text != "" {
		rc.SystemPromptAdditions = append(rc.SystemPromptAdditions, text)
	}
}

// EmitEvent emits a domain event using the configured notifier.
// This is a convenience method for tools and middleware to emit events.
// Returns an error if no notifier is configured or if the event emission fails.
func (rc *RequestContext) EmitEvent(event Event) error {
	if rc.Config == nil || rc.Config.Notifier == nil {
		return ErrNoEventNotifier
	}

	err := rc.Config.Notifier.HandleDomainEvent(event)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrEventEmissionFailed, err)
	}

	return nil
}

// GetEffectiveSystemPrompt returns the combined system prompt including
// all additions from middleware.
func (rc *RequestContext) GetEffectiveSystemPrompt() string {
	if len(rc.SystemPromptAdditions) == 0 {
		return rc.SystemPrompt
	}

	result := rc.SystemPrompt
	var resultSb146 strings.Builder
	for _, addition := range rc.SystemPromptAdditions {
		if addition != "" {
			resultSb146.WriteString("\n\n" + addition)
		}
	}
	result += resultSb146.String()

	return result
}

// GetEffectiveTools returns all tools including those added by middleware.
func (rc *RequestContext) GetEffectiveTools() []Tool {
	if len(rc.AdditionalTools) == 0 {
		return rc.Tools
	}

	allTools := make([]Tool, 0, len(rc.Tools)+len(rc.AdditionalTools))
	allTools = append(allTools, rc.Tools...)
	allTools = append(allTools, rc.AdditionalTools...)

	return allTools
}

// ContextOption is a function that modifies a RequestContext.
// This enables the builder pattern for context modifications.
type ContextOption func(*RequestContext)

// WithMessages sets the messages in the context.
func WithMessages(messages []llm.ChatMessage) ContextOption {
	return func(rc *RequestContext) {
		rc.Messages = messages
	}
}

// WithSystemPrompt sets the system prompt in the context.
func WithSystemPrompt(prompt string) ContextOption {
	return func(rc *RequestContext) {
		rc.SystemPrompt = prompt
	}
}

// WithTools sets the tools in the context.
func WithTools(tools []Tool) ContextOption {
	return func(rc *RequestContext) {
		rc.Tools = tools
	}
}

// WithResult sets the result in the context.
func WithResult(result any) ContextOption {
	return func(rc *RequestContext) {
		rc.Result = result
	}
}

// Override creates a new RequestContext with the given options applied.
// This follows an immutable pattern, leaving the original context unchanged.
func (rc *RequestContext) Override(opts ...ContextOption) *RequestContext {
	// Create a shallow copy
	newCtx := &RequestContext{
		ThreadID:              rc.ThreadID,
		Messages:              rc.Messages,
		SystemPrompt:          rc.SystemPrompt,
		Tools:                 rc.Tools,
		Config:                rc.Config,
		State:                 rc.State, // Share the same state map
		Result:                rc.Result,
		Terminate:             rc.Terminate,
		AdditionalTools:       rc.AdditionalTools,
		SystemPromptAdditions: rc.SystemPromptAdditions,
	}

	// Apply options
	for _, opt := range opts {
		opt(newCtx)
	}

	return newCtx
}

// MiddlewareFunc is an adapter that allows ordinary functions to act as Middleware.
// This enables a functional programming style for simple middleware.
//
// Example:
//
//	loggingFunc := func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
//	    log.Println("Processing request")
//	    return next(ctx, reqCtx)
//	}
//	middleware := MiddlewareFunc(loggingFunc)
type MiddlewareFunc func(ctx context.Context, reqCtx *RequestContext, next NextHandler) error

// Process implements the Middleware interface for MiddlewareFunc.
func (f MiddlewareFunc) Process(ctx context.Context, reqCtx *RequestContext, next NextHandler) error {
	return f(ctx, reqCtx, next)
}

// ChainMiddleware composes multiple middleware into a single handler chain.
// Middleware are executed in order: the first middleware in the slice is the
// outermost layer that wraps all others.
//
// The composition pattern is:
//   - First middleware wraps second middleware
//   - Second middleware wraps third middleware
//   - ...and so on until the base handler
//
// This allows middleware to run code before AND after the inner layers execute.
//
// Example:
//
//	handler := ChainMiddleware(
//	    []Middleware{authMiddleware, retryMiddleware, loggingMiddleware},
//	    baseHandler,
//	)
//	// Execution flow:
//	// authMiddleware (before) ->
//	//   retryMiddleware (before) ->
//	//     loggingMiddleware (before) ->
//	//       baseHandler ->
//	//     loggingMiddleware (after) ->
//	//   retryMiddleware (after) ->
//	// authMiddleware (after)
func ChainMiddleware(middlewares []Middleware, baseHandler NextHandler) NextHandler {
	if len(middlewares) == 0 {
		return baseHandler
	}

	// Build the chain from right to left (innermost to outermost)
	// This ensures the first middleware in the slice is the outermost wrapper
	// Safe: loop bounds validated by len > 0 check above and i >= 0 condition
	handler := baseHandler
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = wrapWithMiddleware(middlewares[i], handler)
	}

	return handler
}

// wrapWithMiddleware wraps a handler with a middleware, properly capturing variables to avoid closure issues.
func wrapWithMiddleware(middleware Middleware, next NextHandler) NextHandler {
	return func(ctx context.Context, reqCtx *RequestContext) error {
		// Check if we should terminate before executing this middleware
		if reqCtx.Terminate {
			return nil
		}

		return middleware.Process(ctx, reqCtx, next)
	}
}

// NoOpHandler is a NextHandler that does nothing.
// Useful for testing middleware in isolation.
func NoOpHandler(ctx context.Context, reqCtx *RequestContext) error {
	return nil
}

// ErrorHandler creates a NextHandler that always returns the given error.
// Useful for testing error handling in middleware.
func ErrorHandler(err error) NextHandler {
	return func(ctx context.Context, reqCtx *RequestContext) error {
		return err
	}
}

// ResultHandler creates a NextHandler that sets a result in the context.
// Useful for testing middleware that inspects results.
func ResultHandler(result any) NextHandler {
	return func(ctx context.Context, reqCtx *RequestContext) error {
		reqCtx.Result = result

		return nil
	}
}

// Verify MiddlewareFunc implements Middleware.
var _ Middleware = (*MiddlewareFunc)(nil)
