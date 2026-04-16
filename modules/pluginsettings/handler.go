package pluginsettings

import (
	"aicompass/foundation/i18n"
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"
	"aicompass/shared"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// handler manages plugin-settings HTTP requests.
type handler struct {
	log *logger.Logger
	svc *Service
}

func newHandler(log *logger.Logger, svc *Service) *handler {
	return &handler{log: log, svc: svc}
}

// ─── Response types ─────────────────────────────────────────────────────────

type pluginSettingsResponse struct {
	PluginID string            `json:"plugin_id"`
	Settings map[string]string `json:"settings"` // masked for sensitive fields
}

type pluginSchemaResponse struct {
	PluginID string                       `json:"plugin_id"`
	Icon     string                       `json:"icon"`
	Schema   []toolregistry.SettingsField `json:"schema"`
}

// ─── Handlers ───────────────────────────────────────────────────────────────

// listSchemas handles GET /api/plugin-settings/schemas
// Returns the settings schema for every plugin that declares one.
func (h *handler) listSchemas(w http.ResponseWriter, r *http.Request) {
	schemas := toolregistry.GetAllPluginSettingsSchemas()

	out := make([]pluginSchemaResponse, 0, len(schemas))
	for _, s := range schemas {
		out = append(out, pluginSchemaResponse{
			PluginID: s.PluginID,
			Icon:     s.Icon,
			Schema:   s.Schema,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// getAll handles GET /api/plugin-settings
// Returns every plugin's settings for the authenticated user (values masked).
func (h *handler) getAll(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	all, err := h.svc.GetAllForUser(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get plugin settings", "error", err)
		http.Error(w, i18n.T(r.Context(), "Failed to retrieve plugin settings"), http.StatusInternalServerError)
		return
	}

	out := make([]pluginSettingsResponse, 0, len(all))
	for _, s := range all {
		sensitiveKeys := SensitiveKeysForPlugin(s.PluginID)
		out = append(out, pluginSettingsResponse{
			PluginID: s.PluginID,
			Settings: s.MaskedSettings(sensitiveKeys),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// getOne handles GET /api/plugin-settings/{pluginID}
func (h *handler) getOne(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	pluginID := chi.URLParam(r, "pluginID")
	if pluginID == "" {
		http.Error(w, "Missing pluginID", http.StatusBadRequest)
		return
	}

	s, err := h.svc.GetForUserPlugin(r.Context(), user.ID, pluginID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get plugin settings", "error", err, "plugin_id", pluginID)
		http.Error(w, i18n.T(r.Context(), "Failed to retrieve plugin settings"), http.StatusInternalServerError)
		return
	}

	if s == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pluginSettingsResponse{
			PluginID: pluginID,
			Settings: map[string]string{},
		})
		return
	}

	sensitiveKeys := SensitiveKeysForPlugin(pluginID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pluginSettingsResponse{
		PluginID: s.PluginID,
		Settings: s.MaskedSettings(sensitiveKeys),
	})
}

// save handles PUT /api/plugin-settings/{pluginID}
func (h *handler) save(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	pluginID := chi.URLParam(r, "pluginID")
	if pluginID == "" {
		http.Error(w, "Missing pluginID", http.StatusBadRequest)
		return
	}

	var req struct {
		Settings map[string]string `json:"settings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "Invalid request body"), http.StatusBadRequest)
		return
	}

	if err := h.svc.Save(r.Context(), user.ID, pluginID, req.Settings); err != nil {
		h.log.Error(r.Context(), "failed to save plugin settings", "error", err, "plugin_id", pluginID)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// remove handles DELETE /api/plugin-settings/{pluginID}
func (h *handler) remove(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	pluginID := chi.URLParam(r, "pluginID")
	if pluginID == "" {
		http.Error(w, "Missing pluginID", http.StatusBadRequest)
		return
	}

	if err := h.svc.Delete(r.Context(), user.ID, pluginID); err != nil {
		h.log.Error(r.Context(), "failed to delete plugin settings", "error", err, "plugin_id", pluginID)
		http.Error(w, i18n.T(r.Context(), "Failed to delete plugin settings"), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
