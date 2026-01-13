package types

import (
	"context"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/llm"
)

// ChatClient defines the interface for LLM providers.
type ChatClient interface {
	GetStreamingResponse(
		ctx context.Context,
		messages []llm.ChatMessage,
		options *llm.ChatOptions,
	) iter.Seq2[llm.ChatResponseUpdate, error]
}

// ThreadStore provides read access to thread state.
type ThreadStore interface {
	GetAgentThread(threadID string) (core.AgentThread, bool)
	GetOrCreateAgentThread(ctx context.Context, threadID string) (core.AgentThread, error)
	Get(threadID string) (*domain.Thread, bool)
	Messages(threadID string) []domain.Message
	All() []domain.Thread
	Update(event core.Event) error
}

// ShouldContinueIterationFn determines if the agent should continue iterating after tool calls.
type ShouldContinueIterationFn func(toolCalls []domain.ToolCall) bool
