package llm

import (
	"context"
	"iter"
)

// ChatClient defines the interface for LLM providers.
// This interface abstracts the underlying LLM implementation, allowing
// the agent framework to work with different providers (OpenAI, Gemini, Ollama, etc.).
type ChatClient interface {
	// GetStreamingResponse sends a chat request and returns a streaming response.
	// The returned iterator yields ChatResponseUpdate chunks as they arrive.
	GetStreamingResponse(
		ctx context.Context,
		messages []ChatMessage,
		options *ChatOptions,
	) iter.Seq2[ChatResponseUpdate, error]

	// Provider returns the name of the LLM provider (e.g., "openai", "gemini", "ollama").
	Provider() string
}
