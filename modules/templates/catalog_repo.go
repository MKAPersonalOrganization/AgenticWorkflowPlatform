package templates

import (
	"aicompass/modules/agentdefinition"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// postgresAgentCatalogRepository implements agentdefinition.AgentCatalogRepository.
type postgresAgentCatalogRepository struct {
	db *sqlx.DB
}

func newPostgresAgentCatalogRepository(db *sqlx.DB) *postgresAgentCatalogRepository {
	return &postgresAgentCatalogRepository{db: db}
}

func (r *postgresAgentCatalogRepository) List(ctx context.Context, category string, limit, offset int) ([]agentdefinition.AgentCatalogEntry, int, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	var entries []agentdefinition.AgentCatalogEntry
	var total int

	if category != "" {
		countQuery := `SELECT COUNT(*) FROM agent_store_catalog WHERE category = $1`
		if err := r.db.QueryRowContext(ctx, countQuery, category).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("counting catalog entries: %w", err)
		}

		query := `
SELECT id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count,
       COALESCE(workflow_graph::text, '') as workflow_graph,
       COALESCE(sub_agents::text, '') as sub_agents,
       created_at, updated_at
FROM agent_store_catalog
WHERE category = $1
ORDER BY featured DESC, add_count DESC, name ASC
LIMIT $2 OFFSET $3
`
		rows, err := r.db.QueryContext(ctx, query, category, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("listing catalog entries: %w", err)
		}
		defer func() { _ = rows.Close() }()
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
SELECT id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count,
       COALESCE(workflow_graph::text, '') as workflow_graph,
       COALESCE(sub_agents::text, '') as sub_agents,
       created_at, updated_at
FROM agent_store_catalog
ORDER BY featured DESC, add_count DESC, name ASC
LIMIT $1 OFFSET $2
`
		rows, err := r.db.QueryContext(ctx, query, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("listing catalog entries: %w", err)
		}
		defer func() { _ = rows.Close() }()
		entries, err = scanCatalogRows(rows)
		if err != nil {
			return nil, 0, err
		}
	}

	return entries, total, nil
}

func (r *postgresAgentCatalogRepository) Search(ctx context.Context, query string, limit, offset int) ([]agentdefinition.AgentCatalogEntry, int, error) {
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
SELECT id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count,
       COALESCE(workflow_graph::text, '') as workflow_graph,
       COALESCE(sub_agents::text, '') as sub_agents,
       created_at, updated_at
FROM agent_store_catalog
WHERE name ILIKE $1 OR description ILIKE $1 OR category ILIKE $1
ORDER BY featured DESC, add_count DESC, name ASC
LIMIT $2 OFFSET $3
`
	rows, err := r.db.QueryContext(ctx, searchQuery, pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("searching catalog: %w", err)
	}
	defer func() { _ = rows.Close() }()

	entries, err := scanCatalogRows(rows)
	if err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

func (r *postgresAgentCatalogRepository) GetByID(ctx context.Context, id string) (*agentdefinition.AgentCatalogEntry, error) {
	query := `
SELECT id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count,
       COALESCE(workflow_graph::text, '') as workflow_graph,
       COALESCE(sub_agents::text, '') as sub_agents,
       created_at, updated_at
FROM agent_store_catalog
WHERE id = $1
`
	var entry agentdefinition.AgentCatalogEntry
	var tags []string
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&entry.ID, &entry.Name, &entry.Description, &entry.SystemPrompt,
		&entry.Category, pq.Array(&tags), &entry.AuthorName, &entry.Icon,
		&entry.Featured, &entry.AddCount,
		&entry.WorkflowGraphJSON, &entry.LLMAgentsJSON,
		&entry.CreatedAt, &entry.UpdatedAt,
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

func (r *postgresAgentCatalogRepository) Categories(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT category FROM agent_store_catalog ORDER BY category`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *postgresAgentCatalogRepository) IncrementAddCount(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE agent_store_catalog SET add_count = add_count + 1 WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("incrementing add count: %w", err)
	}
	return nil
}

func (r *postgresAgentCatalogRepository) Seed(ctx context.Context, entries []agentdefinition.AgentCatalogEntry) error {
	// ON CONFLICT (name) DO UPDATE: every field is overwritten so that editing a
	// YAML catalog file and redeploying immediately reflects the change in the DB.
	// add_count and created_at are preserved to avoid resetting popularity counters.
	query := `
INSERT INTO agent_store_catalog (id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count, workflow_graph, sub_agents, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 0, $10, $11, $12, $12)
ON CONFLICT (name) DO UPDATE SET
  id             = EXCLUDED.id,
  description    = EXCLUDED.description,
  system_prompt  = EXCLUDED.system_prompt,
  category       = EXCLUDED.category,
  tags           = EXCLUDED.tags,
  author_name    = EXCLUDED.author_name,
  icon           = EXCLUDED.icon,
  featured       = EXCLUDED.featured,
  workflow_graph = EXCLUDED.workflow_graph,
  sub_agents     = EXCLUDED.sub_agents,
  updated_at     = EXCLUDED.updated_at
`
	now := time.Now()
	for _, e := range entries {
		id := e.ID
		if id == "" {
			id = uuid.New().String()
		}
		var wfGraph, llmAgents interface{}
		if e.WorkflowGraphJSON != "" {
			wfGraph = e.WorkflowGraphJSON
		}
		if e.LLMAgentsJSON != "" {
			llmAgents = e.LLMAgentsJSON
		}
		_, err := r.db.ExecContext(ctx, query,
			id, e.Name, e.Description, e.SystemPrompt, e.Category,
			pq.Array(e.Tags), e.AuthorName, e.Icon, e.Featured,
			wfGraph, llmAgents, now,
		)
		if err != nil {
			return fmt.Errorf("seeding catalog entry %q: %w", e.Name, err)
		}
	}
	return nil
}

func (r *postgresAgentCatalogRepository) DeleteNotIn(ctx context.Context, names []string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM agent_store_catalog WHERE name != ALL($1)`,
		pq.Array(names),
	)
	if err != nil {
		return fmt.Errorf("deleting stale catalog entries: %w", err)
	}
	return nil
}

func scanCatalogRows(rows *sql.Rows) ([]agentdefinition.AgentCatalogEntry, error) {
	var entries []agentdefinition.AgentCatalogEntry
	for rows.Next() {
		var e agentdefinition.AgentCatalogEntry
		var tags []string
		if err := rows.Scan(
			&e.ID, &e.Name, &e.Description, &e.SystemPrompt,
			&e.Category, pq.Array(&tags), &e.AuthorName, &e.Icon,
			&e.Featured, &e.AddCount,
			&e.WorkflowGraphJSON, &e.LLMAgentsJSON,
			&e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning catalog entry: %w", err)
		}
		e.Tags = tags
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
