package banner

import (
	"encoding/json"
	"net/http"

	"school-with-ai/foundation/logger"
	"school-with-ai/handlers/auth"

	"github.com/go-chi/chi/v5"
)

// Banner represents a banner message displayed to users.
type Banner struct {
	ID          string `json:"id"`
	Message     string `json:"message"`
	Persistable bool   `json:"persistable"`
}

// Module is the banner module.
type Module struct {
	handler *handler
}

// New creates a new banner Module.
func New(log *logger.Logger) *Module {
	return &Module{
		handler: newHandler(log),
	}
}

// RegisterRoutes registers the banner routes on the given router.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/api/banner", authMW.RequireAuthFunc(m.handler.getBanner))
}

// handler contains the handler logic for the banner module.
type handler struct {
	log *logger.Logger
}

// newHandler creates a new handler.
func newHandler(log *logger.Logger) *handler {
	return &handler{
		log: log,
	}
}

// getBanner returns the current banner as JSON.
func (h *handler) getBanner(w http.ResponseWriter, r *http.Request) {
	b := Banner{
		ID:          "welcome-v3",
		Message:     "Welcome to the new version! (with HTML link)",
		Persistable: false,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(b)
}
