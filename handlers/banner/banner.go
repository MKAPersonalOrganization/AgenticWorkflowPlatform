package banner

import (
	"encoding/json"
	"net/http"

	"school-with-ai/foundation/logger"
)

type Banner struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	Persistable bool   `json:"persistable"`
}

type Handler struct {
	log *logger.Logger
}

func NewHandler(log *logger.Logger) *Handler {
	return &Handler{
		log: log,
	}
}

func (h *Handler) GetBanner(w http.ResponseWriter, r *http.Request) {
	banner := Banner{
		ID:          "welcome-v3",
		Message:     "Welcome to the new version! <a href='#'>Learn more</a>",
		Persistable: false,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(banner); err != nil {
		h.log.Error(r.Context(), "failed to encode banner response", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}
