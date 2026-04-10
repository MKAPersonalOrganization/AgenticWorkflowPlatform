package templates

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"aicompass/modules/agentdefinition"
	"context"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the template-catalog bounded context.
//
// It owns:
//   - Browsable, read-only catalog of agent templates
//   - Catalog search by query string
//   - "Add to my agents" (forking catalog entries into user's agents)
//   - YAML-based catalog seeding from embedded files
//
// Catalog entries are shared, read-only templates available to all users.
// When a user adds a template, it creates a copy in their personal agent store
// via the agentdefinition module.
type Module struct {
	log            *logger.Logger
	catalogHandler *catalogHandler
	catalogRepo    AgentCatalogRepository
}

// New creates the template-catalog module.
func New(
	log *logger.Logger,
	db *sqlx.DB,
	agentRepo agentdefinition.AgentDefinitionRepository,
	importer ComposedAgentImporter,
) *Module {
	catalogRepo := newPostgresAgentCatalogRepository(db)
	h := newCatalogHandler(log, catalogRepo, agentRepo, importer)

	return &Module{
		log:            log,
		catalogHandler: h,
		catalogRepo:    catalogRepo,
	}
}

// RegisterRoutes mounts all template-catalog HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	// Agent Catalog (shared templates)
	r.Get("/v1/agent-catalog", authMW.RequireAuthFunc(m.catalogHandler.list))
	r.Get("/v1/agent-catalog/search", authMW.RequireAuthFunc(m.catalogHandler.search))
	r.Post("/v1/agent-catalog/{id}/add", authMW.RequireAuthFunc(m.catalogHandler.addToMyAgents))
}

// SeedCatalog syncs the catalog with the embedded YAML files.
// It upserts all current entries and removes any entries whose YAML file has
// been deleted, keeping the DB in sync with the catalog directory.
func (m *Module) SeedCatalog(ctx context.Context) error {
	entries := SeedCatalog()
	if err := m.catalogRepo.Seed(ctx, entries); err != nil {
		return err
	}
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name
	}
	return m.catalogRepo.DeleteNotIn(ctx, names)
}
