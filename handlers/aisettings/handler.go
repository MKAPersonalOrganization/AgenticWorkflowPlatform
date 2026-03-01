package aisettings

import (
	"encoding/json"
	"net/http"

	"school-with-ai/domain"
	"school-with-ai/foundation/i18n"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/aisettings"

	"github.com/go-chi/chi/v5"
)

// Handler manages AI settings HTTP requests.
type Handler struct {
	log     *logger.Logger
	service *aisettings.Service
}

// NewHandler creates a new AI settings handler.
func NewHandler(log *logger.Logger, service *aisettings.Service) *Handler {
	return &Handler{log: log, service: service}
}

// --------------------------------------------------------------------------
// Response types
// --------------------------------------------------------------------------

type providerInfo struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Models          []string `json:"models"`
	EmbeddingModels []string `json:"embedding_models"`
	TTSModels       []string `json:"tts_models"`
	SupportsTTS     bool     `json:"supports_tts"`
}

type settingsResponse struct {
	ID                  string  `json:"id"`
	Provider            string  `json:"provider"`
	APIKeySet           bool    `json:"api_key_set"`
	APIKeyMasked        string  `json:"api_key_masked"`
	DefaultLLMModel     string  `json:"default_llm_model"`
	DefaultEmbedModel   string  `json:"default_embed_model"`
	IsDefault           bool    `json:"is_default"`
	MaxTokens           int     `json:"max_tokens"`
	Temperature         float64 `json:"temperature"`
	EmbeddingDimensions int     `json:"embedding_dimensions"`
	TTSModel            string  `json:"tts_model"`
	TTSVoice            string  `json:"tts_voice"`
}

func toSettingsResponse(s domain.AISettings) settingsResponse {
	return settingsResponse{
		ID:                  s.ID,
		Provider:            string(s.Provider),
		APIKeySet:           s.APIKey != "",
		APIKeyMasked:        s.MaskedAPIKey(),
		DefaultLLMModel:     s.DefaultLLMModel,
		DefaultEmbedModel:   s.DefaultEmbedModel,
		IsDefault:           s.IsDefault,
		MaxTokens:           s.MaxTokens,
		Temperature:         s.Temperature,
		EmbeddingDimensions: s.EmbeddingDimensions,
		TTSModel:            s.TTSModel,
		TTSVoice:            s.TTSVoice,
	}
}

// --------------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------------

// ListProviders handles GET /api/ai-settings/providers
func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	providers := h.service.ListProviders()

	var result []providerInfo
	for _, p := range providers {
		ttsModels := h.service.ListTTSModels(p)
		result = append(result, providerInfo{
			ID:              string(p),
			Name:            providerDisplayName(p),
			Models:          h.service.ListModels(p),
			EmbeddingModels: h.service.ListEmbeddingModels(p),
			TTSModels:       ttsModels,
			SupportsTTS:     len(ttsModels) > 0,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// GetSettings handles GET /api/ai-settings
func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	settings, err := h.service.GetUserSettings(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get ai settings", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to retrieve AI settings"), http.StatusInternalServerError)
		return
	}

	var result []settingsResponse
	for _, s := range settings {
		result = append(result, toSettingsResponse(s))
	}
	if result == nil {
		result = []settingsResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// SaveSettings handles PUT /api/ai-settings
func (h *Handler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	var req struct {
		Provider            string  `json:"provider"`
		APIKey              string  `json:"api_key"`
		DefaultLLMModel     string  `json:"default_llm_model"`
		DefaultEmbedModel   string  `json:"default_embed_model"`
		MaxTokens           int     `json:"max_tokens"`
		Temperature         float64 `json:"temperature"`
		EmbeddingDimensions int     `json:"embedding_dimensions"`
		TTSModel            string  `json:"tts_model"`
		TTSVoice            string  `json:"tts_voice"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "Invalid request body"), http.StatusBadRequest)
		return
	}

	settings := &domain.AISettings{
		UserID:              user.ID,
		Provider:            domain.AIProvider(req.Provider),
		APIKey:              req.APIKey,
		DefaultLLMModel:     req.DefaultLLMModel,
		DefaultEmbedModel:   req.DefaultEmbedModel,
		MaxTokens:           req.MaxTokens,
		Temperature:         req.Temperature,
		EmbeddingDimensions: req.EmbeddingDimensions,
		TTSModel:            req.TTSModel,
		TTSVoice:            req.TTSVoice,
	}

	// If no API key provided, try to reuse the existing key for this provider
	if settings.APIKey == "" {
		existing, err := h.service.GetUserSettings(r.Context(), user.ID)
		if err == nil {
			for _, ex := range existing {
				if ex.Provider == settings.Provider && ex.APIKey != "" {
					settings.APIKey = ex.APIKey
					// Preserve existing fields when only updating embed model
					if settings.DefaultLLMModel == "" {
						settings.DefaultLLMModel = ex.DefaultLLMModel
					}
					if settings.MaxTokens == 0 {
						settings.MaxTokens = ex.MaxTokens
					}
					if settings.Temperature == 0 {
						settings.Temperature = ex.Temperature
					}
					if settings.TTSModel == "" {
						settings.TTSModel = ex.TTSModel
					}
					if settings.TTSVoice == "" {
						settings.TTSVoice = ex.TTSVoice
					}
					break
				}
			}
		}
	}

	if err := h.service.SaveSettings(r.Context(), settings); err != nil {
		h.log.Error(r.Context(), "failed to save ai settings", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(toSettingsResponse(*settings))
}

// DeleteSettings handles DELETE /api/ai-settings/{id}
func (h *Handler) DeleteSettings(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, i18n.T(r.Context(), "Missing id"), http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteSettings(r.Context(), id, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to delete ai settings", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to delete AI settings"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SetDefault handles PUT /api/ai-settings/{id}/default
func (h *Handler) SetDefault(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, i18n.T(r.Context(), "Missing id"), http.StatusBadRequest)
		return
	}

	if err := h.service.SetDefault(r.Context(), id, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to set default ai settings", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to set default"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func providerDisplayName(p domain.AIProvider) string {
	switch p {
	case domain.AIProviderOpenAI:
		return "OpenAI"
	case domain.AIProviderAnthropic:
		return "Anthropic"
	case domain.AIProviderGemini:
		return "Google Gemini"
	default:
		return string(p)
	}
}

// GetTTSVoices handles GET /api/ai-settings/tts-voices?model=...
func (h *Handler) GetTTSVoices(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	voices := domain.TTSVoices(model)
	if voices == nil {
		voices = []string{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"model":  model,
		"voices": voices,
	})
}
