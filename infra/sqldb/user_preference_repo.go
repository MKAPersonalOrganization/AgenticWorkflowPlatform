package sqldb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"school-with-ai/domain"
	"time"

	"github.com/jmoiron/sqlx"
)

// PostgresUserPreferenceRepository implements domain.UserPreferenceRepository.
type PostgresUserPreferenceRepository struct {
	db *sqlx.DB
}

// NewPostgresUserPreferenceRepository creates a new repository.
func NewPostgresUserPreferenceRepository(db *sqlx.DB) *PostgresUserPreferenceRepository {
	return &PostgresUserPreferenceRepository{db: db}
}

// Get retrieves a single preference by user ID and key.
func (r *PostgresUserPreferenceRepository) Get(ctx context.Context, userID, key string) (*domain.UserPreference, error) {
	query := `
		SELECT user_id, key, value, updated_at
		FROM user_preferences
		WHERE user_id = $1 AND key = $2
	`
	var pref domain.UserPreference
	err := r.db.QueryRowContext(ctx, query, userID, key).Scan(
		&pref.UserID,
		&pref.Key,
		&pref.Value,
		&pref.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user preference: %w", err)
	}
	return &pref, nil
}

// GetAll retrieves all preferences for a user.
func (r *PostgresUserPreferenceRepository) GetAll(ctx context.Context, userID string) ([]domain.UserPreference, error) {
	query := `
		SELECT user_id, key, value, updated_at
		FROM user_preferences
		WHERE user_id = $1
		ORDER BY key
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user preferences: %w", err)
	}
	defer rows.Close()

	var prefs []domain.UserPreference
	for rows.Next() {
		var p domain.UserPreference
		if err := rows.Scan(&p.UserID, &p.Key, &p.Value, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user preference: %w", err)
		}
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}

// Set creates or updates a single preference.
func (r *PostgresUserPreferenceRepository) Set(ctx context.Context, pref *domain.UserPreference) error {
	query := `
		INSERT INTO user_preferences (user_id, key, value, updated_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, key) DO UPDATE SET
			value = EXCLUDED.value,
			updated_at = EXCLUDED.updated_at
	`
	pref.UpdatedAt = time.Now()

	_, err := r.db.ExecContext(ctx, query,
		pref.UserID,
		pref.Key,
		pref.Value,
		pref.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to upsert user preference: %w", err)
	}
	return nil
}
