package aisettings

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"school-with-ai/domain"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ═══════════════════════════════════════════════════════════════════════════════
// AI Settings Repository
// ═══════════════════════════════════════════════════════════════════════════════

// postgresAISettingsRepository implements domain.AISettingsRepository.
type postgresAISettingsRepository struct {
	db *sqlx.DB
}

// newPostgresAISettingsRepository creates a new repository.
func newPostgresAISettingsRepository(db *sqlx.DB) *postgresAISettingsRepository {
	return &postgresAISettingsRepository{db: db}
}

// GetByUserID returns all AI settings for a user.
func (r *postgresAISettingsRepository) GetByUserID(ctx context.Context, userID string) ([]domain.AISettings, error) {
	query := `
SELECT id, user_id, provider, api_key, is_default, created_at, updated_at
FROM user_ai_settings
WHERE user_id = $1
ORDER BY is_default DESC, created_at ASC
`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying ai settings: %w", err)
	}
	defer rows.Close()

	var results []domain.AISettings
	for rows.Next() {
		var s domain.AISettings
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.Provider, &s.APIKey,
			&s.IsDefault, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning ai settings row: %w", err)
		}
		results = append(results, s)
	}

	return results, rows.Err()
}

// GetDefaultForUser returns the user's default AI settings.
func (r *postgresAISettingsRepository) GetDefaultForUser(ctx context.Context, userID string) (*domain.AISettings, error) {
	query := `
SELECT id, user_id, provider, api_key, is_default, created_at, updated_at
FROM user_ai_settings
WHERE user_id = $1 AND is_default = true
LIMIT 1
`
	var s domain.AISettings
	err := r.db.QueryRowContext(ctx, query, userID).Scan(
		&s.ID, &s.UserID, &s.Provider, &s.APIKey,
		&s.IsDefault, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying default ai settings: %w", err)
	}

	return &s, nil
}

// GetByID returns a specific AI settings entry.
func (r *postgresAISettingsRepository) GetByID(ctx context.Context, id string) (*domain.AISettings, error) {
	query := `
SELECT id, user_id, provider, api_key, is_default, created_at, updated_at
FROM user_ai_settings
WHERE id = $1
`
	var s domain.AISettings
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&s.ID, &s.UserID, &s.Provider, &s.APIKey,
		&s.IsDefault, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying ai settings by id: %w", err)
	}

	return &s, nil
}

// Upsert creates or updates an AI settings entry.
func (r *postgresAISettingsRepository) Upsert(ctx context.Context, settings *domain.AISettings) error {
	now := time.Now()
	if settings.ID == "" {
		settings.ID = uuid.New().String()
	}
	if settings.CreatedAt.IsZero() {
		settings.CreatedAt = now
	}
	settings.UpdatedAt = now

	query := `
INSERT INTO user_ai_settings (id, user_id, provider, api_key, is_default, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (user_id, provider) DO UPDATE SET
api_key = EXCLUDED.api_key,
is_default = EXCLUDED.is_default,
updated_at = EXCLUDED.updated_at
`

	_, err := r.db.ExecContext(ctx, query,
		settings.ID, settings.UserID, settings.Provider, settings.APIKey,
		settings.IsDefault,
		settings.CreatedAt, settings.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upserting ai settings: %w", err)
	}

	return nil
}

// Delete removes an AI settings entry.
func (r *postgresAISettingsRepository) Delete(ctx context.Context, id string, userID string) error {
	query := `DELETE FROM user_ai_settings WHERE id = $1 AND user_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("deleting ai settings: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("ai settings not found")
	}
	return nil
}

// SetDefault marks one entry as default and unsets all others for the user.
func (r *postgresAISettingsRepository) SetDefault(ctx context.Context, id string, userID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Unset all defaults for user
	if _, err := tx.ExecContext(ctx, `UPDATE user_ai_settings SET is_default = false, updated_at = $1 WHERE user_id = $2`, time.Now(), userID); err != nil {
		return fmt.Errorf("unsetting defaults: %w", err)
	}

	// Set the chosen one as default
	result, err := tx.ExecContext(ctx, `UPDATE user_ai_settings SET is_default = true, updated_at = $1 WHERE id = $2 AND user_id = $3`, time.Now(), id, userID)
	if err != nil {
		return fmt.Errorf("setting default: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("ai settings not found")
	}

	return tx.Commit()
}

// ═══════════════════════════════════════════════════════════════════════════════
// AI Capability Config Repository
// ═══════════════════════════════════════════════════════════════════════════════

// postgresAICapabilityConfigRepository implements domain.AICapabilityConfigRepository.
type postgresAICapabilityConfigRepository struct {
	db *sqlx.DB
}

// newPostgresAICapabilityConfigRepository creates a new repository.
func newPostgresAICapabilityConfigRepository(db *sqlx.DB) *postgresAICapabilityConfigRepository {
	return &postgresAICapabilityConfigRepository{db: db}
}

// GetAllByUserID returns all capability configs for a user.
func (r *postgresAICapabilityConfigRepository) GetAllByUserID(ctx context.Context, userID string) ([]domain.AICapabilityConfig, error) {
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
func (r *postgresAICapabilityConfigRepository) GetByCapability(ctx context.Context, userID string, capability domain.AICapability) (*domain.AICapabilityConfig, error) {
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
func (r *postgresAICapabilityConfigRepository) Upsert(ctx context.Context, config *domain.AICapabilityConfig) error {
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
func (r *postgresAICapabilityConfigRepository) Delete(ctx context.Context, userID string, capability domain.AICapability) error {
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
