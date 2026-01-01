package llm

import (
	"school-with-ai/domain"
)

// CompletionConfig contains configuration for a completion request
type CompletionConfig struct {
	Messages     []domain.Message
	Tools        []Tool
	SystemPrompt string // Optional system-level instructions
}

// StreamChunk represents a single chunk from the streaming response
type StreamChunk struct {
	Content   string
	ToolCalls []ToolCallDelta
}

// ToolCallDelta represents a streaming tool call delta
type ToolCallDelta struct {
	Index     int
	ID        string
	Name      string
	Arguments string
}

// Tool represents a tool/function that can be called by the LLM
type Tool struct {
	Type        string
	Name        string
	Description string
	Parameters  any // JSON schema for parameters
}
