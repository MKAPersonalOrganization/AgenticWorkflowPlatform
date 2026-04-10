package templates

import (
	"aicompass/modules/agentdefinition"
	"context"
)

// AgentCatalogRepository defines the storage contract for the shared agent catalog.
type AgentCatalogRepository interface {
	// List returns catalog entries with optional category filter, paginated.
	List(ctx context.Context, category string, limit, offset int) ([]agentdefinition.AgentCatalogEntry, int, error)

	// Search searches the catalog by query string with full-text search.
	Search(ctx context.Context, query string, limit, offset int) ([]agentdefinition.AgentCatalogEntry, int, error)

	// GetByID retrieves a single catalog entry.
	GetByID(ctx context.Context, id string) (*agentdefinition.AgentCatalogEntry, error)

	// Categories returns the list of distinct categories.
	Categories(ctx context.Context) ([]string, error)

	// IncrementAddCount atomically increments the add_count for a catalog entry.
	IncrementAddCount(ctx context.Context, id string) error

	// Seed upserts catalog entries (insert or update on name conflict).
	Seed(ctx context.Context, entries []agentdefinition.AgentCatalogEntry) error

	// DeleteNotIn removes catalog entries whose name is not in the provided list.
	// Used after Seed to purge entries whose YAML files have been deleted.
	DeleteNotIn(ctx context.Context, names []string) error
}

// ComposedAgentImporter creates sub-agents and a composed parent from a catalog entry.
type ComposedAgentImporter interface {
	ImportComposedFromCatalogEntry(ctx context.Context, userID string, entry *agentdefinition.AgentCatalogEntry, sourceCatalogID string) (string, error)
}
