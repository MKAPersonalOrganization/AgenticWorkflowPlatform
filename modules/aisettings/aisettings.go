package aisettings

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/handlers/auth"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the AI-settings bounded context. It owns repositories, service,
// and handler as internal collaborators, exposing only the wiring methods that
// main.go (and other modules) need.
//
// Extracted as part of the modular-monolith migration
// (see arch/adr/012-modular-monolith-migration.md). It replaces four former
// packages: handlers/aisettings, services/aisettings,
// infra/sqldb/ai_settings_repo.go, and infra/sqldb/ai_capability_repo.go.
//
// Domain types (AISettings, AICapabilityConfig, AIProvider, etc.) remain in
// the shared domain/ package for Phase 1. Phase 2 will move them here.
//
// Public API for main.go:
//
// module := aisettings.New(log, db)
// module.RegisterRoutes(r, authMiddleware)
//
// Cross-module queries:
//
// module.GetModelConfigForUser(ctx, userID)  — satisfies custom.ModelConfigResolver
// module.GetTTSConfigForUserFlat(ctx, userID) — satisfies custom.TTSConfigResolver
// module.GetEmbeddingConfigForUser(ctx, userID) — used by knowledge handler
type Module struct {
	handler *handler
	svc     *service
}

// New creates the AI-settings module.
func New(log *logger.Logger, db *sqlx.DB) *Module {
	settingsRepo := newPostgresAISettingsRepository(db)
	capabilityRepo := newPostgresAICapabilityConfigRepository(db)
	svc := newService(settingsRepo, capabilityRepo)
	h := newHandler(log, svc)

	return &Module{handler: h, svc: svc}
}

// RegisterRoutes mounts all AI-settings HTTP routes onto the given chi.Router.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	r.Get("/api/ai-settings/providers", authMW.RequireAuthFunc(m.handler.listProviders))
	r.Get("/api/ai-settings/tts-voices", authMW.RequireAuthFunc(m.handler.getTTSVoices))
	r.Get("/api/ai-settings", authMW.RequireAuthFunc(m.handler.getSettings))
	r.Put("/api/ai-settings", authMW.RequireAuthFunc(m.handler.saveSettings))
	r.Delete("/api/ai-settings/{id}", authMW.RequireAuthFunc(m.handler.deleteSettings))
	r.Put("/api/ai-settings/{id}/default", authMW.RequireAuthFunc(m.handler.setDefault))
	// Capability config routes (decoupled per-capability settings)
	r.Get("/api/ai-settings/capabilities", authMW.RequireAuthFunc(m.handler.getCapabilityConfigs))
	r.Put("/api/ai-settings/capabilities/{capability}", authMW.RequireAuthFunc(m.handler.saveCapabilityConfig))
	r.Delete("/api/ai-settings/capabilities/{capability}", authMW.RequireAuthFunc(m.handler.deleteCapabilityConfig))
}

// ─────────────────────────────────────────────────────────────────────────────
// Cross-module queries
// ─────────────────────────────────────────────────────────────────────────────

// GetModelConfigForUser resolves the active LLM ModelConfig for a given user.
// Satisfies the custom.ModelConfigResolver interface used by the runner adapter.
func (m *Module) GetModelConfigForUser(ctx context.Context, userID string) (*agent.ModelConfig, error) {
	return m.svc.getModelConfigForUser(ctx, userID)
}

// GetTTSConfigForUserFlat returns TTS config as flat values.
// Satisfies the custom.TTSConfigResolver interface used by the runner adapter.
func (m *Module) GetTTSConfigForUserFlat(ctx context.Context, userID string) (apiKey, model, voice string, err error) {
	return m.svc.getTTSConfigForUserFlat(ctx, userID)
}

// GetEmbeddingConfigForUser resolves the user's embedding configuration.
// Used by the knowledge handler for workspace creation.
func (m *Module) GetEmbeddingConfigForUser(ctx context.Context, userID string) (*EmbeddingConfig, error) {
	return m.svc.getEmbeddingConfigForUser(ctx, userID)
}
