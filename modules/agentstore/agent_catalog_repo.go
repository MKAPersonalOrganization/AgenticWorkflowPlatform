package agentstore

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

// postgresAgentCatalogRepository implements domain.AgentCatalogRepository.
type postgresAgentCatalogRepository struct {
db *sqlx.DB
}

func newPostgresAgentCatalogRepository(db *sqlx.DB) *postgresAgentCatalogRepository {
return &postgresAgentCatalogRepository{db: db}
}

func (r *postgresAgentCatalogRepository) List(ctx context.Context, category string, limit, offset int) ([]domain.AgentCatalogEntry, int, error) {
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
SELECT id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count, created_at, updated_at
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
SELECT id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count, created_at, updated_at
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

func (r *postgresAgentCatalogRepository) Search(ctx context.Context, query string, limit, offset int) ([]domain.AgentCatalogEntry, int, error) {
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
SELECT id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count, created_at, updated_at
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

func (r *postgresAgentCatalogRepository) GetByID(ctx context.Context, id string) (*domain.AgentCatalogEntry, error) {
query := `
SELECT id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count, created_at, updated_at
FROM agent_store_catalog
WHERE id = $1
`
var entry domain.AgentCatalogEntry
var tags []string
err := r.db.QueryRowContext(ctx, query, id).Scan(
&entry.ID, &entry.Name, &entry.Description, &entry.SystemPrompt,
&entry.Category, pq.Array(&tags), &entry.AuthorName, &entry.Icon,
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

func (r *postgresAgentCatalogRepository) Categories(ctx context.Context) ([]string, error) {
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

func (r *postgresAgentCatalogRepository) IncrementAddCount(ctx context.Context, id string) error {
_, err := r.db.ExecContext(ctx, `UPDATE agent_store_catalog SET add_count = add_count + 1 WHERE id = $1`, id)
if err != nil {
return fmt.Errorf("incrementing add count: %w", err)
}
return nil
}

func (r *postgresAgentCatalogRepository) Seed(ctx context.Context, entries []domain.AgentCatalogEntry) error {
var count int
if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM agent_store_catalog`).Scan(&count); err != nil {
return fmt.Errorf("checking catalog count: %w", err)
}
if count > 0 {
return nil
}

query := `
INSERT INTO agent_store_catalog (id, name, description, system_prompt, category, tags, author_name, icon, featured, add_count, created_at, updated_at)
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
pq.Array(e.Tags), e.AuthorName, e.Icon, e.Featured, now,
)
if err != nil {
return fmt.Errorf("seeding catalog entry %q: %w", e.Name, err)
}
}
return nil
}

func scanCatalogRows(rows *sql.Rows) ([]domain.AgentCatalogEntry, error) {
var entries []domain.AgentCatalogEntry
for rows.Next() {
var e domain.AgentCatalogEntry
var tags []string
if err := rows.Scan(
&e.ID, &e.Name, &e.Description, &e.SystemPrompt,
&e.Category, pq.Array(&tags), &e.AuthorName, &e.Icon,
&e.Featured, &e.AddCount, &e.CreatedAt, &e.UpdatedAt,
); err != nil {
return nil, fmt.Errorf("scanning catalog entry: %w", err)
}
e.Tags = tags
entries = append(entries, e)
}
return entries, rows.Err()
}
