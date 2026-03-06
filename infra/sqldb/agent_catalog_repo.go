package sqldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"school-with-ai/domain"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// PostgresAgentCatalogRepository implements domain.AgentCatalogRepository.
type PostgresAgentCatalogRepository struct {
	db *sqlx.DB
}

// NewPostgresAgentCatalogRepository creates a new repository.
func NewPostgresAgentCatalogRepository(db *sqlx.DB) *PostgresAgentCatalogRepository {
	return &PostgresAgentCatalogRepository{db: db}
}

// List returns catalog entries with optional category filter, paginated.
func (r *PostgresAgentCatalogRepository) List(ctx context.Context, category string, limit, offset int) ([]domain.AgentCatalogEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var entries []domain.AgentCatalogEntry
	var total int

	if category != "" {
		countQuery := `SELECT COUNT(*) FROM agent_store_catalog WHERE category = $1`
		if err := r.db.QueryRowContext(ctx, countQuery, category).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("counting catalog entries: %w", err)
		}

		query := `
			SELECT id, name, description, system_prompt, category, tags, author_name, icon_label, featured, add_count, created_at, updated_at
			FROM agent_store_catalog
			WHERE category = $1
			ORDER BY featured DESC, add_count DESC, name ASC
			LIMIT $2 OFFSET $3
		`
		rows, err := r.db.QueryContext(ctx, query, category, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("listing catalog entries: %w", err)
		}
		defer rows.Close()
		entries, err = scanCatalogRows(rows)
		if err != nil {
			return nil, 0, err
		}
	} else {
		countQuery := `SELECT COUNT(*) FROM agent_store_catalog`
		if err := r.db.QueryRowContext(ctx, countQuery).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("counting catalog entries: %w", err)
		}

		query := `
			SELECT id, name, description, system_prompt, category, tags, author_name, icon_label, featured, add_count, created_at, updated_at
			FROM agent_store_catalog
			ORDER BY featured DESC, add_count DESC, name ASC
			LIMIT $1 OFFSET $2
		`
		rows, err := r.db.QueryContext(ctx, query, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("listing catalog entries: %w", err)
		}
		defer rows.Close()
		entries, err = scanCatalogRows(rows)
		if err != nil {
			return nil, 0, err
		}
	}

	return entries, total, nil
}

// Search searches the catalog by query string.
func (r *PostgresAgentCatalogRepository) Search(ctx context.Context, query string, limit, offset int) ([]domain.AgentCatalogEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	pattern := "%" + query + "%"

	countQuery := `SELECT COUNT(*) FROM agent_store_catalog WHERE name ILIKE $1 OR description ILIKE $1 OR category ILIKE $1`
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, pattern).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("counting search results: %w", err)
	}

	searchQuery := `
		SELECT id, name, description, system_prompt, category, tags, author_name, icon_label, featured, add_count, created_at, updated_at
		FROM agent_store_catalog
		WHERE name ILIKE $1 OR description ILIKE $1 OR category ILIKE $1
		ORDER BY featured DESC, add_count DESC, name ASC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.db.QueryContext(ctx, searchQuery, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("searching catalog: %w", err)
	}
	defer rows.Close()

	entries, err := scanCatalogRows(rows)
	if err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

// GetByID retrieves a single catalog entry.
func (r *PostgresAgentCatalogRepository) GetByID(ctx context.Context, id string) (*domain.AgentCatalogEntry, error) {
	query := `
		SELECT id, name, description, system_prompt, category, tags, author_name, icon_label, featured, add_count, created_at, updated_at
		FROM agent_store_catalog
		WHERE id = $1
	`
	var entry domain.AgentCatalogEntry
	var tags []string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&entry.ID, &entry.Name, &entry.Description, &entry.SystemPrompt,
		&entry.Category, pq.Array(&tags), &entry.AuthorName, &entry.IconLabel,
		&entry.Featured, &entry.AddCount, &entry.CreatedAt, &entry.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("catalog entry not found: %s", id)
		}
		return nil, fmt.Errorf("getting catalog entry: %w", err)
	}
	entry.Tags = tags
	return &entry, nil
}

// Categories returns the list of distinct categories.
func (r *PostgresAgentCatalogRepository) Categories(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT category FROM agent_store_catalog ORDER BY category`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, fmt.Errorf("scanning category: %w", err)
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

// IncrementAddCount atomically increments the add_count for a catalog entry.
func (r *PostgresAgentCatalogRepository) IncrementAddCount(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE agent_store_catalog SET add_count = add_count + 1 WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("incrementing add count: %w", err)
	}
	return nil
}

// Seed inserts initial catalog entries if the table is empty.
func (r *PostgresAgentCatalogRepository) Seed(ctx context.Context, entries []domain.AgentCatalogEntry) error {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_store_catalog`).Scan(&count); err != nil {
		return fmt.Errorf("checking catalog count: %w", err)
	}
	if count > 0 {
		return nil // Already seeded
	}

	query := `
		INSERT INTO agent_store_catalog (id, name, description, system_prompt, category, tags, author_name, icon_label, featured, add_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, $10, $10)
	`
	now := time.Now()
	for _, e := range entries {
		id := e.ID
		if id == "" {
			id = uuid.New().String()
		}
		_, err := r.db.ExecContext(ctx, query,
			id, e.Name, e.Description, e.SystemPrompt, e.Category,
			pq.Array(e.Tags), e.AuthorName, e.IconLabel, e.Featured, now,
		)
		if err != nil {
			return fmt.Errorf("seeding catalog entry %q: %w", e.Name, err)
		}
	}
	return nil
}

// scanCatalogRows scans multiple catalog rows.
func scanCatalogRows(rows *sql.Rows) ([]domain.AgentCatalogEntry, error) {
	var entries []domain.AgentCatalogEntry
	for rows.Next() {
		var e domain.AgentCatalogEntry
		var tags []string
		if err := rows.Scan(
			&e.ID, &e.Name, &e.Description, &e.SystemPrompt,
			&e.Category, pq.Array(&tags), &e.AuthorName, &e.IconLabel,
			&e.Featured, &e.AddCount, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning catalog entry: %w", err)
		}
		e.Tags = tags
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
