package preferences

import (
	"encoding/json"
	"net/http"

	"aicompass/foundation/logger"
	"aicompass/shared"
)

// handler manages user preference HTTP requests.
type handler struct {
	log  *logger.Logger
	repo UserPreferenceRepository
}

// newHandler creates a new preferences handler.
func newHandler(log *logger.Logger, repo UserPreferenceRepository) *handler {
	return &handler{log: log, repo: repo}
}

// getPreference handles GET /api/preferences/{key}
func (h *handler) getPreference(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "Missing preference key", http.StatusBadRequest)
		return
	}

	pref, err := h.repo.Get(r.Context(), user.ID, key)
	if err != nil {
		h.log.Error(r.Context(), "failed to get preference", "error", err, "key", key)
		http.Error(w, "Failed to retrieve preference", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if pref == nil {
		// Return empty value rather than 404 for simpler client handling
		json.NewEncoder(w).Encode(map[string]string{"key": key, "value": ""})
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"key": pref.Key, "value": pref.Value})
}

// setPreference handles PUT /api/preferences/{key}
func (h *handler) setPreference(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "Missing preference key", http.StatusBadRequest)
		return
	}

	var req struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	pref := &UserPreference{
		UserID: user.ID,
		Key:    key,
		Value:  req.Value,
	}

	if err := h.repo.Set(r.Context(), pref); err != nil {
		h.log.Error(r.Context(), "failed to set preference", "error", err, "key", key)
		http.Error(w, "Failed to save preference", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// getAllPreferences handles GET /api/preferences
func (h *handler) getAllPreferences(w http.ResponseWriter, r *http.Request) {
	user := shared.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	prefs, err := h.repo.GetAll(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to list preferences", "error", err)
		http.Error(w, "Failed to retrieve preferences", http.StatusInternalServerError)
		return
	}

	// Return as a key-value map for easy client consumption
	result := make(map[string]string)
	for _, p := range prefs {
		result[p.Key] = p.Value
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}
