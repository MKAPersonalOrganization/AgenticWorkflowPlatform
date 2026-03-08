package skillstore

import (
	"context"

	"school-with-ai/foundation/auth"
	"school-with-ai/foundation/logger"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
)

// Module is the skill-store bounded context. It owns CRUD operations for
// user skills, the shared skill catalog (marketplace), and skill-agent
// permissions.
//
// Mirrors the agentstore module pattern (see arch/adr/017-skills-system.md).
//
// Public API for main.go:
//
//	module := skillstore.New(log, db)
//	module.SeedCatalog(ctx)  // during startup, before routes
//	module.RegisterRoutes(r, authMiddleware)
//
// Cross-module accessors:
//
//	module.UserSkillRepo()       — for plugin access at runtime
//	module.PermissionRepo()      — for plugin permission checks
//	module.Service()             — for the activate_skill tool
type Module struct {
	log               *logger.Logger
	skillHandler      *skillHandler
	catalogHandler    *catalogHandler
	permissionHandler *permissionHandler
	svc               *service
	userSkillRepo     UserSkillRepository
	catalogRepo       SkillCatalogRepository
	permRepo          SkillPermissionRepository
}

// New creates the skill-store module.
func New(log *logger.Logger, db *sqlx.DB) *Module {
	userSkillRepo := newPostgresUserSkillRepository(db)
	catalogRepo := newPostgresSkillCatalogRepository(db)
	permRepo := newPostgresSkillPermissionRepository(db)
	svc := newService(log, userSkillRepo, permRepo)
	sh := newSkillHandler(log, svc)
	ch := newCatalogHandler(log, catalogRepo, userSkillRepo)
	ph := newPermissionHandler(log, svc)

	return &Module{
		log:               log,
		skillHandler:      sh,
		catalogHandler:    ch,
		permissionHandler: ph,
		svc:               svc,
		userSkillRepo:     userSkillRepo,
		catalogRepo:       catalogRepo,
		permRepo:          permRepo,
	}
}

// RegisterRoutes mounts all skill-store HTTP routes.
func (m *Module) RegisterRoutes(r chi.Router, authMW *auth.Middleware) {
	// Skill Catalog routes (shared skill marketplace)
	r.Get("/v1/skill-catalog", authMW.RequireAuthFunc(m.catalogHandler.list))
	r.Get("/v1/skill-catalog/search", authMW.RequireAuthFunc(m.catalogHandler.search))
	r.Post("/v1/skill-catalog/{id}/add", authMW.RequireAuthFunc(m.catalogHandler.addToMySkills))

	// User Skill CRUD routes (personal library)
	r.Get("/v1/skills", authMW.RequireAuthFunc(m.skillHandler.list))
	r.Get("/v1/skills/search", authMW.RequireAuthFunc(m.skillHandler.search))
	r.Post("/v1/skills", authMW.RequireAuthFunc(m.skillHandler.create))
	r.Get("/v1/skills/{id}", authMW.RequireAuthFunc(m.skillHandler.get))
	r.Put("/v1/skills/{id}", authMW.RequireAuthFunc(m.skillHandler.update))
	r.Delete("/v1/skills/{id}", authMW.RequireAuthFunc(m.skillHandler.delete))

	// Skill-Agent Permission routes
	r.Get("/v1/skills/{id}/permissions", authMW.RequireAuthFunc(m.permissionHandler.listBySkill))
	r.Put("/v1/skills/{id}/permissions/{agentId}", authMW.RequireAuthFunc(m.permissionHandler.setPermission))
	r.Get("/v1/agents/{agentId}/skills", authMW.RequireAuthFunc(m.permissionHandler.listByAgent))
}

// SeedCatalog populates the skill catalog with initial entries if the table
// is empty. Should be called once during application startup.
func (m *Module) SeedCatalog(ctx context.Context) error {
	entries := seedCatalog()
	return m.catalogRepo.Seed(ctx, entries)
}

// ─────────────────────────────────────────────────────────────────────────────
// Cross-module accessors
// ─────────────────────────────────────────────────────────────────────────────

// UserSkillRepo returns the user skill repository.
// Used by the skills plugin for runtime skill lookups.
func (m *Module) UserSkillRepo() UserSkillRepository {
	return m.userSkillRepo
}

// PermissionRepo returns the skill permission repository.
// Used by the skills plugin for permission checks.
func (m *Module) PermissionRepo() SkillPermissionRepository {
	return m.permRepo
}

// SkillService returns the service for skill operations.
// Used by the activate_skill tool plugin.
func (m *Module) SkillService() *service {
	return m.svc
}

// ListSkillsForAgent returns the full UserSkill objects that an agent is
// permitted to use. This is the primary API for the activate_skill plugin.
func (m *Module) ListSkillsForAgent(ctx context.Context, userID, agentID string) ([]UserSkill, error) {
	return m.svc.listSkillsForAgent(ctx, userID, agentID)
}

// GetSkillByNameForAgent looks up a skill by name from the set of skills
// the agent is permitted to use.
func (m *Module) GetSkillByNameForAgent(ctx context.Context, userID, agentID, skillName string) (*UserSkill, error) {
	return m.svc.getSkillByNameForAgent(ctx, userID, agentID, skillName)
}
