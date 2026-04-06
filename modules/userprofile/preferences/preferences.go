package preferences

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// SubModule is the user-preferences sub-module.
type SubModule struct {
	handler *handler
}

// New creates the preferences sub-module.
func New(log *logger.Logger, db *sqlx.DB) *SubModule {
	repo := newPostgresUserPreferenceRepository(db)
	h := newHandler(log, repo)

	return &SubModule{handler: h}
}

// RegisterRoutes mounts all user-preference HTTP routes onto the given chi.Router.
func (m *SubModule) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/api/preferences", authMW.RequireAuthFunc(m.handler.getAllPreferences))
	r.Get("/api/preferences/{key}", authMW.RequireAuthFunc(m.handler.getPreference))
	r.Put("/api/preferences/{key}", authMW.RequireAuthFunc(m.handler.setPreference))
}
