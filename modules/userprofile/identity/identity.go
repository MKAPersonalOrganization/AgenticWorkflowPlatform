package identity

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// SubModule is the identity (user profile) sub-module.
type SubModule struct {
	handler     *handler
	profileRepo UserProfileRepository
}

// New creates the identity sub-module.
func New(log *logger.Logger, db *sqlx.DB) *SubModule {
	repo := newPostgresUserProfileRepository(db)
	h := newHandler(log, repo)

	return &SubModule{
		handler:     h,
		profileRepo: repo,
	}
}

// RegisterRoutes mounts all identity/profile HTTP routes onto the given chi.Router.
func (m *SubModule) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/profile", authMW.RequireAuthFunc(m.handler.handleProfilePage))
	r.Get("/api/profile", authMW.RequireAuthFunc(m.handler.getProfile))
	r.Put("/api/profile", authMW.RequireAuthFunc(m.handler.updateProfile))
}

// UserProfileRepo returns the user profile repository for cross-module
// consumers (e.g., agent orchestrator, custom runner).
func (m *SubModule) UserProfileRepo() UserProfileRepository {
	return m.profileRepo
}
