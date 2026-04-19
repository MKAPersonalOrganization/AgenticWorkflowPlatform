package toolreqs

import (
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"

	"github.com/redis/go-redis/v9"
)

// =============================================================================
// Module — standard modular-monolith entry point
// =============================================================================

// Module is the tool requirements module. It owns two peer types with
// cleanly separated responsibilities (see permission_store.go and
// service.go for the rationale):
//
//   - [PermissionStore] owns persistence and Redis caching. Callers that
//     read or write tool-requirement config take the store.
//   - [Service] owns the evaluator pipeline, confirmation, and block
//     middleware. Callers that make runtime decisions take the service.
//
// Both are constructed here so the toolRegistry and repository wiring
// lives in exactly one place.
type Module struct {
	service *Service
	store   *PermissionStore
}

// New creates a new tool requirements module. The tool registry is
// optional; when nil, [Service.IsToolInToolSet] returns false and the
// confirmation scan falls back to allow (fail-open for unknown custom
// tools — see checkConfirmation in middleware.go).
func New(log *logger.Logger, repo Repository, rdb redis.Cmdable, toolRegistry *toolregistry.Registry) *Module {
	store := NewPermissionStore(log, repo, rdb)
	svc := NewService(ServiceConfig{
		Log:          log,
		Store:        store,
		ToolRegistry: toolRegistry,
	})
	return &Module{service: svc, store: store}
}

// Service returns the evaluation / confirmation / middleware surface
// — the thing the adk-go adapter depends on.
func (m *Module) Service() *Service {
	return m.service
}

// Store returns the persistence surface — the thing HTTP handlers that
// read/write requirement configs depend on.
func (m *Module) Store() *PermissionStore {
	return m.store
}
