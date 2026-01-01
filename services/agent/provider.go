package agent

import (
	"context"
	"iter"

	"school-with-ai/services/llm"
)

// Provider defines the interface for LLM providers (OpenAI, Anthropic, etc.)
// This interface is defined here (consumer) following idiomatic Go patterns.
// Implementations live in the llm package.
type Provider interface {
	// CreateStreamingCompletion creates a streaming chat completion
	// Returns an iterator that yields chunks and errors
	CreateStreamingCompletion(
		ctx context.Context,
		config llm.CompletionConfig,
	) iter.Seq2[llm.StreamChunk, error]
}
