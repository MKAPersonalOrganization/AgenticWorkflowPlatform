package userprofile

import (
	"school-with-ai/domain"
	"school-with-ai/foundation/auth"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/userprofile/identity"
	"school-with-ai/modules/userprofile/preferences"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the unified user-profile bounded context. It merges three
// formerly-separate tiny modules (identity, preferences, banner) that share
// a common trait: per-user UI configuration with no domain logic.
type Module struct {
	identity    *identity.SubModule
	preferences *preferences.SubModule
	banner      *bannerHandler
}

// New creates the userprofile module.
func New(log *logger.Logger, db *sqlx.DB) *Module {
	return &Module{
		identity:    identity.New(log, db),
		preferences: preferences.New(log, db),
		banner:      &bannerHandler{log: log},
	}
}

// RegisterRoutes mounts all user-profile HTTP routes onto the given chi.Router.
// This replaces the three separate RegisterRoutes calls that were previously
// made in main.go for identity, preferences, and banner.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	// Identity (student profile) routes
	m.identity.RegisterRoutes(r, authMW)

	// Preferences routes
	m.preferences.RegisterRoutes(r, authMW)

	// Banner route
	r.Get("/api/banner", authMW.RequireAuthFunc(m.banner.getBanner))
}

// StudentProfileRepo returns the student profile repository for cross-module
// consumers (e.g., agent orchestrator, custom runner).
func (m *Module) StudentProfileRepo() domain.StudentProfileRepository {
	return m.identity.StudentProfileRepo()
}
