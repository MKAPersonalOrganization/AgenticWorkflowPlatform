package skillstore

import (
	"context"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────────
// Skill Catalog (shared marketplace)
// ─────────────────────────────────────────────────────────────────────────────

// SkillCatalogEntry represents a shared, read-only skill template in the skill marketplace.
// These are available to all users and can be "added" (imported) into a user's personal skill library.
type SkillCatalogEntry struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	Instructions string    `json:"instructions" db:"instructions"`
	Category     string    `json:"category" db:"category"`
	Tags         []string  `json:"tags" db:"tags"`
	AuthorName   string    `json:"author_name" db:"author_name"`
	Icon         string    `json:"icon" db:"icon"` // SVG icon markup
	Featured     bool      `json:"featured" db:"featured"`
	AddCount     int64     `json:"add_count" db:"add_count"` // Popularity counter
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

// SkillCatalogRepository defines the storage contract for the shared skill catalog.
type SkillCatalogRepository interface {
	// List returns catalog entries with optional category filter, paginated.
	List(ctx context.Context, category string, limit, offset int) ([]SkillCatalogEntry, int, error)
	// Search searches the catalog by query string with full-text search.
	Search(ctx context.Context, query string, limit, offset int) ([]SkillCatalogEntry, int, error)
	// GetByID retrieves a single catalog entry.
	GetByID(ctx context.Context, id string) (*SkillCatalogEntry, error)
	// Categories returns the list of distinct categories.
	Categories(ctx context.Context) ([]string, error)
	// IncrementAddCount atomically increments the add_count for a catalog entry.
	IncrementAddCount(ctx context.Context, id string) error
	// Seed inserts initial catalog entries if the table is empty.
	Seed(ctx context.Context, entries []SkillCatalogEntry) error
}

// ─────────────────────────────────────────────────────────────────────────────
// User Skills (per-user library: imported + custom)
// ─────────────────────────────────────────────────────────────────────────────

// UserSkill represents a skill in a user's personal library.
// It may have been imported from the catalog (source_catalog_id != "")
// or created as a custom skill (is_custom == true).
type UserSkill struct {
	ID              string    `json:"id" db:"id"`
	UserID          string    `json:"user_id" db:"user_id"`
	Name            string    `json:"name" db:"name"`
	Description     string    `json:"description" db:"description"`
	Instructions    string    `json:"instructions" db:"instructions"`
	Category        string    `json:"category" db:"category"`
	Icon            string    `json:"icon" db:"icon"`
	IsCustom        bool      `json:"is_custom" db:"is_custom"`
	SourceCatalogID string    `json:"source_catalog_id,omitempty" db:"source_catalog_id"`
	Resources       string    `json:"resources" db:"resources"` // JSONB string
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// UserSkillRepository defines the storage contract for per-user skills.
type UserSkillRepository interface {
	// Create persists a new user skill.
	Create(ctx context.Context, skill *UserSkill) error
	// Update modifies an existing user skill (must be owned by user).
	Update(ctx context.Context, skill *UserSkill) error
	// Delete removes a user skill by ID (must be owned by user).
	Delete(ctx context.Context, id, userID string) error
	// GetByID retrieves a single skill by ID (must be owned by user).
	GetByID(ctx context.Context, id, userID string) (*UserSkill, error)
	// GetByIDs retrieves multiple skills by their IDs in a single query (must be owned by user).
	// Missing or deleted skills are silently omitted from the result.
	GetByIDs(ctx context.Context, ids []string, userID string) ([]UserSkill, error)
	// GetByName retrieves a skill by name (must be owned by user).
	GetByName(ctx context.Context, userID, name string) (*UserSkill, error)
	// ListByUser returns all skills for a specific user.
	ListByUser(ctx context.Context, userID string) ([]UserSkill, error)
	// Search finds skills matching a query string within a user's library.
	Search(ctx context.Context, userID, query string) ([]UserSkill, error)
	// ListAddedCatalogIDs returns the set of catalog entry IDs that the
	// user has already imported into their personal library.
	ListAddedCatalogIDs(ctx context.Context, userID string) ([]string, error)
	// ExistsByCatalogID checks whether the user already has a skill
	// imported from the given catalog entry.
	ExistsByCatalogID(ctx context.Context, userID, catalogID string) (bool, error)
	// CountByUser returns the total number of skills for a user.
	CountByUser(ctx context.Context, userID string) (int, error)
}

// ─────────────────────────────────────────────────────────────────────────────
// Skill-Agent Permissions
// ─────────────────────────────────────────────────────────────────────────────

// SkillAgentPermission represents a grant for an agent to access a specific skill.
// Scoped by (user_id, agent_id, skill_id) for full multi-tenant isolation.
type SkillAgentPermission struct {
	UserID    string    `json:"user_id" db:"user_id"`
	AgentID   string    `json:"agent_id" db:"agent_id"`
	SkillID   string    `json:"skill_id" db:"skill_id"`
	Enabled   bool      `json:"enabled" db:"enabled"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// SkillPermissionRepository defines the storage contract for skill-agent permissions.
type SkillPermissionRepository interface {
	// Grant enables an agent to access a skill for a specific user.
	Grant(ctx context.Context, perm *SkillAgentPermission) error
	// Revoke removes an agent's access to a skill.
	Revoke(ctx context.Context, userID, agentID, skillID string) error
	// ListBySkill returns all permission grants for a skill (within a user's scope).
	ListBySkill(ctx context.Context, userID, skillID string) ([]SkillAgentPermission, error)
	// ListByAgent returns all skill permissions for a specific agent (within a user's scope).
	ListByAgent(ctx context.Context, userID, agentID string) ([]SkillAgentPermission, error)
	// IsGranted checks if an agent has permission to use a specific skill.
	IsGranted(ctx context.Context, userID, agentID, skillID string) (bool, error)
	// ListGrantedSkillIDs returns the IDs of skills an agent is permitted to use.
	ListGrantedSkillIDs(ctx context.Context, userID, agentID string) ([]string, error)
}
