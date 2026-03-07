package resources

import (
	"context"
	"io"

	"school-with-ai/foundation/auth"
	"school-with-ai/services/artifact"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// Module is the resources bounded context. It owns the repository, service,
// and handler as internal collaborators, exposing only the wiring methods that
// main.go (and other modules) need.
//
// Extracted as part of the modular-monolith migration
// (see arch/adr/012-modular-monolith-migration.md). It replaces three former
// packages: handlers/resources, services/resources, and
// infra/sqldb/resources_repo.go.
//
// Public API for main.go:
//
// module := resources.New(db, artifactService)
// module.RegisterRoutes(r, authMiddleware)
//
// Cross-module queries:
//
// module.ListResources(ctx, userID, source) — used by resourcepermissions handler
// module.GetResourceContent(ctx, id) — used by resourcepermissions handler
type Module struct {
	handler *handler
	svc     *service
}

// New creates the resources module.
func New(db *sqlx.DB, artifactService artifact.Service) *Module {
	repo := newPostgresResourcesRepository(db)
	svc := newService(repo, artifactService)
	h := newHandler(svc)

	return &Module{handler: h, svc: svc}
}

// RegisterRoutes mounts all resource HTTP routes onto the given chi.Router.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/api/resources", authMW.RequireAuthFunc(m.handler.list))
	r.Post("/api/resources/upload", authMW.RequireAuthFunc(m.handler.upload))
	r.Get("/api/resources/{id}/content", authMW.RequireAuthFunc(m.handler.getContent))
	r.Delete("/api/resources/{id}", authMW.RequireAuthFunc(m.handler.delete))
}

// ─────────────────────────────────────────────────────────────────────────────
// Cross-module queries
// ─────────────────────────────────────────────────────────────────────────────

// ListResources returns resources for a user and source.
// Used by the resourcepermissions handler to build agent resource lists.
func (m *Module) ListResources(ctx context.Context, userID string, source ResourceSource) ([]Resource, error) {
	return m.svc.listResources(ctx, userID, source)
}

// GetResourceContent returns a reader for the resource content from S3.
// Used by the resourcepermissions handler.
func (m *Module) GetResourceContent(ctx context.Context, id uuid.UUID) (Resource, io.ReadCloser, error) {
	return m.svc.getResourceContent(ctx, id)
}
