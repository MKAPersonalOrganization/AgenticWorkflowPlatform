package preferences

import (
	"encoding/json"
	"net/http"
	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
)

// Handler manages user preference requests.
type Handler struct {
	Log  *logger.Logger
	Repo domain.UserPreferenceRepository
}

// NewHandler creates a new preferences handler.
func NewHandler(log *logger.Logger, repo domain.UserPreferenceRepository) *Handler {
	return &Handler{Log: log, Repo: repo}
}

// GetPreference handles GET /api/preferences/{key}
func (h *Handler) GetPreference(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	key := r.PathValue("key")
	if key == "" {
		http.Error(w, "Missing preference key", http.StatusBadRequest)
		return
	}

	pref, err := h.Repo.Get(r.Context(), user.ID, key)
	if err != nil {
		h.Log.Error(r.Context(), "failed to get preference", "error", err, "key", key)
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

// SetPreference handles PUT /api/preferences/{key}
func (h *Handler) SetPreference(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
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

	pref := &domain.UserPreference{
		UserID: user.ID,
		Key:    key,
		Value:  req.Value,
	}

	if err := h.Repo.Set(r.Context(), pref); err != nil {
		h.Log.Error(r.Context(), "failed to set preference", "error", err, "key", key)
		http.Error(w, "Failed to save preference", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// GetAllPreferences handles GET /api/preferences
func (h *Handler) GetAllPreferences(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	prefs, err := h.Repo.GetAll(r.Context(), user.ID)
	if err != nil {
		h.Log.Error(r.Context(), "failed to list preferences", "error", err)
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
