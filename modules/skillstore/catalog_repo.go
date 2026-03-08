package skillstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// ─────────────────────────────────────────────────────────────────────────────
// Skill Catalog Repository (PostgreSQL)
// ─────────────────────────────────────────────────────────────────────────────

type postgresSkillCatalogRepository struct {
	db *sqlx.DB
}

func newPostgresSkillCatalogRepository(db *sqlx.DB) *postgresSkillCatalogRepository {
	return &postgresSkillCatalogRepository{db: db}
}

func (r *postgresSkillCatalogRepository) List(ctx context.Context, category string, limit, offset int) ([]SkillCatalogEntry, int, error) {
	var entries []SkillCatalogEntry
	var total int

	if category != "" {
		err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM skill_catalog WHERE category = $1`, category)
		if err != nil {
			return nil, 0, fmt.Errorf("counting skill catalog entries: %w", err)
		}
		rows, err := r.db.QueryxContext(ctx, `
			SELECT id, name, description, instructions, category, tags, author_name, icon, featured, add_count, created_at, updated_at
			FROM skill_catalog WHERE category = $1 ORDER BY featured DESC, add_count DESC, name ASC LIMIT $2 OFFSET $3`,
			category, limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("listing skill catalog entries: %w", err)
		}
		defer rows.Close()
		entries, err = scanCatalogRows(rows)
		if err != nil {
			return nil, 0, err
		}
	} else {
		err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM skill_catalog`)
		if err != nil {
			return nil, 0, fmt.Errorf("counting skill catalog entries: %w", err)
		}
		rows, err := r.db.QueryxContext(ctx, `
			SELECT id, name, description, instructions, category, tags, author_name, icon, featured, add_count, created_at, updated_at
			FROM skill_catalog ORDER BY featured DESC, add_count DESC, name ASC LIMIT $1 OFFSET $2`,
			limit, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("listing skill catalog entries: %w", err)
		}
		defer rows.Close()
		entries, err = scanCatalogRows(rows)
		if err != nil {
			return nil, 0, err
		}
	}

	return entries, total, nil
}

func (r *postgresSkillCatalogRepository) Search(ctx context.Context, query string, limit, offset int) ([]SkillCatalogEntry, int, error) {
	var total int
	pattern := "%" + query + "%"

	err := r.db.GetContext(ctx, &total, `
		SELECT COUNT(*) FROM skill_catalog WHERE name ILIKE $1 OR description ILIKE $1 OR category ILIKE $1`, pattern)
	if err != nil {
		return nil, 0, fmt.Errorf("counting search results: %w", err)
	}

	rows, err := r.db.QueryxContext(ctx, `
		SELECT id, name, description, instructions, category, tags, author_name, icon, featured, add_count, created_at, updated_at
		FROM skill_catalog WHERE name ILIKE $1 OR description ILIKE $1 OR category ILIKE $1
		ORDER BY featured DESC, add_count DESC, name ASC LIMIT $2 OFFSET $3`,
		pattern, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("searching skill catalog: %w", err)
	}
	defer rows.Close()

	entries, err := scanCatalogRows(rows)
	if err != nil {
		return nil, 0, err
	}

	return entries, total, nil
}

func (r *postgresSkillCatalogRepository) GetByID(ctx context.Context, id string) (*SkillCatalogEntry, error) {
	var entry SkillCatalogEntry
	var tags pq.StringArray
	err := r.db.QueryRowxContext(ctx, `
		SELECT id, name, description, instructions, category, tags, author_name, icon, featured, add_count, created_at, updated_at
		FROM skill_catalog WHERE id = $1`, id).Scan(
		&entry.ID, &entry.Name, &entry.Description, &entry.Instructions,
		&entry.Category, &tags, &entry.AuthorName, &entry.Icon,
		&entry.Featured, &entry.AddCount, &entry.CreatedAt, &entry.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("skill catalog entry not found: %s", id)
		}
		return nil, fmt.Errorf("getting skill catalog entry: %w", err)
	}
	entry.Tags = []string(tags)
	return &entry, nil
}

func (r *postgresSkillCatalogRepository) Categories(ctx context.Context) ([]string, error) {
	var categories []string
	err := r.db.SelectContext(ctx, &categories, `SELECT DISTINCT category FROM skill_catalog ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("listing categories: %w", err)
	}
	return categories, nil
}

func (r *postgresSkillCatalogRepository) IncrementAddCount(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE skill_catalog SET add_count = add_count + 1 WHERE id = $1`, id)
	return err
}

func (r *postgresSkillCatalogRepository) Seed(ctx context.Context, entries []SkillCatalogEntry) error {
	// Only seed if table is empty
	var count int
	if err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM skill_catalog`); err != nil {
		return fmt.Errorf("checking skill catalog count: %w", err)
	}
	if count > 0 {
		return nil
	}

	for _, entry := range entries {
		if entry.ID == "" {
			entry.ID = uuid.New().String()
		}
		now := time.Now()
		_, err := r.db.ExecContext(ctx, `
			INSERT INTO skill_catalog (id, name, description, instructions, category, tags, author_name, icon, featured, add_count, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
			entry.ID, entry.Name, entry.Description, entry.Instructions,
			entry.Category, pq.Array(entry.Tags), entry.AuthorName, entry.Icon,
			entry.Featured, entry.AddCount, now, now)
		if err != nil {
			return fmt.Errorf("seeding skill catalog entry %q: %w", entry.Name, err)
		}
	}

	return nil
}

func scanCatalogRows(rows *sqlx.Rows) ([]SkillCatalogEntry, error) {
	var entries []SkillCatalogEntry
	for rows.Next() {
		var entry SkillCatalogEntry
		var tags pq.StringArray
		if err := rows.Scan(
			&entry.ID, &entry.Name, &entry.Description, &entry.Instructions,
			&entry.Category, &tags, &entry.AuthorName, &entry.Icon,
			&entry.Featured, &entry.AddCount, &entry.CreatedAt, &entry.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning skill catalog row: %w", err)
		}
		entry.Tags = []string(tags)
		entries = append(entries, entry)
	}
	return entries, nil
}
