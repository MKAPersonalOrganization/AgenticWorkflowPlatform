package evaluation

import (
	"aicompass/foundation/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the evaluation bounded context. It owns the repository, service,
// and handler as internal collaborators, exposing only the wiring methods that
// main.go (and other modules) need.
//
// Public API for main.go:
//
//	module := evaluation.New(db)
//	module.RegisterRoutes(r, authMiddleware)
type Module struct {
	handler *handler
	svc     *service
}

// New creates the evaluation module.
func New(db *sqlx.DB) *Module {
	repo := newPostgresEvalRepo(db)
	svc := newService(repo)
	h := newHandler(svc)

	return &Module{handler: h, svc: svc}
}

// RegisterRoutes mounts all evaluation HTTP routes onto the given chi.Router.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	// EvalSet routes
	r.Get("/api/eval-sets", authMW.RequireAuthFunc(m.handler.listEvalSets))
	r.Post("/api/eval-sets", authMW.RequireAuthFunc(m.handler.createEvalSet))
	r.Get("/api/eval-sets/{id}", authMW.RequireAuthFunc(m.handler.getEvalSet))
	r.Put("/api/eval-sets/{id}", authMW.RequireAuthFunc(m.handler.updateEvalSet))
	r.Delete("/api/eval-sets/{id}", authMW.RequireAuthFunc(m.handler.deleteEvalSet))

	// Import/export routes
	r.Post("/api/eval-sets/import", authMW.RequireAuthFunc(m.handler.importEvalSet))
	r.Get("/api/eval-sets/{id}/export", authMW.RequireAuthFunc(m.handler.exportEvalSet))

	// EvalCase routes (scoped under eval_set)
	r.Get("/api/eval-sets/{id}/cases", authMW.RequireAuthFunc(m.handler.listEvalCases))
	r.Post("/api/eval-sets/{id}/cases", authMW.RequireAuthFunc(m.handler.createEvalCase))
	r.Post("/api/eval-sets/{id}/reorder", authMW.RequireAuthFunc(m.handler.reorderEvalCases))

	// EvalCase routes (direct by ID)
	r.Put("/api/eval-cases/{id}", authMW.RequireAuthFunc(m.handler.updateEvalCase))
	r.Delete("/api/eval-cases/{id}", authMW.RequireAuthFunc(m.handler.deleteEvalCase))
}
