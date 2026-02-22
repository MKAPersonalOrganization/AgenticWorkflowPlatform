package sqldb

import (
	"context"
	"database/sql"
	"fmt"

	"school-with-ai/services/resources"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// PostgresResourcesRepository implements the Resources Repository using PostgreSQL.
type PostgresResourcesRepository struct {
	db *sqlx.DB
}

// NewPostgresResourcesRepository creates a new PostgresResourcesRepository.
func NewPostgresResourcesRepository(db *sqlx.DB) *PostgresResourcesRepository {
	return &PostgresResourcesRepository{
		db: db,
	}
}

// Create inserts a new resource into the database.
func (r *PostgresResourcesRepository) Create(ctx context.Context, res resources.Resource) error {
	const q = `
		INSERT INTO resources (
			resource_id, user_id, title, file_type, file_path, source, created_at, updated_at
		) VALUES (
			:resource_id, :user_id, :title, :file_type, :file_path, :source, :created_at, :updated_at
		)`

	if _, err := r.db.NamedExecContext(ctx, q, res); err != nil {
		return fmt.Errorf("inserting resource: %w", err)
	}

	return nil
}

// List returns a list of resources based on user_id and source filter.
func (r *PostgresResourcesRepository) List(ctx context.Context, userID string, source resources.ResourceSource) ([]resources.Resource, error) {
	var res []resources.Resource
	var err error

	if source == resources.SourcePublic {
		const q = `SELECT * FROM resources WHERE source = 'public' ORDER BY created_at DESC LIMIT 500`
		err = r.db.SelectContext(ctx, &res, q)
	} else if userID != "" {
		const q = `SELECT * FROM resources WHERE user_id = $1 AND source = 'private' ORDER BY created_at DESC LIMIT 500`
		err = r.db.SelectContext(ctx, &res, q, userID)
	} else {
		return nil, fmt.Errorf("user_id required for private resources")
	}

	if err != nil {
		return nil, fmt.Errorf("selecting resources: %w", err)
	}

	return res, nil
}

// GetByID retrieves a resource by its ID.
func (r *PostgresResourcesRepository) GetByID(ctx context.Context, id uuid.UUID) (resources.Resource, error) {
	const q = `SELECT * FROM resources WHERE resource_id = $1`

	var res resources.Resource
	if err := r.db.GetContext(ctx, &res, q, id); err != nil {
		if err == sql.ErrNoRows {
			return resources.Resource{}, fmt.Errorf("resource not found")
		}
		return resources.Resource{}, fmt.Errorf("selecting resource by id: %w", err)
	}

	return res, nil
}

// Delete removes a resource from the database.
func (r *PostgresResourcesRepository) Delete(ctx context.Context, id uuid.UUID, userID string) error {
	const q = `DELETE FROM resources WHERE resource_id = $1 AND (user_id = $2 OR source = 'public')`

	res, err := r.db.ExecContext(ctx, q, id, userID)
	if err != nil {
		return fmt.Errorf("deleting resource: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("resource not found or unauthorized")
	}

	return nil
}
