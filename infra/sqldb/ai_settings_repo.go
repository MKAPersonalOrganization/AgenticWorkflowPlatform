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

// PostgresAISettingsRepository implements domain.AISettingsRepository.
type PostgresAISettingsRepository struct {
	db *sqlx.DB
}

// NewPostgresAISettingsRepository creates a new repository.
func NewPostgresAISettingsRepository(db *sqlx.DB) *PostgresAISettingsRepository {
	return &PostgresAISettingsRepository{db: db}
}

// GetByUserID returns all AI settings for a user.
func (r *PostgresAISettingsRepository) GetByUserID(ctx context.Context, userID string) ([]domain.AISettings, error) {
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
func (r *PostgresAISettingsRepository) GetDefaultForUser(ctx context.Context, userID string) (*domain.AISettings, error) {
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
func (r *PostgresAISettingsRepository) GetByID(ctx context.Context, id string) (*domain.AISettings, error) {
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
func (r *PostgresAISettingsRepository) Upsert(ctx context.Context, settings *domain.AISettings) error {
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
func (r *PostgresAISettingsRepository) Delete(ctx context.Context, id string, userID string) error {
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
func (r *PostgresAISettingsRepository) SetDefault(ctx context.Context, id string, userID string) error {
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
