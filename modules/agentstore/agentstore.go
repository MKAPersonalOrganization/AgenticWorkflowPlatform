package agentstore

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the agent-store bounded context.
//
// It owns:
//   - CRUD for user-created agent definitions (both simple and composed)
//   - Composition graph validation (sequential, parallel, loop patterns)
//   - Agent sharing (share token generation, forking)
//   - Agent catalog (browsable templates)
//
// In the unified model, every agent — whether a simple LLM agent or a composed
// multi-agent workflow — is an AgentDefinition. This mirrors ADK-Go where
// every agent type implements the same Agent interface.
type Module struct {
	log            *logger.Logger
	storeHandler   *storeHandler
	catalogHandler *catalogHandler
	svc            *service
	agentDefRepo   AgentDefinitionRepository
	catalogRepo    AgentCatalogRepository
}

// New creates the agent-store module.
func New(log *logger.Logger, db *sqlx.DB) *Module {
	agentDefRepo := newPostgresAgentDefinitionRepository(db)
	catalogRepo := newPostgresAgentCatalogRepository(db)
	svc := newService(log, agentDefRepo)
	sh := newStoreHandler(log, svc)
	ch := newCatalogHandler(log, catalogRepo, agentDefRepo, svc)

	return &Module{
		log:            log,
		storeHandler:   sh,
		catalogHandler: ch,
		svc:            svc,
		agentDefRepo:   agentDefRepo,
		catalogRepo:    catalogRepo,
	}
}

// RegisterRoutes mounts all agent-store HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	// Agent definition CRUD (handles both simple and composed agents)
	r.Get("/v1/agent-definitions", authMW.RequireAuthFunc(m.storeHandler.list))
	r.Get("/v1/agent-definitions/search", authMW.RequireAuthFunc(m.storeHandler.search))
	r.Post("/v1/agent-definitions", authMW.RequireAuthFunc(m.storeHandler.create))
	r.Get("/v1/agent-definitions/{id}", authMW.RequireAuthFunc(m.storeHandler.get))
	r.Put("/v1/agent-definitions/{id}", authMW.RequireAuthFunc(m.storeHandler.update))
	r.Delete("/v1/agent-definitions/{id}", authMW.RequireAuthFunc(m.storeHandler.delete))

	// Agent sharing
	r.Post("/v1/agent-definitions/{id}/share", authMW.RequireAuthFunc(m.storeHandler.shareAgent))
	r.Delete("/v1/agent-definitions/{id}/share", authMW.RequireAuthFunc(m.storeHandler.unshareAgent))
	r.Get("/v1/shared/agents/{token}", m.storeHandler.getSharedAgent) // public — no auth
	r.Post("/v1/shared/agents/{token}/fork", authMW.RequireAuthFunc(m.storeHandler.forkSharedAgent))

	// Agent Catalog (shared templates)
	r.Get("/v1/agent-catalog", authMW.RequireAuthFunc(m.catalogHandler.list))
	r.Get("/v1/agent-catalog/search", authMW.RequireAuthFunc(m.catalogHandler.search))
	r.Post("/v1/agent-catalog/{id}/add", authMW.RequireAuthFunc(m.catalogHandler.addToMyAgents))

	// Backward-compatible workflow routes (redirect to agent definitions)
	r.Get("/v1/workflows", authMW.RequireAuthFunc(m.storeHandler.list))
	r.Post("/v1/workflows", authMW.RequireAuthFunc(m.storeHandler.create))
	r.Get("/v1/workflows/{id}", authMW.RequireAuthFunc(m.storeHandler.get))
	r.Put("/v1/workflows/{id}", authMW.RequireAuthFunc(m.storeHandler.update))
	r.Delete("/v1/workflows/{id}", authMW.RequireAuthFunc(m.storeHandler.delete))
}

// SeedCatalog syncs the agent catalog with the embedded YAML files.
// It upserts all current entries and removes any entries whose YAML file has
// been deleted, keeping the DB in sync with the catalog directory.
func (m *Module) SeedCatalog(ctx context.Context) error {
	entries := seedCatalog()
	if err := m.catalogRepo.Seed(ctx, entries); err != nil {
		return err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return m.catalogRepo.DeleteNotIn(ctx, names)
}

// AgentDefinitionRepo returns the agent definition repository for cross-module access.
func (m *Module) AgentDefinitionRepo() AgentDefinitionRepository {
	return m.agentDefRepo
}
