package aisettings

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/i18n"
	"aicompass/foundation/logger"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handler manages AI settings HTTP requests.
type handler struct {
	log *logger.Logger
	svc *service
}

// newHandler creates a new AI settings handler.
func newHandler(log *logger.Logger, svc *service) *handler {
	return &handler{log: log, svc: svc}
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
	ID           string `json:"id"`
	Provider     string `json:"provider"`
	APIKeySet    bool   `json:"api_key_set"`
	APIKeyMasked string `json:"api_key_masked"`
	IsDefault    bool   `json:"is_default"`
}

type capabilityConfigResponse struct {
	ID         string          `json:"id"`
	Capability string          `json:"capability"`
	Provider   string          `json:"provider"`
	Model      string          `json:"model"`
	Config     json.RawMessage `json:"config"`
}

func toSettingsResponse(s AISettings) settingsResponse {
	return settingsResponse{
		ID:           s.ID,
		Provider:     string(s.Provider),
		APIKeySet:    s.APIKey != "",
		APIKeyMasked: s.MaskedAPIKey(),
		IsDefault:    s.IsDefault,
	}
}

// --------------------------------------------------------------------------
// Handlers
// --------------------------------------------------------------------------

// listProviders handles GET /api/ai-settings/providers
func (h *handler) listProviders(w http.ResponseWriter, r *http.Request) {
	providers := h.svc.listProviders()

	var result []providerInfo
	for _, p := range providers {
		ttsModels := h.svc.listTTSModels(p)
		result = append(result, providerInfo{
			ID:              string(p),
			Name:            providerDisplayName(p),
			Models:          h.svc.listModels(p),
			EmbeddingModels: h.svc.listEmbeddingModels(p),
			TTSModels:       ttsModels,
			SupportsTTS:     len(ttsModels) > 0,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// getSettings handles GET /api/ai-settings
func (h *handler) getSettings(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	settings, err := h.svc.getUserSettings(r.Context(), user.ID)
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
	_ = json.NewEncoder(w).Encode(result)
}

// saveSettings handles PUT /api/ai-settings
func (h *handler) saveSettings(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	var req struct {
		Provider string `json:"provider"`
		APIKey   string `json:"api_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "Invalid request body"), http.StatusBadRequest)
		return
	}

	settings := &AISettings{
		UserID:   user.ID,
		Provider: AIProvider(req.Provider),
		APIKey:   req.APIKey,
	}

	// If no API key provided, try to reuse the existing key for this provider
	if settings.APIKey == "" {
		existing, err := h.svc.getUserSettings(r.Context(), user.ID)
		if err == nil {
			for _, ex := range existing {
				if ex.Provider == settings.Provider && ex.APIKey != "" {
					settings.APIKey = ex.APIKey
					break
				}
			}
		}
	}

	if err := h.svc.saveSettings(r.Context(), settings); err != nil {
		h.log.Error(r.Context(), "failed to save ai settings", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toSettingsResponse(*settings))
}

// deleteSettings handles DELETE /api/ai-settings/{id}
func (h *handler) deleteSettings(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, i18n.T(r.Context(), "Missing id"), http.StatusBadRequest)
		return
	}

	if err := h.svc.deleteSettings(r.Context(), id, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to delete ai settings", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to delete AI settings"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// setDefault handles PUT /api/ai-settings/{id}/default
func (h *handler) setDefault(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, i18n.T(r.Context(), "Missing id"), http.StatusBadRequest)
		return
	}

	if err := h.svc.setDefault(r.Context(), id, user.ID); err != nil {
		h.log.Error(r.Context(), "failed to set default ai settings", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to set default"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func providerDisplayName(p AIProvider) string {
	switch p {
	case AIProviderOpenAI:
		return "OpenAI"
	case AIProviderAnthropic:
		return "Anthropic"
	case AIProviderGemini:
		return "Google Gemini"
	default:
		return string(p)
	}
}

// listProviderModels handles GET /api/ai-settings/providers/{provider}/models
// It proxies the call to the provider's model-listing API using the user's stored key.
func (h *handler) listProviderModels(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	provider := chi.URLParam(r, "provider")
	models, err := h.svc.listModelsFromProvider(r.Context(), user.ID, provider)
	if err != nil {
		h.log.Error(r.Context(), "failed to list provider models", "error", err, "provider", provider)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if models == nil {
		models = []ProviderModel{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(models)
}

// getTTSVoices handles GET /api/ai-settings/tts-voices?model=...
func (h *handler) getTTSVoices(w http.ResponseWriter, r *http.Request) {
	model := r.URL.Query().Get("model")
	voices := TTSVoices(model)
	if voices == nil {
		voices = []string{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"model":  model,
		"voices": voices,
	})
}

// --------------------------------------------------------------------------
// Capability Config Handlers
// --------------------------------------------------------------------------

// getCapabilityConfigs handles GET /api/ai-settings/capabilities
func (h *handler) getCapabilityConfigs(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	configs, err := h.svc.getAllCapabilityConfigs(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get capability configs", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to retrieve capability configs"), http.StatusInternalServerError)
		return
	}

	var result []capabilityConfigResponse
	for _, c := range configs {
		result = append(result, capabilityConfigResponse{
			ID:         c.ID,
			Capability: string(c.Capability),
			Provider:   string(c.Provider),
			Model:      c.Model,
			Config:     c.Config,
		})
	}
	if result == nil {
		result = []capabilityConfigResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

// saveCapabilityConfig handles PUT /api/ai-settings/capabilities/{capability}
func (h *handler) saveCapabilityConfig(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	capability := chi.URLParam(r, "capability")
	if capability == "" {
		http.Error(w, "Missing capability", http.StatusBadRequest)
		return
	}

	var req struct {
		Provider string          `json:"provider"`
		Model    string          `json:"model"`
		Config   json.RawMessage `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "Invalid request body"), http.StatusBadRequest)
		return
	}

	config := &AICapabilityConfig{
		UserID:     user.ID,
		Capability: AICapability(capability),
		Provider:   AIProvider(req.Provider),
		Model:      req.Model,
		Config:     req.Config,
	}

	if err := h.svc.saveCapabilityConfig(r.Context(), config); err != nil {
		h.log.Error(r.Context(), "failed to save capability config", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(capabilityConfigResponse{
		ID:         config.ID,
		Capability: string(config.Capability),
		Provider:   string(config.Provider),
		Model:      config.Model,
		Config:     config.Config,
	})
}

// deleteCapabilityConfig handles DELETE /api/ai-settings/capabilities/{capability}
func (h *handler) deleteCapabilityConfig(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	capability := chi.URLParam(r, "capability")
	if capability == "" {
		http.Error(w, "Missing capability", http.StatusBadRequest)
		return
	}

	if err := h.svc.deleteCapabilityConfig(r.Context(), user.ID, AICapability(capability)); err != nil {
		h.log.Error(r.Context(), "failed to delete capability config", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to delete capability config"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
