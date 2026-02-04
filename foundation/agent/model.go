package agent

import (
	"context"
	"iter"
)

// Model represents an LLM (Large Language Model) client.
// This interface abstracts the underlying LLM provider (OpenAI, Gemini, Ollama, etc.).
type Model interface {
	// Generate produces a response to the given messages.
	// Returns an iterator that yields response chunks as they arrive.
	Generate(context.Context, *GenerateRequest) iter.Seq2[*GenerateResponse, error]

	// Provider returns the model provider name (e.g., "openai", "gemini").
	Provider() string
}

// GenerateRequest contains all parameters for an LLM request.
type GenerateRequest struct {
	// Messages is the conversation history.
	Messages []*Message

	// Tools available for function calling.
	Tools []ToolSchema

	// SystemPrompt optionally overrides system instructions.
	SystemPrompt string

	// Temperature controls randomness (0.0-2.0).
	Temperature float64

	// MaxTokens limits output length.
	MaxTokens int

	// TopP controls nucleus sampling (0.0-1.0).
	TopP float64

	// StopSequences are strings that stop generation.
	StopSequences []string

	// ModelID optionally overrides the default model.
	ModelID string
}

// GenerateResponse represents a single chunk of LLM output.
type GenerateResponse struct {
	// Content is the generated text or tool calls.
	Content []Content

	// FinishReason indicates why generation stopped.
	FinishReason FinishReason

	// Usage contains token consumption metrics.
	Usage *Usage

	// ModelID is the actual model used.
	ModelID string
}

// FinishReason indicates why the model stopped generating.
type FinishReason string

const (
	FinishReasonStop      FinishReason = "stop"       // Natural completion
	FinishReasonLength    FinishReason = "length"     // Max tokens reached
	FinishReasonToolCalls FinishReason = "tool_calls" // Tool calling required
	FinishReasonError     FinishReason = "error"      // Generation error
	FinishReasonCancelled FinishReason = "cancelled"  // User cancelled
	FinishReasonHandoff   FinishReason = "handoff"    // Agent transfer
)

// Usage tracks token consumption.
type Usage struct {
	// PromptTokens is the number of input tokens.
	PromptTokens int

	// CompletionTokens is the number of output tokens.
	CompletionTokens int

	// TotalTokens is the sum of prompt and completion tokens.
	TotalTokens int
}

// ToolSchema describes a tool for function calling.
type ToolSchema struct {
	// Name is the function name.
	Name string

	// Description explains what the tool does.
	Description string

	// Parameters is the JSON schema for tool input.
	Parameters map[string]any
}
