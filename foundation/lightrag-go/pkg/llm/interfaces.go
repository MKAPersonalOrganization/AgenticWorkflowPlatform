// Package llm provides LLM client interfaces and implementations.
package llm

import (
	"context"
)

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"` // "system", "user", "assistant"
	Content string `json:"content"`
}

// CompletionRequest represents a completion request.
type CompletionRequest struct {
	Messages       []Message         `json:"messages"`
	Model          string            `json:"model,omitempty"`
	Temperature    float64           `json:"temperature,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	TopP           float64           `json:"top_p,omitempty"`
	Stream         bool              `json:"stream,omitempty"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

// CompletionResponse represents a completion response.
type CompletionResponse struct {
	Content      string            `json:"content"`
	FinishReason string            `json:"finish_reason"`
	Usage        *UsageInfo        `json:"usage,omitempty"`
	Model        string            `json:"model,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// UsageInfo represents token usage information.
type UsageInfo struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// EmbeddingRequest represents an embedding request.
type EmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model,omitempty"`
}

// EmbeddingResponse represents an embedding response.
type EmbeddingResponse struct {
	Embeddings [][]float64 `json:"embeddings"`
	Model      string      `json:"model,omitempty"`
	Usage      *UsageInfo  `json:"usage,omitempty"`
}

// LLMClient is the interface for LLM operations.
type LLMClient interface {
	// Complete generates a completion for the given request.
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// CompleteStream generates a streaming completion.
	CompleteStream(ctx context.Context, req *CompletionRequest) (<-chan *CompletionResponse, <-chan error)

	// Embed generates embeddings for the given texts.
	Embed(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error)

	// ModelName returns the model name.
	ModelName() string

	// Close closes the client and releases resources.
	Close() error
}

// LLMConfig holds common configuration for LLM clients.
type LLMConfig struct {
	APIKey         string
	BaseURL        string
	Model          string
	EmbeddingModel string
	MaxRetries     int
	Timeout        int // seconds
	Temperature    float64
	MaxTokens      int
	TopP           float64
	EmbeddingDim   int
	OrganizationID string
	Headers        map[string]string
}

// DefaultLLMConfig returns a default LLM configuration.
func DefaultLLMConfig() *LLMConfig {
	return &LLMConfig{
		MaxRetries:  3,
		Timeout:     120,
		Temperature: 0.7,
		MaxTokens:   4096,
		TopP:        1.0,
	}
}

// LLMFunc is a function type for generating completions.
type LLMFunc func(ctx context.Context, messages []Message) (string, error)

// EmbeddingFunc is a function type for generating embeddings.
type EmbeddingFunc func(ctx context.Context, texts []string) ([][]float64, error)

// NewLLMFunc creates an LLMFunc from an LLMClient.
func NewLLMFunc(client LLMClient) LLMFunc {
	return func(ctx context.Context, messages []Message) (string, error) {
		req := &CompletionRequest{
			Messages: messages,
		}
		resp, err := client.Complete(ctx, req)
		if err != nil {
			return "", err
		}
		return resp.Content, nil
	}
}

// NewEmbeddingFunc creates an EmbeddingFunc from an LLMClient.
func NewEmbeddingFunc(client LLMClient) EmbeddingFunc {
	return func(ctx context.Context, texts []string) ([][]float64, error) {
		req := &EmbeddingRequest{
			Input: texts,
		}
		resp, err := client.Embed(ctx, req)
		if err != nil {
			return nil, err
		}
		return resp.Embeddings, nil
	}
}
