package preferences

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// postgresUserPreferenceRepository implements UserPreferenceRepository.
type postgresUserPreferenceRepository struct {
	db *sqlx.DB
}

// newPostgresUserPreferenceRepository creates a new repository.
func newPostgresUserPreferenceRepository(db *sqlx.DB) *postgresUserPreferenceRepository {
	return &postgresUserPreferenceRepository{db: db}
}

// Get retrieves a single preference by user ID and key.
func (r *postgresUserPreferenceRepository) Get(ctx context.Context, userID, key string) (*UserPreference, error) {
	query := `
		SELECT user_id, key, value, updated_at
		FROM user_preferences
		WHERE user_id = $1 AND key = $2
	`
	var pref UserPreference
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
func (r *postgresUserPreferenceRepository) GetAll(ctx context.Context, userID string) ([]UserPreference, error) {
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
	defer func() { _ = rows.Close() }()

	var prefs []UserPreference
	for rows.Next() {
		var p UserPreference
		if err := rows.Scan(&p.UserID, &p.Key, &p.Value, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan user preference: %w", err)
		}
		prefs = append(prefs, p)
	}
	return prefs, rows.Err()
}

// Set creates or updates a single preference.
func (r *postgresUserPreferenceRepository) Set(ctx context.Context, pref *UserPreference) error {
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
