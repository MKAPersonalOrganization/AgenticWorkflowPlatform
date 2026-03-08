package pluginsettings

import (
	"context"

	"school-with-ai/foundation/auth"
	"school-with-ai/foundation/logger"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the plugin-settings bounded context.
// It owns the repository, service, and handler — exposing only the wiring
// methods that main.go (and the adapter layer) need.
type Module struct {
	svc     *Service
	handler *handler
}

// New creates the plugin-settings module.
func New(log *logger.Logger, db *sqlx.DB) *Module {
	repo := NewPostgresRepository(db)
	svc := NewService(repo)
	h := newHandler(log, svc)
	return &Module{svc: svc, handler: h}
}

// RegisterRoutes mounts plugin-settings HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/api/plugin-settings/schemas", authMW.RequireAuthFunc(m.handler.listSchemas))
	r.Get("/api/plugin-settings", authMW.RequireAuthFunc(m.handler.getAll))
	r.Get("/api/plugin-settings/{pluginID}", authMW.RequireAuthFunc(m.handler.getOne))
	r.Put("/api/plugin-settings/{pluginID}", authMW.RequireAuthFunc(m.handler.save))
	r.Delete("/api/plugin-settings/{pluginID}", authMW.RequireAuthFunc(m.handler.remove))
}

// Service exposes the underlying service for cross-module use (e.g. the runner adapter).
func (m *Module) Service() *Service {
	return m.svc
}

// ─── PluginSettingsResolver ─────────────────────────────────────────────────
// Satisfies the adapter-layer interface without importing the adapter package.

// GetAllPluginSettingsForUser returns all plugin settings as a map for a user.
// This implements the adapter's PluginSettingsResolver interface.
func (m *Module) GetAllPluginSettingsForUser(ctx context.Context, userID string) (map[string]map[string]string, error) {
	return m.svc.GetAllForUserAsMap(ctx, userID)
}
