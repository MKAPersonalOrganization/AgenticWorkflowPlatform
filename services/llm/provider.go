package llm

import (
	"context"
	"iter"

	"school-with-ai/internal/session"
)

// Provider defines the interface for LLM providers (OpenAI, Anthropic, etc.)
type Provider interface {
	// CreateStreamingCompletion creates a streaming chat completion
	// Returns an iterator that yields chunks and errors
	CreateStreamingCompletion(
		ctx context.Context,
		config CompletionConfig,
	) iter.Seq2[StreamChunk, error]
}

// CompletionConfig contains configuration for a completion request
type CompletionConfig struct {
	Messages []session.Message
	Tools    []Tool
}

// StreamChunk represents a single chunk from the streaming response
type StreamChunk struct {
	Content   string
	ToolCalls []ToolCallDelta
}

// ToolCallDelta represents a streaming tool call delta
type ToolCallDelta struct {
	Index    int
	ID       string
	Name     string
	Arguments string
}

// Tool represents a tool/function that can be called by the LLM
type Tool struct {
	Type        string
	Name        string
	Description string
	Parameters  any // JSON schema for parameters
}
