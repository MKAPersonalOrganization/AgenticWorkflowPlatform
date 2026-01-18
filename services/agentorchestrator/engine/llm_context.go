package engine

import (
	"context"
	"fmt"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/services/threadmgmt"
)

// LLMContext holds all data needed for an LLM call.
// This is the output of context preparation - ready to be passed to the chat client.
type LLMContext struct {
	Messages   []llm.ChatMessage
	Options    *llm.ChatOptions
	RequestCtx *core.RequestContext
}

// GetEffectiveTools returns the tools available after middleware processing.
func (c *LLMContext) GetEffectiveTools() []core.Tool {
	return c.RequestCtx.GetEffectiveTools()
}

// LLMContextManager centralizes all LLM context management.
// It handles:
// - Conversation history retrieval and message conversion
// - Middleware application and chat options building
// - Thread persistence (adding assistant messages and tool results)
type LLMContextManager struct {
	config        core.AgentConfig
	threadMgmt    *threadmgmt.Manager
	reqCtxCapture **core.RequestContext // optional capture for testing
}

// NewLLMContextManager creates a new LLMContextManager.
func NewLLMContextManager(
	config core.AgentConfig,
	threadMgmt *threadmgmt.Manager,
	reqCtxCapture **core.RequestContext,
) *LLMContextManager {
	return &LLMContextManager{
		config:        config,
		threadMgmt:    threadMgmt,
		reqCtxCapture: reqCtxCapture,
	}
}

// PrepareResult represents the result of context preparation.
type PrepareResult struct {
	Context    *LLMContext
	Terminated bool // true if middleware terminated early
}

// Prepare converts messages to LLM format, applies middleware, and returns the complete context needed for an LLM call.
//
// Returns PrepareResult with Terminated=true if middleware signals early termination.
func (m *LLMContextManager) Prepare(ctx context.Context, runCtx core.RunContext, messages []core.Message) (*PrepareResult, error) {
	// 1. Convert core messages to LLM format
	chatMessages := m.convertMessages(messages)

	// 3. Apply middleware and build chat options
	chatOptions, reqCtx, err := core.ToChatOptionsWithMiddleware(ctx, m.config, chatMessages, runCtx.ThreadID, runCtx.Options)
	if err != nil {
		return nil, fmt.Errorf("failed to apply middleware: %w", err)
	}

	// 4. Capture request context for testing if configured
	if m.reqCtxCapture != nil {
		*m.reqCtxCapture = reqCtx
	}

	// 5. Check for middleware termination
	if reqCtx.Terminate {
		return &PrepareResult{Terminated: true}, nil
	}

	return &PrepareResult{
		Context: &LLMContext{
			Messages:   chatMessages,
			Options:    chatOptions,
			RequestCtx: reqCtx,
		},
		Terminated: false,
	}, nil
}

// convertMessages converts core messages to LLM chat messages.
func (m *LLMContextManager) convertMessages(msgs []core.Message) []llm.ChatMessage {
	return mapFromCoreToLLMMessages(msgs)
}
