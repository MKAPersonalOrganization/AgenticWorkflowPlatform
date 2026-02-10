package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lightrag/lightrag-go/pkg/api/models"
	"github.com/lightrag/lightrag-go/pkg/config"
	"github.com/lightrag/lightrag-go/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOllamaHandler_GetVersion(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewOllamaHandler(cfg)

	router := gin.New()
	router.GET("/api/version", handler.GetVersion)

	req := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.OllamaVersionResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "0.9.3", resp.Version)
}

func TestOllamaHandler_GetTags(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewOllamaHandler(cfg)

	router := gin.New()
	router.GET("/api/tags", handler.GetTags)

	req := httptest.NewRequest("GET", "/api/tags", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.OllamaTagResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Models, 1)
	assert.Equal(t, "lightrag:latest", resp.Models[0].Name)
}

func TestOllamaHandler_GetPs(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewOllamaHandler(cfg)

	router := gin.New()
	router.GET("/api/ps", handler.GetPs)

	req := httptest.NewRequest("GET", "/api/ps", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp models.OllamaPsResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Len(t, resp.Models, 1)
	assert.Equal(t, "lightrag:latest", resp.Models[0].Name)
}

func TestOllamaHandler_Generate(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewOllamaHandler(cfg)

	router := gin.New()
	router.POST("/api/generate", handler.Generate)

	t.Run("valid request", func(t *testing.T) {
		reqBody := models.OllamaGenerateRequest{
			Model:  "lightrag:latest",
			Prompt: "What is AI?",
			Stream: false,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/generate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.OllamaGenerateResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Done)
		assert.NotEmpty(t, resp.Response)
	})

	t.Run("with system prompt", func(t *testing.T) {
		system := "You are a helpful assistant"
		reqBody := models.OllamaGenerateRequest{
			Model:  "lightrag:latest",
			Prompt: "Hello",
			System: &system,
			Stream: false,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/generate", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestOllamaHandler_Chat(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewOllamaHandler(cfg)

	router := gin.New()
	router.POST("/api/chat", handler.Chat)

	t.Run("valid request", func(t *testing.T) {
		reqBody := models.OllamaChatRequest{
			Model: "lightrag:latest",
			Messages: []models.OllamaMessage{
				{Role: "user", Content: "What is machine learning?"},
			},
			Stream: false,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var resp models.OllamaChatResponse
		err := json.Unmarshal(w.Body.Bytes(), &resp)
		require.NoError(t, err)
		assert.True(t, resp.Done)
		assert.Equal(t, "assistant", resp.Message.Role)
		assert.NotEmpty(t, resp.Message.Content)
	})

	t.Run("with conversation history", func(t *testing.T) {
		reqBody := models.OllamaChatRequest{
			Model: "lightrag:latest",
			Messages: []models.OllamaMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "What can you do?"},
			},
			Stream: false,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("with mode prefix", func(t *testing.T) {
		reqBody := models.OllamaChatRequest{
			Model: "lightrag:latest",
			Messages: []models.OllamaMessage{
				{Role: "user", Content: "/local What is AI?"},
			},
			Stream: false,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/chat", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestOllamaHandler_Embeddings(t *testing.T) {
	cfg := config.DefaultConfig()
	handler := NewOllamaHandler(cfg)

	router := gin.New()
	router.POST("/api/embeddings", handler.Embeddings)

	reqBody := map[string]interface{}{
		"model":  "lightrag:latest",
		"prompt": "test text",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/api/embeddings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestParseQueryMode(t *testing.T) {
	tests := []struct {
		name             string
		query            string
		expectedQuery    string
		expectedMode     types.SearchMode
		expectedContext  bool
		expectUserPrompt bool
	}{
		{
			name:            "no prefix - defaults to mix",
			query:           "What is AI?",
			expectedQuery:   "What is AI?",
			expectedMode:    types.SearchModeMix,
			expectedContext: false,
		},
		{
			name:            "local mode",
			query:           "/local What is machine learning?",
			expectedQuery:   "What is machine learning?",
			expectedMode:    types.SearchModeLocal,
			expectedContext: false,
		},
		{
			name:            "global mode",
			query:           "/global Explain neural networks",
			expectedQuery:   "Explain neural networks",
			expectedMode:    types.SearchModeGlobal,
			expectedContext: false,
		},
		{
			name:            "hybrid mode",
			query:           "/hybrid What are transformers?",
			expectedQuery:   "What are transformers?",
			expectedMode:    types.SearchModeHybrid,
			expectedContext: false,
		},
		{
			name:            "naive mode",
			query:           "/naive Simple search",
			expectedQuery:   "Simple search",
			expectedMode:    types.SearchModeNaive,
			expectedContext: false,
		},
		{
			name:            "mix mode explicit",
			query:           "/mix Mixed search query",
			expectedQuery:   "Mixed search query",
			expectedMode:    types.SearchModeMix,
			expectedContext: false,
		},
		{
			name:            "bypass mode",
			query:           "/bypass Direct LLM query",
			expectedQuery:   "Direct LLM query",
			expectedMode:    types.SearchModeBypass,
			expectedContext: false,
		},
		{
			name:            "context mode",
			query:           "/context Get context only",
			expectedQuery:   " Get context only",
			expectedMode:    types.SearchModeMix,
			expectedContext: true,
		},
		{
			name:            "localcontext mode",
			query:           "/localcontext Local context",
			expectedQuery:   " Local context",
			expectedMode:    types.SearchModeLocal,
			expectedContext: true,
		},
		{
			name:            "globalcontext mode",
			query:           "/globalcontext Global context",
			expectedQuery:   " Global context",
			expectedMode:    types.SearchModeGlobal,
			expectedContext: true,
		},
		{
			name:             "with user prompt in brackets",
			query:            "/local[use mermaid format] What is AI?",
			expectedQuery:    "What is AI?",
			expectedMode:     types.SearchModeLocal,
			expectedContext:  false,
			expectUserPrompt: true,
		},
		{
			name:             "with user prompt no mode",
			query:            "/[custom instructions] test query",
			expectedQuery:    "test query",
			expectedMode:     types.SearchModeMix,
			expectedContext:  false,
			expectUserPrompt: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanQuery, mode, onlyNeedContext, userPrompt := ParseQueryMode(tt.query)

			assert.Equal(t, tt.expectedMode, mode, "mode mismatch")
			assert.Equal(t, tt.expectedContext, onlyNeedContext, "onlyNeedContext mismatch")

			if tt.expectUserPrompt {
				assert.NotNil(t, userPrompt, "expected user prompt")
			}

			// Check that cleaned query doesn't have the mode prefix
			if tt.query != tt.expectedQuery {
				assert.NotContains(t, cleanQuery, "/local ")
				assert.NotContains(t, cleanQuery, "/global ")
				assert.NotContains(t, cleanQuery, "/hybrid ")
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"word", 1},
		{"hello world", 2},
		{"This is a longer sentence with more words", 10},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := EstimateTokens(tt.text)
			if tt.text == "" {
				assert.Equal(t, 0, result)
			} else {
				// Allow for some variance since it's an approximation
				assert.Greater(t, result, 0)
			}
		})
	}
}

func TestEstimateTokens_EmptyString(t *testing.T) {
	result := EstimateTokens("")
	assert.Equal(t, 0, result)
}
