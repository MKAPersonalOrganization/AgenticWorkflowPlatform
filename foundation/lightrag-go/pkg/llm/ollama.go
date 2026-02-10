// Package llm provides Ollama LLM client implementation.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// OllamaClient implements LLMClient for Ollama API.
type OllamaClient struct {
	config     *LLMConfig
	httpClient *http.Client
	logger     *logrus.Logger
}

// NewOllamaClient creates a new Ollama client.
func NewOllamaClient(config *LLMConfig, logger *logrus.Logger) *OllamaClient {
	if logger == nil {
		logger = logrus.New()
	}
	if config.BaseURL == "" {
		config.BaseURL = "http://localhost:11434"
	}
	if config.Model == "" {
		config.Model = "llama3.2"
	}
	if config.EmbeddingModel == "" {
		config.EmbeddingModel = "nomic-embed-text"
	}
	if config.Timeout == 0 {
		config.Timeout = 300 // Ollama can be slower
	}

	return &OllamaClient{
		config: config,
		httpClient: &http.Client{
			Timeout: time.Duration(config.Timeout) * time.Second,
		},
		logger: logger,
	}
}

// ollamaChatRequest represents the Ollama chat request format.
type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
	Format   string          `json:"format,omitempty"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
}

// ollamaChatResponse represents the Ollama chat response format.
type ollamaChatResponse struct {
	Model           string        `json:"model"`
	CreatedAt       string        `json:"created_at"`
	Message         ollamaMessage `json:"message"`
	Done            bool          `json:"done"`
	TotalDuration   int64         `json:"total_duration,omitempty"`
	LoadDuration    int64         `json:"load_duration,omitempty"`
	PromptEvalCount int           `json:"prompt_eval_count,omitempty"`
	EvalCount       int           `json:"eval_count,omitempty"`
}

// ollamaEmbeddingRequest represents the Ollama embedding request format.
type ollamaEmbeddingRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// ollamaEmbeddingResponse represents the Ollama embedding response format.
type ollamaEmbeddingResponse struct {
	Embedding []float64 `json:"embedding"`
}

// ollamaEmbedRequest represents the new Ollama embed API request format.
type ollamaEmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// ollamaEmbedResponse represents the new Ollama embed API response format.
type ollamaEmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float64 `json:"embeddings"`
}

// Complete generates a completion for the given request.
func (c *OllamaClient) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Convert messages
	messages := make([]ollamaMessage, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = ollamaMessage{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	model := req.Model
	if model == "" {
		model = c.config.Model
	}

	temperature := req.Temperature
	if temperature == 0 {
		temperature = c.config.Temperature
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.config.MaxTokens
	}

	ollamaReq := ollamaChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   false,
		Options: &ollamaOptions{
			Temperature: temperature,
			NumPredict:  maxTokens,
			TopP:        req.TopP,
		},
	}

	// Set format for JSON response if requested
	if req.ResponseFormat != nil {
		if _, ok := req.ResponseFormat["type"]; ok {
			ollamaReq.Format = "json"
		}
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaChatResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	finishReason := "stop"
	if !ollamaResp.Done {
		finishReason = "length"
	}

	return &CompletionResponse{
		Content:      ollamaResp.Message.Content,
		FinishReason: finishReason,
		Model:        ollamaResp.Model,
		Usage: &UsageInfo{
			PromptTokens:     ollamaResp.PromptEvalCount,
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.PromptEvalCount + ollamaResp.EvalCount,
		},
	}, nil
}

// CompleteStream generates a streaming completion.
func (c *OllamaClient) CompleteStream(ctx context.Context, req *CompletionRequest) (<-chan *CompletionResponse, <-chan error) {
	respChan := make(chan *CompletionResponse)
	errChan := make(chan error, 1)

	go func() {
		defer close(respChan)
		defer close(errChan)

		// For simplicity, use non-streaming and return at once
		resp, err := c.Complete(ctx, req)
		if err != nil {
			errChan <- err
			return
		}
		respChan <- resp
	}()

	return respChan, errChan
}

// Embed generates embeddings for the given texts.
func (c *OllamaClient) Embed(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResponse, error) {
	model := req.Model
	if model == "" {
		model = c.config.EmbeddingModel
	}

	// Try the new /api/embed endpoint first (Ollama 0.4+)
	embeddings, err := c.embedBatch(ctx, model, req.Input)
	if err != nil {
		// Fall back to legacy /api/embeddings endpoint
		embeddings, err = c.embedLegacy(ctx, model, req.Input)
		if err != nil {
			return nil, err
		}
	}

	return &EmbeddingResponse{
		Embeddings: embeddings,
		Model:      model,
	}, nil
}

// embedBatch uses the new /api/embed endpoint.
func (c *OllamaClient) embedBatch(ctx context.Context, model string, input []string) ([][]float64, error) {
	ollamaReq := ollamaEmbedRequest{
		Model: model,
		Input: input,
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var ollamaResp ollamaEmbedResponse
	if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return ollamaResp.Embeddings, nil
}

// embedLegacy uses the legacy /api/embeddings endpoint for older Ollama versions.
func (c *OllamaClient) embedLegacy(ctx context.Context, model string, input []string) ([][]float64, error) {
	embeddings := make([][]float64, len(input))

	for i, text := range input {
		ollamaReq := ollamaEmbeddingRequest{
			Model:  model,
			Prompt: text,
		}

		body, err := json.Marshal(ollamaReq)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		httpReq, err := http.NewRequestWithContext(ctx, "POST", c.config.BaseURL+"/api/embeddings", bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		httpReq.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(httpReq)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Ollama API error (%d): %s", resp.StatusCode, string(respBody))
		}

		var ollamaResp ollamaEmbeddingResponse
		if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
			return nil, fmt.Errorf("failed to parse response: %w", err)
		}

		embeddings[i] = ollamaResp.Embedding
	}

	return embeddings, nil
}

// ModelName returns the model name.
func (c *OllamaClient) ModelName() string {
	return c.config.Model
}

// Close closes the client.
func (c *OllamaClient) Close() error {
	c.httpClient.CloseIdleConnections()
	return nil
}

// ListModels lists available models on the Ollama server.
func (c *OllamaClient) ListModels(ctx context.Context) ([]string, error) {
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.config.BaseURL+"/api/tags", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Ollama API error (%d): %s", resp.StatusCode, string(respBody))
	}

	var tagsResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(respBody, &tagsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	models := make([]string, len(tagsResp.Models))
	for i, m := range tagsResp.Models {
		models[i] = m.Name
	}

	return models, nil
}
