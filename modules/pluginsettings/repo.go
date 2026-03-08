package pluginsettings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Repository defines persistence for user plugin settings.
type Repository interface {
	// GetByUserPlugin returns the settings for a specific user + plugin pair.
	GetByUserPlugin(ctx context.Context, userID, pluginID string) (*UserPluginSettings, error)

	// GetAllByUser returns all plugin settings for a user.
	GetAllByUser(ctx context.Context, userID string) ([]UserPluginSettings, error)

	// Upsert creates or updates the settings for a user + plugin pair.
	Upsert(ctx context.Context, s *UserPluginSettings) error

	// Delete removes the settings for a user + plugin pair.
	Delete(ctx context.Context, userID, pluginID string) error
}

// ─── PostgreSQL implementation ──────────────────────────────────────────────

type postgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository creates a new PostgreSQL-backed repository.
func NewPostgresRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

func (r *postgresRepository) GetByUserPlugin(ctx context.Context, userID, pluginID string) (*UserPluginSettings, error) {
	const q = `
SELECT user_id, plugin_id, settings, created_at, updated_at
FROM user_plugin_settings
WHERE user_id = $1 AND plugin_id = $2`

	var (
		s       UserPluginSettings
		rawJSON []byte
	)
	err := r.db.QueryRowContext(ctx, q, userID, pluginID).Scan(
		&s.UserID, &s.PluginID, &rawJSON, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying user_plugin_settings: %w", err)
	}

	if err := json.Unmarshal(rawJSON, &s.Settings); err != nil {
		return nil, fmt.Errorf("unmarshalling settings JSON: %w", err)
	}
	return &s, nil
}

func (r *postgresRepository) GetAllByUser(ctx context.Context, userID string) ([]UserPluginSettings, error) {
	const q = `
SELECT user_id, plugin_id, settings, created_at, updated_at
FROM user_plugin_settings
WHERE user_id = $1
ORDER BY plugin_id ASC`

	rows, err := r.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("querying user_plugin_settings: %w", err)
	}
	defer rows.Close()

	var results []UserPluginSettings
	for rows.Next() {
		var (
			s       UserPluginSettings
			rawJSON []byte
		)
		if err := rows.Scan(&s.UserID, &s.PluginID, &rawJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning user_plugin_settings row: %w", err)
		}
		if err := json.Unmarshal(rawJSON, &s.Settings); err != nil {
			return nil, fmt.Errorf("unmarshalling settings JSON: %w", err)
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

func (r *postgresRepository) Upsert(ctx context.Context, s *UserPluginSettings) error {
	now := time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	s.UpdatedAt = now

	rawJSON, err := json.Marshal(s.Settings)
	if err != nil {
		return fmt.Errorf("marshalling settings JSON: %w", err)
	}

	const q = `
INSERT INTO user_plugin_settings (user_id, plugin_id, settings, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id, plugin_id) DO UPDATE SET
  settings   = EXCLUDED.settings,
  updated_at = EXCLUDED.updated_at`

	_, err = r.db.ExecContext(ctx, q,
		s.UserID, s.PluginID, rawJSON, s.CreatedAt, s.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("upserting user_plugin_settings: %w", err)
	}
	return nil
}

func (r *postgresRepository) Delete(ctx context.Context, userID, pluginID string) error {
	const q = `DELETE FROM user_plugin_settings WHERE user_id = $1 AND plugin_id = $2`
	result, err := r.db.ExecContext(ctx, q, userID, pluginID)
	if err != nil {
		return fmt.Errorf("deleting user_plugin_settings: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("plugin settings not found for plugin %q", pluginID)
	}
	return nil
}
