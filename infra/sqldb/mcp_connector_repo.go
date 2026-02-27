package sqldb

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

// PostgresMCPConnectorRepository implements domain.MCPConnectorRepository.
type PostgresMCPConnectorRepository struct {
	db *sqlx.DB
}

// NewPostgresMCPConnectorRepository creates a new repository.
func NewPostgresMCPConnectorRepository(db *sqlx.DB) *PostgresMCPConnectorRepository {
	return &PostgresMCPConnectorRepository{db: db}
}

// Create inserts a new MCP connector.
func (r *PostgresMCPConnectorRepository) Create(ctx context.Context, c *domain.MCPConnector) error {
	now := time.Now()
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now

	if c.Status == "" {
		c.Status = domain.ConnectorStatusCreated
	}

	query := `
		INSERT INTO mcp_connectors (
			id, user_id, title, server_url, status, error_msg,
			disconnect_reason, disconnect_permanent,
			oauth_client_id, oauth_client_secret, oauth_token_json, oauth_server_meta,
			oauth_state, oauth_code_verifier,
			server_name, server_version, capabilities,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8,
			$9, $10, $11, $12,
			$13, $14,
			$15, $16, $17,
			$18, $19
		)
	`
	_, err := r.db.ExecContext(ctx, query,
		c.ID, c.UserID, c.Title, c.ServerURL, c.Status, c.ErrorMsg,
		c.DisconnectReason, c.DisconnectPermanent,
		c.OAuthClientID, c.OAuthClientSecret, c.OAuthTokenJSON, c.OAuthServerMeta,
		c.OAuthState, c.OAuthCodeVerifier,
		c.ServerName, c.ServerVersion, c.Capabilities,
		c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting mcp connector: %w", err)
	}
	return nil
}

// GetByID returns a connector by its ID.
func (r *PostgresMCPConnectorRepository) GetByID(ctx context.Context, id string) (*domain.MCPConnector, error) {
	query := `
		SELECT id, user_id, title, server_url, status, error_msg,
		       disconnect_reason, disconnect_permanent,
		       oauth_client_id, oauth_client_secret, oauth_token_json, oauth_server_meta,
		       oauth_state, oauth_code_verifier,
		       server_name, server_version, capabilities,
		       created_at, updated_at
		FROM mcp_connectors
		WHERE id = $1
	`
	var c domain.MCPConnector
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.UserID, &c.Title, &c.ServerURL, &c.Status, &c.ErrorMsg,
		&c.DisconnectReason, &c.DisconnectPermanent,
		&c.OAuthClientID, &c.OAuthClientSecret, &c.OAuthTokenJSON, &c.OAuthServerMeta,
		&c.OAuthState, &c.OAuthCodeVerifier,
		&c.ServerName, &c.ServerVersion, &c.Capabilities,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying mcp connector by id: %w", err)
	}
	return &c, nil
}

// ListByUser returns all connectors for a given user.
func (r *PostgresMCPConnectorRepository) ListByUser(ctx context.Context, userID string) ([]domain.MCPConnector, error) {
	query := `
		SELECT id, user_id, title, server_url, status, error_msg,
		       disconnect_reason, disconnect_permanent,
		       oauth_client_id, oauth_client_secret, oauth_token_json, oauth_server_meta,
		       oauth_state, oauth_code_verifier,
		       server_name, server_version, capabilities,
		       created_at, updated_at
		FROM mcp_connectors
		WHERE user_id = $1
		ORDER BY created_at ASC
	`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("querying mcp connectors: %w", err)
	}
	defer rows.Close()

	var results []domain.MCPConnector
	for rows.Next() {
		var c domain.MCPConnector
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Title, &c.ServerURL, &c.Status, &c.ErrorMsg,
			&c.DisconnectReason, &c.DisconnectPermanent,
			&c.OAuthClientID, &c.OAuthClientSecret, &c.OAuthTokenJSON, &c.OAuthServerMeta,
			&c.OAuthState, &c.OAuthCodeVerifier,
			&c.ServerName, &c.ServerVersion, &c.Capabilities,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning mcp connector row: %w", err)
		}
		results = append(results, c)
	}
	return results, rows.Err()
}

// Update updates an existing connector.
func (r *PostgresMCPConnectorRepository) Update(ctx context.Context, c *domain.MCPConnector) error {
	c.UpdatedAt = time.Now()

	query := `
		UPDATE mcp_connectors SET
			title = $1, server_url = $2, status = $3, error_msg = $4,
			disconnect_reason = $5, disconnect_permanent = $6,
			oauth_client_id = $7, oauth_client_secret = $8, oauth_token_json = $9, oauth_server_meta = $10,
			oauth_state = $11, oauth_code_verifier = $12,
			server_name = $13, server_version = $14, capabilities = $15,
			updated_at = $16
		WHERE id = $17
	`
	result, err := r.db.ExecContext(ctx, query,
		c.Title, c.ServerURL, c.Status, c.ErrorMsg,
		c.DisconnectReason, c.DisconnectPermanent,
		c.OAuthClientID, c.OAuthClientSecret, c.OAuthTokenJSON, c.OAuthServerMeta,
		c.OAuthState, c.OAuthCodeVerifier,
		c.ServerName, c.ServerVersion, c.Capabilities,
		c.UpdatedAt, c.ID,
	)
	if err != nil {
		return fmt.Errorf("updating mcp connector: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking updated rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("mcp connector not found: %s", c.ID)
	}
	return nil
}

// CountByUser returns the number of connectors a user has registered.
func (r *PostgresMCPConnectorRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mcp_connectors WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting mcp connectors: %w", err)
	}
	return count, nil
}

// GetByOAuthState looks up a connector by its pending OAuth state parameter.
// Returns nil if no connector has this state (expired or invalid).
func (r *PostgresMCPConnectorRepository) GetByOAuthState(ctx context.Context, state string) (*domain.MCPConnector, error) {
	query := `
		SELECT id, user_id, title, server_url, status, error_msg,
		       disconnect_reason, disconnect_permanent,
		       oauth_client_id, oauth_client_secret, oauth_token_json, oauth_server_meta,
		       oauth_state, oauth_code_verifier,
		       server_name, server_version, capabilities,
		       created_at, updated_at
		FROM mcp_connectors
		WHERE oauth_state = $1 AND oauth_state != ''
	`
	var c domain.MCPConnector
	err := r.db.QueryRowContext(ctx, query, state).Scan(
		&c.ID, &c.UserID, &c.Title, &c.ServerURL, &c.Status, &c.ErrorMsg,
		&c.DisconnectReason, &c.DisconnectPermanent,
		&c.OAuthClientID, &c.OAuthClientSecret, &c.OAuthTokenJSON, &c.OAuthServerMeta,
		&c.OAuthState, &c.OAuthCodeVerifier,
		&c.ServerName, &c.ServerVersion, &c.Capabilities,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying mcp connector by oauth state: %w", err)
	}
	return &c, nil
}

// Delete removes a connector by ID.
func (r *PostgresMCPConnectorRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM mcp_connectors WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting mcp connector: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking deleted rows: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("mcp connector not found: %s", id)
	}
	return nil
}

// UpsertTools inserts or replaces the discovered tools for a connector.
// It deletes all existing tools for the connector and inserts the new set.
func (r *PostgresMCPConnectorRepository) UpsertTools(ctx context.Context, connectorID string, tools []domain.MCPDiscoveredTool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing tools
	if _, err := tx.ExecContext(ctx, `DELETE FROM mcp_discovered_tools WHERE connector_id = $1`, connectorID); err != nil {
		return fmt.Errorf("deleting existing tools: %w", err)
	}

	// Insert new tools
	now := time.Now()
	for i := range tools {
		if tools[i].ID == "" {
			tools[i].ID = uuid.New().String()
		}
		tools[i].ConnectorID = connectorID
		if tools[i].CreatedAt.IsZero() {
			tools[i].CreatedAt = now
		}
		tools[i].UpdatedAt = now

		_, err := tx.ExecContext(ctx, `
			INSERT INTO mcp_discovered_tools (id, connector_id, name, description, input_schema, annotations, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`, tools[i].ID, tools[i].ConnectorID, tools[i].Name, tools[i].Description,
			tools[i].InputSchema, tools[i].Annotations, tools[i].CreatedAt, tools[i].UpdatedAt)
		if err != nil {
			return fmt.Errorf("inserting discovered tool %s: %w", tools[i].Name, err)
		}
	}

	return tx.Commit()
}

// ListTools returns all discovered tools for a connector.
func (r *PostgresMCPConnectorRepository) ListTools(ctx context.Context, connectorID string) ([]domain.MCPDiscoveredTool, error) {
	query := `
		SELECT id, connector_id, name, description, input_schema, annotations, created_at, updated_at
		FROM mcp_discovered_tools
		WHERE connector_id = $1
		ORDER BY name ASC
	`
	rows, err := r.db.QueryContext(ctx, query, connectorID)
	if err != nil {
		return nil, fmt.Errorf("querying discovered tools: %w", err)
	}
	defer rows.Close()

	var results []domain.MCPDiscoveredTool
	for rows.Next() {
		var t domain.MCPDiscoveredTool
		if err := rows.Scan(&t.ID, &t.ConnectorID, &t.Name, &t.Description, &t.InputSchema, &t.Annotations, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning discovered tool row: %w", err)
		}
		results = append(results, t)
	}
	return results, rows.Err()
}

// DeleteToolsByConnector removes all discovered tools for a connector.
func (r *PostgresMCPConnectorRepository) DeleteToolsByConnector(ctx context.Context, connectorID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM mcp_discovered_tools WHERE connector_id = $1`, connectorID)
	if err != nil {
		return fmt.Errorf("deleting tools for connector %s: %w", connectorID, err)
	}
	return nil
}
