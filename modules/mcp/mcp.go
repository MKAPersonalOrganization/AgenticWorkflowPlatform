// Package mcp is the MCP connector bounded context. It owns the full lifecycle
// of MCP (Model Context Protocol) connectors: creation, OAuth authentication,
// tool discovery, proxying, and integration with the tool registry.
//
// Extracted as part of the modular-monolith migration
// (see arch/adr/012-modular-monolith-migration.md). It replaces three former
// packages: handlers/mcp, services/plugindiscovery, and the MCP connector
// repository from infra/sqldb/mcp_connector_repo.go.
//
// Domain types (MCPConnector, MCPDiscoveredTool, ConnectorStatus, and
// MCPConnectorRepository) remain in the shared domain/ package for Phase 1.
//
// Public API for main.go:
//
// module := mcp.New(log, db, callbackURL, toolRegistry)
// module.RegisterRoutes(r, authMiddleware)
//
// Cross-module accessor:
//
// module.Service() — returns *Service for the custom runner adapter
//
//	(shared dependency for MCP tool resolution during agent execution)
package mcp

import (
	"school-with-ai/foundation/auth"
	"school-with-ai/foundation/logger"
	"school-with-ai/platform/toolregistry"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the MCP connector bounded context.
type Module struct {
	log     *logger.Logger
	handler *handler
	svc     *Service
}

// New creates the MCP module.
func New(log *logger.Logger, db *sqlx.DB, callbackURL string, toolRegistry *toolregistry.Registry) *Module {
	repo := newPostgresMCPConnectorRepository(db)
	svc := NewService(log, repo, callbackURL, toolRegistry)
	h := newHandler(log, svc)

	return &Module{
		log:     log,
		handler: h,
		svc:     svc,
	}
}

// RegisterRoutes mounts all MCP connector HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	// MCP Connector endpoints
	r.Get("/v1/mcp/connectors", authMW.RequireAuthFunc(m.handler.listConnectors))
	r.Get("/v1/mcp/connectors/presets", m.handler.listPresets) // Public — no auth needed for preset catalog
	r.Post("/v1/mcp/connectors", authMW.RequireAuthFunc(m.handler.createConnector))
	r.Get("/v1/mcp/connectors/{id}", authMW.RequireAuthFunc(m.handler.getConnector))
	r.Delete("/v1/mcp/connectors/{id}", authMW.RequireAuthFunc(m.handler.deleteConnector))
	r.Post("/v1/mcp/connectors/{id}/connect", authMW.RequireAuthFunc(m.handler.connect))
	r.Post("/v1/mcp/connectors/{id}/disconnect", authMW.RequireAuthFunc(m.handler.disconnect))
	r.Get("/v1/mcp/connectors/{id}/tools", authMW.RequireAuthFunc(m.handler.listTools))
	r.Post("/v1/mcp/connectors/{id}/refresh", authMW.RequireAuthFunc(m.handler.refreshTools))
	// MCP Proxy — authenticated reverse proxy to the connector's MCP server (agentstack pattern).
	// Supports all HTTP methods so external MCP clients can use connectors directly.
	r.HandleFunc("/v1/mcp/connectors/{id}/mcp", authMW.RequireAuthFunc(m.handler.mcpProxy))
	r.HandleFunc("/v1/mcp/connectors/{id}/mcp/*", authMW.RequireAuthFunc(m.handler.mcpProxy))
	// OAuth callback (public — called by external OAuth provider redirect)
	r.Get("/v1/mcp/oauth/callback", m.handler.oauthCallback)
}

// Service returns the MCP service for cross-module usage.
// Used by the custom runner adapter to resolve MCP tools during agent execution.
func (m *Module) Service() *Service {
	return m.svc
}
