package engine

import (
	"context"
	"fmt"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
)

// Helper function to get minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LLMContext holds all data needed for an LLM call.
// This is the output of context preparation - ready to be passed to the chat client.
type LLMContext struct {
	Request    *llm.Request
	RequestCtx *core.RequestContext
}

// GetEffectiveTools returns the tools available after middleware processing.
func (c *LLMContext) GetEffectiveTools() []agent.ExecutableTool {
	return c.RequestCtx.GetEffectiveTools()
}

// LLMContextManager centralizes all LLM context management.
// It handles:
// - Conversation history retrieval and message conversion
// - Middleware application and chat options building
type LLMContextManager struct {
	config        core.AgentConfig
	reqCtxCapture **core.RequestContext // optional capture for testing
}

// NewLLMContextManager creates a new LLMContextManager.
func NewLLMContextManager(
	config core.AgentConfig,
	reqCtxCapture **core.RequestContext,
) *LLMContextManager {
	return &LLMContextManager{
		config:        config,
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
	// DEBUG: Log the initial messages
	for i, msg := range messages {
		if i >= 3 { // Only log first 3 messages to avoid spam
			break
		}
		contentPreview := msg.Content
		if len(contentPreview) > 50 {
			contentPreview = contentPreview[:50]
		}
	}

	// 1. Convert core messages to LLM format
	chatMessages := m.convertMessages(messages)

	// 3. Apply middleware and build LLM request
	llmReq, reqCtx, err := core.ToLLMRequestWithMiddleware(ctx, m.config, chatMessages, runCtx.ThreadID, runCtx.Options)
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
			Request:    llmReq,
			RequestCtx: reqCtx,
		},
		Terminated: false,
	}, nil
}

// convertMessages converts core messages to LLM chat messages.
func (m *LLMContextManager) convertMessages(msgs []core.Message) []llm.Message {
	return mapFromCoreToLLMMessages(msgs)
}
