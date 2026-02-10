package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessage(t *testing.T) {
	msg := Message{
		Role:    "user",
		Content: "Hello!",
	}
	assert.Equal(t, "user", msg.Role)
	assert.Equal(t, "Hello!", msg.Content)
}

func TestCompletionRequest(t *testing.T) {
	req := CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
		Model:       "gpt-4",
		Temperature: 0.7,
		MaxTokens:   100,
	}
	assert.Len(t, req.Messages, 1)
	assert.Equal(t, "gpt-4", req.Model)
}

func TestCompletionResponse(t *testing.T) {
	resp := CompletionResponse{
		Content:      "Hello there!",
		FinishReason: "stop",
		Model:        "gpt-4",
		Usage: &UsageInfo{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}
	assert.Equal(t, "Hello there!", resp.Content)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestDefaultLLMConfig(t *testing.T) {
	config := DefaultLLMConfig()
	assert.Equal(t, 3, config.MaxRetries)
	assert.Equal(t, 120, config.Timeout)
	assert.Equal(t, 0.7, config.Temperature)
	assert.Equal(t, 4096, config.MaxTokens)
}

func TestNewOpenAIClient(t *testing.T) {
	config := &LLMConfig{
		APIKey: "test-key",
	}

	client := NewOpenAIClient(config, nil)

	assert.NotNil(t, client)
	assert.Equal(t, "https://api.openai.com/v1", client.config.BaseURL)
	assert.Equal(t, "gpt-4o-mini", client.config.Model)
	assert.Equal(t, "text-embedding-3-small", client.config.EmbeddingModel)
}

func TestOpenAIClient_Complete(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		resp := openAIChatResponse{
			ID:    "test-id",
			Model: "gpt-4",
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{
					Index:        0,
					Message:      openAIMessage{Role: "assistant", Content: "Hello!"},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     5,
				CompletionTokens: 3,
				TotalTokens:      8,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &LLMConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	client := NewOpenAIClient(config, nil)

	req := &CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
	}

	resp, err := client.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "Hello!", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 8, resp.Usage.TotalTokens)
}

func TestOpenAIClient_Complete_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		resp := openAIErrorResponse{
			Error: struct {
				Message string `json:"message"`
				Type    string `json:"type"`
				Code    string `json:"code"`
			}{
				Message: "Invalid API key",
				Type:    "invalid_request_error",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &LLMConfig{
		APIKey:  "invalid-key",
		BaseURL: server.URL,
	}

	client := NewOpenAIClient(config, nil)

	req := &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}

	_, err := client.Complete(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Invalid API key")
}

func TestOpenAIClient_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/embeddings", r.URL.Path)

		resp := openAIEmbeddingResponse{
			Object: "list",
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Object: "embedding", Embedding: []float64{0.1, 0.2, 0.3}, Index: 0},
				{Object: "embedding", Embedding: []float64{0.4, 0.5, 0.6}, Index: 1},
			},
			Model: "text-embedding-3-small",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &LLMConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	client := NewOpenAIClient(config, nil)

	req := &EmbeddingRequest{
		Input: []string{"hello", "world"},
	}

	resp, err := client.Embed(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, resp.Embeddings, 2)
	assert.Equal(t, []float64{0.1, 0.2, 0.3}, resp.Embeddings[0])
}

func TestOpenAIClient_ModelName(t *testing.T) {
	config := &LLMConfig{
		Model: "gpt-4-turbo",
	}

	client := NewOpenAIClient(config, nil)
	assert.Equal(t, "gpt-4-turbo", client.ModelName())
}

func TestOpenAIClient_CompleteStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIChatResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{Message: openAIMessage{Content: "Streamed!"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &LLMConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	client := NewOpenAIClient(config, nil)

	req := &CompletionRequest{
		Messages: []Message{{Role: "user", Content: "Hi"}},
		Stream:   true,
	}

	respChan, errChan := client.CompleteStream(context.Background(), req)

	select {
	case resp := <-respChan:
		assert.Equal(t, "Streamed!", resp.Content)
	case err := <-errChan:
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewOllamaClient(t *testing.T) {
	config := &LLMConfig{}

	client := NewOllamaClient(config, nil)

	assert.NotNil(t, client)
	assert.Equal(t, "http://localhost:11434", client.config.BaseURL)
	assert.Equal(t, "llama3.2", client.config.Model)
	assert.Equal(t, "nomic-embed-text", client.config.EmbeddingModel)
}

func TestOllamaClient_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/chat", r.URL.Path)

		resp := ollamaChatResponse{
			Model: "llama3.2",
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "Hello from Ollama!",
			},
			Done:            true,
			PromptEvalCount: 10,
			EvalCount:       5,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &LLMConfig{
		BaseURL: server.URL,
	}

	logger := logrus.New()
	client := NewOllamaClient(config, logger)

	req := &CompletionRequest{
		Messages: []Message{
			{Role: "user", Content: "Hi"},
		},
	}

	resp, err := client.Complete(context.Background(), req)
	require.NoError(t, err)
	assert.Equal(t, "Hello from Ollama!", resp.Content)
	assert.Equal(t, "stop", resp.FinishReason)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestOllamaClient_Embed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/embed" {
			resp := ollamaEmbedResponse{
				Model:      "nomic-embed-text",
				Embeddings: [][]float64{{0.1, 0.2}, {0.3, 0.4}},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &LLMConfig{
		BaseURL: server.URL,
	}

	client := NewOllamaClient(config, nil)

	req := &EmbeddingRequest{
		Input: []string{"hello", "world"},
	}

	resp, err := client.Embed(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, resp.Embeddings, 2)
}

func TestOllamaClient_EmbedLegacy(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/embed" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/api/embeddings" {
			callCount++
			resp := ollamaEmbeddingResponse{
				Embedding: []float64{0.1, 0.2, 0.3},
			}
			json.NewEncoder(w).Encode(resp)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	config := &LLMConfig{
		BaseURL: server.URL,
	}

	client := NewOllamaClient(config, nil)

	req := &EmbeddingRequest{
		Input: []string{"hello", "world"},
	}

	resp, err := client.Embed(context.Background(), req)
	require.NoError(t, err)
	assert.Len(t, resp.Embeddings, 2)
	assert.Equal(t, 2, callCount)
}

func TestOllamaClient_ListModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/tags", r.URL.Path)

		resp := struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}{
			Models: []struct {
				Name string `json:"name"`
			}{
				{Name: "llama3.2:latest"},
				{Name: "mistral:latest"},
			},
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	config := &LLMConfig{
		BaseURL: server.URL,
	}

	client := NewOllamaClient(config, nil)

	models, err := client.ListModels(context.Background())
	require.NoError(t, err)
	assert.Len(t, models, 2)
	assert.Contains(t, models, "llama3.2:latest")
}

func TestNewLLMFunc(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIChatResponse{
			Choices: []struct {
				Index        int           `json:"index"`
				Message      openAIMessage `json:"message"`
				FinishReason string        `json:"finish_reason"`
			}{
				{Message: openAIMessage{Content: "Response"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(&LLMConfig{
		APIKey:  "test",
		BaseURL: server.URL,
	}, nil)

	llmFunc := NewLLMFunc(client)

	result, err := llmFunc(context.Background(), []Message{{Role: "user", Content: "Hi"}})
	require.NoError(t, err)
	assert.Equal(t, "Response", result)
}

func TestNewEmbeddingFunc(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openAIEmbeddingResponse{
			Data: []struct {
				Object    string    `json:"object"`
				Embedding []float64 `json:"embedding"`
				Index     int       `json:"index"`
			}{
				{Embedding: []float64{0.1, 0.2}, Index: 0},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewOpenAIClient(&LLMConfig{
		APIKey:  "test",
		BaseURL: server.URL,
	}, nil)

	embedFunc := NewEmbeddingFunc(client)

	result, err := embedFunc(context.Background(), []string{"hello"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, []float64{0.1, 0.2}, result[0])
}
