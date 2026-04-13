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
	// Metric catalog routes (public, no auth needed)
	r.Get("/api/evaluation/metrics", m.handler.listMetrics)
	r.Get("/api/evaluation/metrics/{id}", m.handler.getMetric)

	// Metric profile routes
	r.Get("/api/evaluation/profiles", authMW.RequireAuthFunc(m.handler.listMetricProfiles))
	r.Post("/api/evaluation/profiles", authMW.RequireAuthFunc(m.handler.createMetricProfile))
	r.Get("/api/evaluation/profiles/{id}", authMW.RequireAuthFunc(m.handler.getMetricProfile))
	r.Put("/api/evaluation/profiles/{id}", authMW.RequireAuthFunc(m.handler.updateMetricProfile))
	r.Delete("/api/evaluation/profiles/{id}", authMW.RequireAuthFunc(m.handler.deleteMetricProfile))
	r.Post("/api/evaluation/profiles/{id}/set-default", authMW.RequireAuthFunc(m.handler.setDefaultProfile))

	// Evaluation run routes
	r.Post("/api/evaluation/runs", authMW.RequireAuthFunc(m.handler.createEvalRun))
	r.Get("/api/evaluation/runs", authMW.RequireAuthFunc(m.handler.listEvalRuns))
	r.Get("/api/evaluation/runs/{id}", authMW.RequireAuthFunc(m.handler.getEvalRun))
	r.Get("/api/evaluation/runs/{id}/results", authMW.RequireAuthFunc(m.handler.getEvalResults))
	r.Post("/api/evaluation/runs/{id}/cancel", authMW.RequireAuthFunc(m.handler.cancelEvalRun))
	r.Delete("/api/evaluation/runs/{id}", authMW.RequireAuthFunc(m.handler.deleteEvalRun))

	// Rubric routes
	r.Get("/api/evaluation/rubrics", authMW.RequireAuthFunc(m.handler.listRubrics))
	r.Post("/api/evaluation/rubrics", authMW.RequireAuthFunc(m.handler.createRubric))
	r.Get("/api/evaluation/rubrics/{id}", authMW.RequireAuthFunc(m.handler.getRubric))
	r.Put("/api/evaluation/rubrics/{id}", authMW.RequireAuthFunc(m.handler.updateRubric))
	r.Delete("/api/evaluation/rubrics/{id}", authMW.RequireAuthFunc(m.handler.deleteRubric))

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
