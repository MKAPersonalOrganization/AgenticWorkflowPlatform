package multiagent

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"aicompass/modules/agentdefinition"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// ──────────────────────────────────────────────────────────────────────────────
// Module Facade
// ──────────────────────────────────────────────────────────────────────────────

// Module is the multi-agent workflow bounded context. It owns CRUD operations
// for user-composed multi-agent workflow definitions.
//
// See arch/adr/019-multi-agent-workflow-composition.md.
type Module struct {
	log     *logger.Logger
	handler *workflowHandler
	svc     *service
	repo    WorkflowDefinitionRepository
}

// New creates the multiagent module.
func New(log *logger.Logger, db *sqlx.DB, agentDefRepo agentdefinition.AgentDefinitionRepository) *Module {
	repo := newPostgresWorkflowRepository(db)
	svc := newService(log, repo, agentDefRepo)
	handler := newWorkflowHandler(log, svc)

	return &Module{
		log:     log,
		handler: handler,
		svc:     svc,
		repo:    repo,
	}
}

// RegisterRoutes mounts all workflow HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/v1/workflows", authMW.RequireAuthFunc(m.handler.list))
	r.Post("/v1/workflows", authMW.RequireAuthFunc(m.handler.create))
	r.Get("/v1/workflows/{id}", authMW.RequireAuthFunc(m.handler.get))
	r.Put("/v1/workflows/{id}", authMW.RequireAuthFunc(m.handler.update))
	r.Delete("/v1/workflows/{id}", authMW.RequireAuthFunc(m.handler.deleteWF))
}

// ─────────────────────────────────────────────────────────────────────────────
// Cross-module accessors
// ─────────────────────────────────────────────────────────────────────────────

// WorkflowRepo returns the workflow repository for cross-module use
// (e.g., the agent provider needs it to resolve workflow agents).
func (m *Module) WorkflowRepo() WorkflowDefinitionRepository {
	return m.repo
}
