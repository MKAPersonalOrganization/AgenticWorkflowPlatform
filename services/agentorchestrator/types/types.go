package types

import (
	"context"
	"iter"
	"school-with-ai/foundation/agentsdk/llm"
)

// ChatClient defines the interface for LLM providers.
type ChatClient interface {
	GetStreamingResponse(
		ctx context.Context,
		messages []llm.ChatMessage,
		options *llm.ChatOptions,
	) iter.Seq2[llm.ChatResponseUpdate, error]
}
