package githubauth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

// Connection represents a stored GitHub OAuth connection for a user.
type Connection struct {
	UserID         string    `json:"user_id" db:"user_id"`
	AccessToken    string    `json:"-" db:"access_token"`
	GitHubUsername string    `json:"github_username" db:"github_username"`
	GitHubAvatar   string    `json:"github_avatar" db:"github_avatar"`
	ConnectedAt    time.Time `json:"connected_at" db:"connected_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Store persists GitHub OAuth connections in PostgreSQL.
type Store struct {
	db *sqlx.DB
}

// NewStore creates a new GitHub connection store.
func NewStore(db *sqlx.DB) *Store {
	return &Store{db: db}
}

// Upsert creates or updates a GitHub connection for the given user.
func (s *Store) Upsert(ctx context.Context, conn Connection) error {
	const q = `
		INSERT INTO github_connections (user_id, access_token, github_username, github_avatar, connected_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (user_id) DO UPDATE SET
			access_token    = EXCLUDED.access_token,
			github_username = EXCLUDED.github_username,
			github_avatar   = EXCLUDED.github_avatar,
			updated_at      = EXCLUDED.updated_at`

	_, err := s.db.ExecContext(ctx, q,
		conn.UserID,
		conn.AccessToken,
		conn.GitHubUsername,
		conn.GitHubAvatar,
		conn.ConnectedAt,
		conn.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("githubauth: upsert connection: %w", err)
	}
	return nil
}

// Get retrieves the GitHub connection for the given user.
// Returns nil, nil when no connection exists.
func (s *Store) Get(ctx context.Context, userID string) (*Connection, error) {
	var conn Connection
	const q = `SELECT * FROM github_connections WHERE user_id = $1`
	err := s.db.GetContext(ctx, &conn, q, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("githubauth: get connection: %w", err)
	}
	return &conn, nil
}

// GetToken retrieves just the access token for the given user.
// Returns empty string when no connection exists.
func (s *Store) GetToken(ctx context.Context, userID string) (string, error) {
	var token string
	const q = `SELECT access_token FROM github_connections WHERE user_id = $1`
	err := s.db.GetContext(ctx, &token, q, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("githubauth: get token: %w", err)
	}
	return token, nil
}

// Delete removes the GitHub connection for the given user.
func (s *Store) Delete(ctx context.Context, userID string) error {
	const q = `DELETE FROM github_connections WHERE user_id = $1`
	_, err := s.db.ExecContext(ctx, q, userID)
	if err != nil {
		return fmt.Errorf("githubauth: delete connection: %w", err)
	}
	return nil
}
