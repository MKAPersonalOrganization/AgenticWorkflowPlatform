// Package preferences is the user-preferences bounded context. It owns CRUD
// operations for per-user key-value preferences used for cross-device UI state
// persistence (e.g., selected agent, sidebar state).
//
// Extracted as part of the modular-monolith migration
// (see arch/adr/013-modular-monolith-phase2-remaining-modules.md). It replaces
// three former locations: handlers/preferences, infra/sqldb/user_preference_repo.go,
// and domain/user_preference.go.
//
// Public API for main.go:
//
//	module := preferences.New(log, db)
//	module.RegisterRoutes(r, authMiddleware)
package preferences

import (
	"school-with-ai/foundation/logger"
	"school-with-ai/handlers/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the user-preferences bounded context.
type Module struct {
	handler *handler
}

// New creates the preferences module.
func New(log *logger.Logger, db *sqlx.DB) *Module {
	repo := newPostgresUserPreferenceRepository(db)
	h := newHandler(log, repo)

	return &Module{handler: h}
}

// RegisterRoutes mounts all user-preference HTTP routes onto the given chi.Router.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/api/preferences", authMW.RequireAuthFunc(m.handler.getAllPreferences))
	r.Get("/api/preferences/{key}", authMW.RequireAuthFunc(m.handler.getPreference))
	r.Put("/api/preferences/{key}", authMW.RequireAuthFunc(m.handler.setPreference))
}
