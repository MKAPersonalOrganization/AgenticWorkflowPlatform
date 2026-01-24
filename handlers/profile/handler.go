package profile

import (
	"encoding/json"
	"net/http"
	"strings"

	"school-with-ai/components/profile"
	"school-with-ai/domain"
	"school-with-ai/foundation/i18n"
	"school-with-ai/foundation/logger"
)

// Handler manages student profile requests.
type Handler struct {
	Log  *logger.Logger
	Repo domain.StudentProfileRepository
}

// NewHandler creates a new profile handler.
func NewHandler(log *logger.Logger, repo domain.StudentProfileRepository) *Handler {
	return &Handler{Log: log, Repo: repo}
}

// HandleProfilePage serves the HTML profile page.
func (h *Handler) HandleProfilePage(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	profileData, err := h.Repo.GetByUserID(r.Context(), user.ID)
	if err != nil {
		h.Log.Error(r.Context(), "failed to get profile for page", "error", err)
		// We can still render the page with an empty profile or error handling
	}

	if profileData == nil {
		profileData = &domain.StudentProfile{UserID: user.ID}
	}

	component := profile.ProfilePage(profile.ProfilePageProps{
		User:    user,
		Profile: profileData,
	})

	if err := component.Render(r.Context(), w); err != nil {
		h.Log.Error(r.Context(), "failed to render profile page", "error", err)
		http.Error(w, i18n.T(r.Context(), "Internal Server Error"), http.StatusInternalServerError)
	}
}

// GetProfile handles GET /api/profile
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	profile, err := h.Repo.GetByUserID(r.Context(), user.ID)
	if err != nil {
		http.Error(w, i18n.T(r.Context(), "Failed to retrieve profile"), http.StatusInternalServerError)
		return
	}

	if profile == nil {
		// Return empty default profile instead of 404 for better UX in form
		profile = &domain.StudentProfile{UserID: user.ID}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(profile); err != nil {
		h.Log.Error(r.Context(), "failed to encode profile response", "error", err)
	}
}

// UpdateProfile handles PUT /api/profile
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	user := domain.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	var req struct {
		LearningStyle string `json:"learning_style"`
		Struggles     string `json:"struggles"` // Receive as comma-separated or raw string
		Strengths     string `json:"strengths"` // Receive as comma-separated or raw string
		Goals         string `json:"goals"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, i18n.T(r.Context(), "Invalid request body"), http.StatusBadRequest)
		return
	}

	// Parse comma-separated lists and trim
	parseList := func(s string) []string {
		if s == "" {
			return nil
		}
		parts := strings.Split(s, ",")
		var result []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}

	profile := &domain.StudentProfile{
		UserID:        user.ID,
		LearningStyle: req.LearningStyle,
		Struggles:     parseList(req.Struggles),
		Strengths:     parseList(req.Strengths),
		Goals:         req.Goals,
	}

	if err := h.Repo.Upsert(r.Context(), profile); err != nil {
		h.Log.Error(r.Context(), "failed to update profile", "error", err, "user_id", user.ID)
		http.Error(w, i18n.T(r.Context(), "Failed to update profile"), http.StatusInternalServerError)
		return
	}

	h.Log.Info(r.Context(), "profile updated successfully", "user_id", user.ID, "learning_style", profile.LearningStyle)
	w.WriteHeader(http.StatusOK)
}
