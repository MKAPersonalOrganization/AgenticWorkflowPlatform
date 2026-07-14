package userprofile

import (
	"aicompass/foundation/logger"
	"encoding/json"
	"net/http"
)

// Banner represents a banner message displayed to users.
type Banner struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	Persistable bool   `json:"persistable"`
}

// bannerHandler contains the handler logic for the banner endpoint.
type bannerHandler struct {
	log *logger.Logger
}

// getBanner returns the current banner as JSON.
func (h *bannerHandler) getBanner(w http.ResponseWriter, r *http.Request) {
	b := Banner{
		ID:          "welcome-demo",
		Message:     "Welcome to the demo version!",
		Persistable: false,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(b)
}
