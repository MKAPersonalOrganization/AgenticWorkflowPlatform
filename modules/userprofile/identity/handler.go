package identity

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/i18n"
	"aicompass/foundation/logger"
	"aicompass/frontend/ui/profile"
	"encoding/json"
	"net/http"
	"strings"
)

// handler manages user profile HTTP requests.
type handler struct {
	log  *logger.Logger
	repo UserProfileRepository
}

// newHandler creates a new profile handler.
func newHandler(log *logger.Logger, repo UserProfileRepository) *handler {
	return &handler{log: log, repo: repo}
}

// handleProfilePage serves the HTML profile page.
func (h *handler) handleProfilePage(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}

	profileData, err := h.repo.GetByUserID(r.Context(), user.ID)
	if err != nil {
		h.log.Error(r.Context(), "failed to get profile for page", "error", err)
		// We can still render the page with an empty profile or error handling
	}

	if profileData == nil {
		profileData = &UserProfile{UserID: user.ID}
	}

	component := profile.ProfilePage(profile.ProfilePageProps{
		User: user,
		Profile: profile.UserProfileDTO{
			LearningStyle: profileData.LearningStyle,
			Struggles:     profileData.Struggles,
			Strengths:     profileData.Strengths,
			Goals:         profileData.Goals,
		},
	})

	if err := component.Render(r.Context(), w); err != nil {
		h.log.Error(r.Context(), "failed to render profile page", "error", err)
		http.Error(w, i18n.T(r.Context(), "Internal Server Error"), http.StatusInternalServerError)
	}
}

// getProfile handles GET /api/profile
func (h *handler) getProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, i18n.T(r.Context(), "Unauthorized"), http.StatusUnauthorized)
		return
	}

	p, err := h.repo.GetByUserID(r.Context(), user.ID)
	if err != nil {
		http.Error(w, i18n.T(r.Context(), "Failed to retrieve profile"), http.StatusInternalServerError)
		return
	}

	if p == nil {
		// Return empty default profile instead of 404 for better UX in form
		p = &UserProfile{UserID: user.ID}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(p); err != nil {
		h.log.Error(r.Context(), "failed to encode profile response", "error", err)
	}
}

// updateProfile handles PUT /api/profile
func (h *handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
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

	p := &UserProfile{
		UserID:        user.ID,
		LearningStyle: req.LearningStyle,
		Struggles:     parseList(req.Struggles),
		Strengths:     parseList(req.Strengths),
		Goals:         req.Goals,
	}

	if err := h.repo.Upsert(r.Context(), p); err != nil {
		h.log.Error(r.Context(), "failed to update profile", "error", err, "user_id", user.ID)
		http.Error(w, i18n.T(r.Context(), "Failed to update profile"), http.StatusInternalServerError)
		return
	}

	h.log.Info(r.Context(), "profile updated successfully", "user_id", user.ID, "learning_style", p.LearningStyle)
	w.WriteHeader(http.StatusOK)
}
