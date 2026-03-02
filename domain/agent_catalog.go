package domain

import (
	"context"
	"time"
)

// AgentCatalogEntry represents a shared, read-only agent template in the agent store.
// These are available to all users and can be "added" (forked) into a user's personal agents.
type AgentCatalogEntry struct {
	ID           string    `json:"id" db:"id"`
	Name         string    `json:"name" db:"name"`
	Description  string    `json:"description" db:"description"`
	SystemPrompt string    `json:"system_prompt" db:"system_prompt"`
	Category     string    `json:"category" db:"category"`
	Tags         []string  `json:"tags" db:"tags"`
	AuthorName   string    `json:"author_name" db:"author_name"`
	IconLabel    string    `json:"icon_label" db:"icon_label"` // Emoji or short label for display
	Featured     bool      `json:"featured" db:"featured"`
	AddCount     int64     `json:"add_count" db:"add_count"` // Popularity counter
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
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

	// Seed inserts initial catalog entries if the table is empty.
	Seed(ctx context.Context, entries []AgentCatalogEntry) error
}
