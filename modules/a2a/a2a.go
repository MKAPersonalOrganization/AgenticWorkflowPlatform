// Package a2a is the A2A (Agent-to-Agent) connector bounded context. It owns
// the full lifecycle of A2A connectors: creation, agent card discovery, skill
// extraction, and integration with the tool registry.
//
// This module follows the same architectural patterns as the MCP module
// (see modules/mcp/), but for the A2A protocol instead of MCP. A2A connectors
// allow users to connect to remote AI agents exposed via the A2A protocol
// (https://github.com/a2aproject/a2a-go) and wire their skills to any local
// agent through the tool registry and permission system.
//
// Public API for main.go:
//
//	module := a2a.New(log, db, toolRegistry)
//	module.RegisterRoutes(r, authMiddleware)
//
// Cross-module accessor:
//
//	module.Service() — returns *Service for A2A tool resolution during agent execution
package a2a

import (
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"aicompass/platform/toolregistry"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the A2A connector bounded context.
type Module struct {
	log     *logger.Logger
	handler *handler
	svc     *Service
}

// New creates the A2A module.
func New(log *logger.Logger, db *sqlx.DB, toolRegistry *toolregistry.Registry) *Module {
	repo := newPostgresA2AConnectorRepository(db)
	svc := NewService(log, repo, toolRegistry)
	h := newHandler(log, svc)

	return &Module{
		log:     log,
		handler: h,
		svc:     svc,
	}
}

// RegisterRoutes mounts all A2A connector HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/v1/a2a/connectors", authMW.RequireAuthFunc(m.handler.listConnectors))
	r.Post("/v1/a2a/connectors", authMW.RequireAuthFunc(m.handler.createConnector))
	r.Get("/v1/a2a/connectors/{id}", authMW.RequireAuthFunc(m.handler.getConnector))
	r.Delete("/v1/a2a/connectors/{id}", authMW.RequireAuthFunc(m.handler.deleteConnector))
	r.Post("/v1/a2a/connectors/{id}/connect", authMW.RequireAuthFunc(m.handler.connect))
	r.Post("/v1/a2a/connectors/{id}/disconnect", authMW.RequireAuthFunc(m.handler.disconnect))
	r.Get("/v1/a2a/connectors/{id}/skills", authMW.RequireAuthFunc(m.handler.listSkills))
	r.Post("/v1/a2a/connectors/{id}/refresh", authMW.RequireAuthFunc(m.handler.refreshSkills))
}

// Service returns the A2A service for cross-module usage.
// Used by the custom runner adapter to resolve A2A tools during agent execution.
func (m *Module) Service() *Service {
	return m.svc
}

// Shutdown stops background goroutines. Call during graceful shutdown.
func (m *Module) Shutdown() {
	m.svc.Shutdown()
}
