package agentdefinition

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the agent-definition bounded context.
//
// It owns:
//   - CRUD for user-created agent definitions (both simple and composed)
//   - Composition graph validation (sequential, parallel, loop patterns)
//   - Agent sharing (share token generation, forking)
//   - YAML import of agent definitions
//
// In the unified model, every agent — whether a simple LLM agent or a composed
// multi-agent workflow — is an AgentDefinition. This mirrors ADK-Go where
// every agent type implements the same Agent interface.
type Module struct {
	log          *logger.Logger
	storeHandler *storeHandler
	svc          *Service
	repo         AgentDefinitionRepository
}

// New creates the agent-definition module.
func New(log *logger.Logger, db *sqlx.DB) *Module {
	repo := newPostgresAgentDefinitionRepository(db)
	svc := newService(log, repo)
	h := newStoreHandler(log, svc)

	return &Module{
		log:          log,
		storeHandler: h,
		svc:          svc,
		repo:         repo,
	}
}

// RegisterRoutes mounts all agent-definition HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	// Agent definition CRUD (handles both simple and composed agents)
	r.Get("/v1/agent-definitions", authMW.RequireAuthFunc(m.storeHandler.list))
	r.Get("/v1/agent-definitions/search", authMW.RequireAuthFunc(m.storeHandler.search))
	r.Post("/v1/agent-definitions", authMW.RequireAuthFunc(m.storeHandler.create))
	r.Post("/v1/agent-definitions/import-yaml", authMW.RequireAuthFunc(m.storeHandler.importYAML))
	r.Get("/v1/agent-definitions/{id}", authMW.RequireAuthFunc(m.storeHandler.get))
	r.Put("/v1/agent-definitions/{id}", authMW.RequireAuthFunc(m.storeHandler.update))
	r.Delete("/v1/agent-definitions/{id}", authMW.RequireAuthFunc(m.storeHandler.delete))
	r.Delete("/v1/agent-definitions", authMW.RequireAuthFunc(m.storeHandler.deleteAll))
	r.Get("/v1/agent-definitions/{id}/consumers", authMW.RequireAuthFunc(m.storeHandler.consumers))
	r.Get("/v1/agent-definitions/{id}/available-state-keys", authMW.RequireAuthFunc(m.storeHandler.availableStateKeys))

	// Agent sharing
	r.Post("/v1/agent-definitions/{id}/share", authMW.RequireAuthFunc(m.storeHandler.shareAgent))
	r.Delete("/v1/agent-definitions/{id}/share", authMW.RequireAuthFunc(m.storeHandler.unshareAgent))
	r.Get("/v1/shared/agents/{token}", m.storeHandler.getSharedAgent) // public — no auth
	r.Post("/v1/shared/agents/{token}/fork", authMW.RequireAuthFunc(m.storeHandler.forkSharedAgent))

	// Backward-compatible workflow routes (redirect to agent definitions)
	r.Get("/v1/workflows", authMW.RequireAuthFunc(m.storeHandler.list))
	r.Post("/v1/workflows", authMW.RequireAuthFunc(m.storeHandler.create))
	r.Get("/v1/workflows/{id}", authMW.RequireAuthFunc(m.storeHandler.get))
	r.Put("/v1/workflows/{id}", authMW.RequireAuthFunc(m.storeHandler.update))
	r.Delete("/v1/workflows/{id}", authMW.RequireAuthFunc(m.storeHandler.delete))
}

// Repo returns the agent definition repository for cross-module access.
func (m *Module) Repo() AgentDefinitionRepository {
	return m.repo
}

// Service returns the agent definition service for cross-module access.
func (m *Module) Service() *Service {
	return m.svc
}
