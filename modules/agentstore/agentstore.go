package agentstore

import (
"context"

"school-with-ai/domain"
"school-with-ai/foundation/logger"
"school-with-ai/handlers/auth"

"github.com/go-chi/chi/v5"
"github.com/jmoiron/sqlx"
)

// Module is the agent-store bounded context. It owns CRUD operations for
// user-created agent definitions and the shared agent catalog.
//
// Extracted as part of the modular-monolith migration
// (see arch/adr/012-modular-monolith-migration.md). It replaces four former
// packages: handlers/agentstore, handlers/agentcatalog, services/agentstore,
// and the corresponding infra repos (agent_definition_repo.go,
// agent_catalog_repo.go).
//
// Domain types (AgentDefinition, AgentCatalogEntry, and their repository
// interfaces) remain in the shared domain/ package for Phase 1.
//
// Public API for main.go:
//
//module := agentstore.New(log, db)
//module.SeedCatalog(ctx)  // during startup, before routes
//module.RegisterRoutes(r, authMiddleware)
//
// Cross-module accessor:
//
//module.AgentDefinitionRepo() — returns domain.AgentDefinitionRepository
//    for custom.NewUserAwareAgentProvider (shared dependency)
type Module struct {
log            *logger.Logger
storeHandler   *storeHandler
catalogHandler *catalogHandler
svc            *service
agentDefRepo   domain.AgentDefinitionRepository
catalogRepo    domain.AgentCatalogRepository
}

// New creates the agent-store module.
func New(log *logger.Logger, db *sqlx.DB) *Module {
agentDefRepo := newPostgresAgentDefinitionRepository(db)
catalogRepo := newPostgresAgentCatalogRepository(db)
svc := newService(log, agentDefRepo)
sh := newStoreHandler(log, svc)
ch := newCatalogHandler(log, catalogRepo, agentDefRepo)

return &Module{
log:            log,
storeHandler:   sh,
catalogHandler: ch,
svc:            svc,
agentDefRepo:   agentDefRepo,
catalogRepo:    catalogRepo,
}
}

// RegisterRoutes mounts all agent-store and agent-catalog HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
// Agent definition CRUD routes (user-created agents)
r.Get("/v1/agent-definitions", authMW.RequireAuthFunc(m.storeHandler.list))
r.Get("/v1/agent-definitions/search", authMW.RequireAuthFunc(m.storeHandler.search))
r.Post("/v1/agent-definitions", authMW.RequireAuthFunc(m.storeHandler.create))
r.Get("/v1/agent-definitions/{id}", authMW.RequireAuthFunc(m.storeHandler.get))
r.Put("/v1/agent-definitions/{id}", authMW.RequireAuthFunc(m.storeHandler.update))
r.Delete("/v1/agent-definitions/{id}", authMW.RequireAuthFunc(m.storeHandler.delete))

// Agent Catalog routes (shared agent store)
r.Get("/v1/agent-catalog", authMW.RequireAuthFunc(m.catalogHandler.list))
r.Get("/v1/agent-catalog/search", authMW.RequireAuthFunc(m.catalogHandler.search))
r.Post("/v1/agent-catalog/{id}/add", authMW.RequireAuthFunc(m.catalogHandler.addToMyAgents))
}

// SeedCatalog populates the agent catalog with initial entries if the table
// is empty. Should be called once during application startup.
func (m *Module) SeedCatalog(ctx context.Context) error {
entries := seedCatalog()
return m.catalogRepo.Seed(ctx, entries)
}

// ─────────────────────────────────────────────────────────────────────────────
// Cross-module accessors
// ─────────────────────────────────────────────────────────────────────────────

// AgentDefinitionRepo returns the agent definition repository.
// Used by custom.NewUserAwareAgentProvider which needs direct repo access
// for agent resolution at runtime.
func (m *Module) AgentDefinitionRepo() domain.AgentDefinitionRepository {
return m.agentDefRepo
}
