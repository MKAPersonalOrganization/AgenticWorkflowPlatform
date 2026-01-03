package sdk

import (
	"context"
	"iter"

	"school-with-ai/services/llm"
)

// ChatClient defines the interface for LLM providers (OpenAI, Anthropic, Ollama, etc.)
// This mirrors Microsoft.Extensions.AI.IChatClient from the .NET agent framework.
//
// The interface is defined here (in the consumer package) following idiomatic Go patterns.
// Implementations live in the llm package.
type ChatClient interface {
	// GetStreamingResponse sends messages and returns a streaming response.
	// The iterator yields ChatResponseUpdate chunks as they arrive from the model.
	//
	// Use this for real-time streaming of content and tool calls.
	// Each update may contain partial text or partial tool call information.
	//
	// Options can be nil to use provider defaults.
	// The iterator should be fully consumed or the context cancelled to avoid resource leaks.
	GetStreamingResponse(
		ctx context.Context,
		messages []llm.ChatMessage,
		options *llm.ChatOptions,
	) iter.Seq2[llm.ChatResponseUpdate, error]
}
