package agentstore

import (
	"context"
	"time"
)

// AgentCatalogEntry represents a shared, read-only agent template in the agent store.
// These are available to all users and can be "added" (forked) into a user's personal agents.
//
// Entries with a non-empty WorkflowGraphJSON represent multi-agent workflow templates.
// When forked, the system creates the sub-agents and a workflow definition for the user.
type AgentCatalogEntry struct {
	ID           string   `json:"id" db:"id"`
	Name         string   `json:"name" db:"name"`
	Description  string   `json:"description" db:"description"`
	SystemPrompt string   `json:"system_prompt" db:"system_prompt"`
	Category     string   `json:"category" db:"category"`
	Tags         []string `json:"tags" db:"tags"`
	AuthorName   string   `json:"author_name" db:"author_name"`
	Icon         string   `json:"icon" db:"icon"` // SVG icon markup
	Featured     bool     `json:"featured" db:"featured"`
	AddCount     int64    `json:"add_count" db:"add_count"` // Popularity counter

	// ── Workflow Template (optional) ──

	// WorkflowGraphJSON is a JSON-encoded WorkflowNode tree template.
	// Agent nodes use placeholder IDs (e.g. "sub-1") that map to SubAgentsJSON.
	// When empty, this is a single-agent catalog entry.
	WorkflowGraphJSON string `json:"-" db:"workflow_graph"`

	// SubAgentsJSON is a JSON-encoded array of CatalogSubAgent definitions.
	// Each entry maps a placeholder ID to the agent's full definition.
	SubAgentsJSON string `json:"-" db:"sub_agents"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// IsWorkflow returns true if this catalog entry represents a multi-agent workflow.
func (e *AgentCatalogEntry) IsWorkflow() bool {
	return e.WorkflowGraphJSON != "" && e.WorkflowGraphJSON != "null"
}

// CatalogSubAgent defines a sub-agent template embedded in a workflow catalog entry.
// When the catalog entry is forked, each sub-agent becomes a real AgentDefinition.
type CatalogSubAgent struct {
	ID           string `json:"id"`            // Placeholder ID (e.g. "sub-1")
	Name         string `json:"name"`          // Display name
	Description  string `json:"description"`   // Short description
	SystemPrompt string `json:"system_prompt"` // System prompt for the agent
	Icon         string `json:"icon"`          // SVG icon markup
}

// AgentCatalogRepository defines the storage contract for the shared agent catalog.
type AgentCatalogRepository interface {
	// List returns catalog entries with optional category filter, paginated.
	List(ctx context.Context, category string, limit, offset int) ([]AgentCatalogEntry, int, error)

	// Search searches the catalog by query string with full-text search.
	Search(ctx context.Context, query string, limit, offset int) ([]AgentCatalogEntry, int, error)

	// GetByID retrieves a single catalog entry.
	GetByID(ctx context.Context, id string) (*AgentCatalogEntry, error)

	// Categories returns the list of distinct categories.
	Categories(ctx context.Context) ([]string, error)

	// IncrementAddCount atomically increments the add_count for a catalog entry.
	IncrementAddCount(ctx context.Context, id string) error

	// Seed upserts catalog entries (insert or update on name conflict).
	Seed(ctx context.Context, entries []AgentCatalogEntry) error

	// DeleteNotIn removes catalog entries whose name is not in the provided list.
	// Used after Seed to purge entries whose YAML files have been deleted.
	DeleteNotIn(ctx context.Context, names []string) error
}
