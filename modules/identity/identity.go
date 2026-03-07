// Package identity is the identity bounded context. It owns the student
// profile — the extended academic profile associated with a user.
//
// Note: Auth handlers + middleware remain in handlers/auth/ because the
// auth middleware is shared infrastructure imported by every module. A future
// ADR may extract the middleware to its own package.
//
// Extracted as part of the modular-monolith migration
// (see arch/adr/013-modular-monolith-phase2-remaining-modules.md). It replaces
// handlers/profile, infra/sqldb/student_profile_repo.go, and
// domain/student_profile.go.
//
// Public API for main.go:
//
//	module := identity.New(log, db)
//	module.RegisterRoutes(r, authMiddleware)
//
// Cross-module accessor:
//
//	module.StudentProfileRepo() — returns StudentProfileRepository
//	    for callers that need the profile repo (e.g., agentorchestrator)
package identity

import (
	"school-with-ai/foundation/logger"
	"school-with-ai/handlers/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the identity bounded context.
type Module struct {
	handler     *handler
	profileRepo StudentProfileRepository
}

// New creates the identity module.
func New(log *logger.Logger, db *sqlx.DB) *Module {
	repo := newPostgresStudentProfileRepository(db)
	h := newHandler(log, repo)

	return &Module{
		handler:     h,
		profileRepo: repo,
	}
}

// RegisterRoutes mounts all identity/profile HTTP routes onto the given chi.Router.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/profile", authMW.RequireAuthFunc(m.handler.handleProfilePage))
	r.Get("/api/profile", authMW.RequireAuthFunc(m.handler.getProfile))
	r.Put("/api/profile", authMW.RequireAuthFunc(m.handler.updateProfile))
}

// StudentProfileRepo returns the student profile repository for cross-module
// consumers (e.g., agent orchestrator, custom runner).
func (m *Module) StudentProfileRepo() StudentProfileRepository {
	return m.profileRepo
}
