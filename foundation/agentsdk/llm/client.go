package llm

import (
	"context"
	"iter"
	"strings"
)

// LLM represents any language model backend.
// This is the minimal interface - all provider-specific features
// are accessed via type assertions on the implementation.
type LLM interface {
	// Generate produces a response for the given request.
	// The iterator yields streaming chunks if supported.
	Generate(ctx context.Context, req *Request) iter.Seq2[*Response, error]
}

// Request contains all parameters for a model call.
// Designed to be provider-agnostic.
type Request struct {
	// Messages is the conversation history.
	Messages []Message

	// System is the system prompt/instructions.
	System string

	// Tools available for this request.
	Tools []ToolDeclaration

	// Parameters for generation (temperature, etc.)
	Parameters *Parameters

	// ResponseFormat for structured output.
	ResponseFormat *ResponseFormat

	// Extensions for provider-specific features.
	Extensions Metadata
}

// Parameters controls generation behavior.
type Parameters struct {
	MaxTokens        int
	Temperature      *float64 // Pointer allows distinguishing unset from 0
	TopP             *float64
	TopK             *int
	StopSequences    []string
	PresencePenalty  *float64
	FrequencyPenalty *float64
}

// Response contains the model's output.
type Response struct {
	// Content is the generated content.
	Content []Content

	// FinishReason indicates why generation stopped.
	FinishReason FinishReason

	// Usage tracks token consumption.
	Usage *Usage

	// Partial indicates this is a streaming chunk.
	Partial bool

	// Extensions for provider-specific data.
	Extensions Metadata
}

// ToolDeclaration describes a tool for the model.
type ToolDeclaration struct {
	Name        string
	Description string
	Parameters  *Schema
}

// ResponseFormat specifies structured output requirements.
type ResponseFormat struct {
	Type   ResponseFormatType
	Schema *Schema // For JSON mode
}

type ResponseFormatType string

const (
	ResponseFormatText ResponseFormatType = "text"
	ResponseFormatJSON ResponseFormatType = "json"
)

// Text returns the concatenated text content of the response.
func (r *Response) Text() string {
	var sb strings.Builder
	for _, c := range r.Content {
		if tc, ok := c.(TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}
	return sb.String()
}

// Usage tracks token consumption.
type Usage struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int

	// Detailed breakdown (Framework specific, kept for internal use)
	CachedTokens             int
	AudioInputTokens         int
	ReasoningTokens          int
	AudioOutputTokens        int
	AcceptedPredictionTokens int
	RejectedPredictionTokens int
}

// FinishReason indicates why the model stopped generating.
type FinishReason string

const (
	FinishReasonStop          FinishReason = "stop"
	FinishReasonLength        FinishReason = "length"
	FinishReasonToolUse       FinishReason = "tool_use"
	FinishReasonContentFilter FinishReason = "content_filter"
)

// FunctionCalls returns all FunctionCall items in the response.
func (r *Response) FunctionCalls() []FunctionCall {
	var calls []FunctionCall
	for _, c := range r.Content {
		if fc, ok := c.(FunctionCall); ok {
			calls = append(calls, fc)
		}
	}
	return calls
}
