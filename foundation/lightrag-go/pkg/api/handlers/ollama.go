// Package handlers provides HTTP handlers for the LightRAG API.
package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lightrag/lightrag-go/pkg/api/models"
	"github.com/lightrag/lightrag-go/pkg/config"
	"github.com/lightrag/lightrag-go/pkg/types"
)

// OllamaHandler handles Ollama-compatible API requests.
type OllamaHandler struct {
	cfg         *config.Config
	serverInfos *config.OllamaServerInfos
	topK        int
}

// NewOllamaHandler creates a new OllamaHandler.
func NewOllamaHandler(cfg *config.Config) *OllamaHandler {
	return &OllamaHandler{
		cfg:         cfg,
		serverInfos: config.NewOllamaServerInfos(cfg.SimulatedModelName, cfg.SimulatedModelTag),
		topK:        cfg.TopK,
	}
}

// GetVersion returns Ollama version information.
func (h *OllamaHandler) GetVersion(c *gin.Context) {
	c.JSON(http.StatusOK, models.OllamaVersionResponse{
		Version: "0.9.3",
	})
}

// GetTags returns available models.
func (h *OllamaHandler) GetTags(c *gin.Context) {
	c.JSON(http.StatusOK, models.OllamaTagResponse{
		Models: []models.OllamaModel{
			{
				Name:       h.serverInfos.LightRAGModel(),
				Model:      h.serverInfos.LightRAGModel(),
				ModifiedAt: h.serverInfos.LightRAGCreatedAt,
				Size:       h.serverInfos.LightRAGSize,
				Digest:     h.serverInfos.LightRAGDigest,
				Details: models.OllamaModelDetails{
					ParentModel:       "",
					Format:            "gguf",
					Family:            h.serverInfos.LightRAGName,
					Families:          []string{h.serverInfos.LightRAGName},
					ParameterSize:     "13B",
					QuantizationLevel: "Q4_0",
				},
			},
		},
	})
}

// GetPs returns running models.
func (h *OllamaHandler) GetPs(c *gin.Context) {
	c.JSON(http.StatusOK, models.OllamaPsResponse{
		Models: []models.OllamaRunningModel{
			{
				Name:   h.serverInfos.LightRAGModel(),
				Model:  h.serverInfos.LightRAGModel(),
				Size:   h.serverInfos.LightRAGSize,
				Digest: h.serverInfos.LightRAGDigest,
				Details: models.OllamaModelDetails{
					ParentModel:       "",
					Format:            "gguf",
					Family:            "llama",
					Families:          []string{"llama"},
					ParameterSize:     "7.2B",
					QuantizationLevel: "Q4_0",
				},
				ExpiresAt: "2050-12-31T14:38:31.83753-07:00",
				SizeVRAM:  h.serverInfos.LightRAGSize,
			},
		},
	})
}

// Generate handles generate completion requests.
func (h *OllamaHandler) Generate(c *gin.Context) {
	var req models.OllamaGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	// Mock response for now
	resp := models.OllamaGenerateResponse{
		Model:     h.serverInfos.LightRAGModel(),
		CreatedAt: h.serverInfos.LightRAGCreatedAt,
		Response:  "This is a mock generate response for: " + req.Prompt,
		Done:      true,
	}

	c.JSON(http.StatusOK, resp)
}

// Chat handles chat completion requests.
func (h *OllamaHandler) Chat(c *gin.Context) {
	var req models.OllamaChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Detail: "Invalid request: " + err.Error(),
		})
		return
	}

	// Get the last user message
	var lastUserMessage string
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			lastUserMessage = req.Messages[i].Content
			break
		}
	}

	// Parse query mode from the message
	cleanQuery, mode, onlyNeedContext, userPrompt := ParseQueryMode(lastUserMessage)

	_ = cleanQuery
	_ = mode
	_ = onlyNeedContext
	_ = userPrompt

	// Mock response
	resp := models.OllamaChatResponse{
		Model:     h.serverInfos.LightRAGModel(),
		CreatedAt: h.serverInfos.LightRAGCreatedAt,
		Message: models.OllamaMessage{
			Role:    "assistant",
			Content: "This is a mock chat response for: " + lastUserMessage,
		},
		Done: true,
	}

	c.JSON(http.StatusOK, resp)
}

// Embeddings handles embedding requests.
func (h *OllamaHandler) Embeddings(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"embedding": []float64{0.1, 0.2, 0.3}, // Mock embedding
	})
}

// ParseQueryMode parses query prefix to determine search mode.
// Returns (cleaned_query, search_mode, only_need_context, user_prompt)
func ParseQueryMode(query string) (string, types.SearchMode, bool, *string) {
	var userPrompt *string

	// First check if there's a bracket format for user prompt
	bracketPattern := regexp.MustCompile(`^/([a-z]*)\[(.*?)\](.*)`)
	bracketMatch := bracketPattern.FindStringSubmatch(query)

	if bracketMatch != nil {
		modePrefix := bracketMatch[1]
		prompt := bracketMatch[2]
		remainingQuery := strings.TrimLeft(bracketMatch[3], " ")
		userPrompt = &prompt

		// Reconstruct query, removing the bracket part
		query = "/" + modePrefix + " " + remainingQuery
		query = strings.TrimSpace(query)
	}

	// Mode mapping
	modeMap := map[string]struct {
		mode            types.SearchMode
		onlyNeedContext bool
	}{
		"/local ":        {types.SearchModeLocal, false},
		"/global ":       {types.SearchModeGlobal, false},
		"/naive ":        {types.SearchModeNaive, false},
		"/hybrid ":       {types.SearchModeHybrid, false},
		"/mix ":          {types.SearchModeMix, false},
		"/bypass ":       {types.SearchModeBypass, false},
		"/context":       {types.SearchModeMix, true},
		"/localcontext":  {types.SearchModeLocal, true},
		"/globalcontext": {types.SearchModeGlobal, true},
		"/hybridcontext": {types.SearchModeHybrid, true},
		"/naivecontext":  {types.SearchModeNaive, true},
		"/mixcontext":    {types.SearchModeMix, true},
	}

	for prefix, modeInfo := range modeMap {
		if strings.HasPrefix(query, prefix) {
			cleanedQuery := strings.TrimLeft(query[len(prefix):], " ")
			return cleanedQuery, modeInfo.mode, modeInfo.onlyNeedContext, userPrompt
		}
	}

	return query, types.SearchModeMix, false, userPrompt
}

// EstimateTokens estimates the number of tokens in text.
// This is a simple approximation based on word count.
func EstimateTokens(text string) int {
	// Simple approximation: ~4 characters per token on average
	return len(text) / 4
}
