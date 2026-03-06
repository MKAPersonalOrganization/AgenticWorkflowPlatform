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
)

// PostgresAICapabilityConfigRepository implements domain.AICapabilityConfigRepository.
type PostgresAICapabilityConfigRepository struct {
	db *sqlx.DB
}

// NewPostgresAICapabilityConfigRepository creates a new repository.
func NewPostgresAICapabilityConfigRepository(db *sqlx.DB) *PostgresAICapabilityConfigRepository {
	return &PostgresAICapabilityConfigRepository{db: db}
}

// GetAllByUserID returns all capability configs for a user.
func (r *PostgresAICapabilityConfigRepository) GetAllByUserID(ctx context.Context, userID string) ([]domain.AICapabilityConfig, error) {
	query := `
		SELECT id, user_id, capability, provider, model, config, created_at, updated_at
		FROM user_ai_capability_config
		WHERE user_id = $1
		ORDER BY capability ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying ai capability configs: %w", err)
	}
	defer rows.Close()

	var results []domain.AICapabilityConfig
	for rows.Next() {
		var c domain.AICapabilityConfig
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Capability, &c.Provider, &c.Model, &c.Config, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning ai capability config row: %w", err)
		}
		results = append(results, c)
	}

	return results, rows.Err()
}

// GetByCapability returns a specific capability config.
func (r *PostgresAICapabilityConfigRepository) GetByCapability(ctx context.Context, userID string, capability domain.AICapability) (*domain.AICapabilityConfig, error) {
	query := `
		SELECT id, user_id, capability, provider, model, config, created_at, updated_at
		FROM user_ai_capability_config
		WHERE user_id = $1 AND capability = $2
	`
	var c domain.AICapabilityConfig
	err := r.db.QueryRowContext(ctx, query, userID, string(capability)).Scan(
		&c.ID, &c.UserID, &c.Capability, &c.Provider, &c.Model, &c.Config, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying ai capability config: %w", err)
	}

	return &c, nil
}

// Upsert creates or updates a capability config.
func (r *PostgresAICapabilityConfigRepository) Upsert(ctx context.Context, config *domain.AICapabilityConfig) error {
	now := time.Now()
	if config.ID == "" {
		config.ID = uuid.New().String()
	}
	if config.CreatedAt.IsZero() {
		config.CreatedAt = now
	}
	config.UpdatedAt = now

	query := `
		INSERT INTO user_ai_capability_config (id, user_id, capability, provider, model, config, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (user_id, capability) DO UPDATE SET
			provider   = EXCLUDED.provider,
			model      = EXCLUDED.model,
			config     = EXCLUDED.config,
			updated_at = EXCLUDED.updated_at
	`

	_, err := r.db.ExecContext(ctx, query,
		config.ID, config.UserID, string(config.Capability), string(config.Provider),
		config.Model, config.Config, config.CreatedAt, config.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upserting ai capability config: %w", err)
	}

	return nil
}

// Delete removes a capability config.
func (r *PostgresAICapabilityConfigRepository) Delete(ctx context.Context, userID string, capability domain.AICapability) error {
	query := `DELETE FROM user_ai_capability_config WHERE user_id = $1 AND capability = $2`
	result, err := r.db.ExecContext(ctx, query, userID, string(capability))
	if err != nil {
		return fmt.Errorf("deleting ai capability config: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("capability config not found")
	}
	return nil
}
