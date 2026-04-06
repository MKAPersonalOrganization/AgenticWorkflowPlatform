package toolreqs

import (
	"aicompass/foundation/logger"

	"github.com/redis/go-redis/v9"
)

// =============================================================================
// Module — standard modular-monolith entry point
// =============================================================================

// Module is the tool requirements module entry point.
// It follows the same pattern as other modules in the system
// (two-phase construction: New → Service).
type Module struct {
	service *Service
}

// New creates a new tool requirements module.
//
// The module owns all permission persistence (PostgreSQL repository + Redis
// caching) and all requirement evaluation (permissions, confirmation, etc.).
func New(log *logger.Logger, repo Repository, rdb redis.Cmdable) *Module {
	svc := NewService(ServiceConfig{
		Log:  log,
		Repo: repo,
		RDB:  rdb,
	})
	return &Module{service: svc}
}

// Service returns the tool requirements service for dependency injection
// into other modules (adapter, chat, etc.).
func (m *Module) Service() *Service {
	return m.service
}
