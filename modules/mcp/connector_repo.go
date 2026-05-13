package mcp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// postgresMCPConnectorRepository implements MCPConnectorRepository.
type postgresMCPConnectorRepository struct {
	db *sqlx.DB
}

// newPostgresMCPConnectorRepository creates a new repository.
func newPostgresMCPConnectorRepository(db *sqlx.DB) *postgresMCPConnectorRepository {
	return &postgresMCPConnectorRepository{db: db}
}

// Create inserts a new MCP connector.
func (r *postgresMCPConnectorRepository) Create(ctx context.Context, c *MCPConnector) error {
	now := time.Now()
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = now
	}
	c.UpdatedAt = now

	if c.Status == "" {
		c.Status = ConnectorStatusCreated
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
func (r *postgresMCPConnectorRepository) GetByID(ctx context.Context, id string) (*MCPConnector, error) {
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
	var c MCPConnector
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

// GetByIDAndUser returns a connector only if it belongs to the given user.
// The ownership check is enforced at the SQL level — the DB never returns
// another user's data, preventing information leakage to the application layer.
// Returns nil, nil if not found (same semantics as GetByID).
func (r *postgresMCPConnectorRepository) GetByIDAndUser(ctx context.Context, id, userID string) (*MCPConnector, error) {
	query := `
		SELECT id, user_id, title, server_url, status, error_msg,
		       disconnect_reason, disconnect_permanent,
		       oauth_client_id, oauth_client_secret, oauth_token_json, oauth_server_meta,
		       oauth_state, oauth_code_verifier,
		       server_name, server_version, capabilities,
		       created_at, updated_at
		FROM mcp_connectors
		WHERE id = $1 AND user_id = $2
	`
	var c MCPConnector
	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(
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
		return nil, fmt.Errorf("querying mcp connector by id and user: %w", err)
	}
	return &c, nil
}

// ListByUser returns all connectors for a given user.
func (r *postgresMCPConnectorRepository) ListByUser(ctx context.Context, userID string) ([]MCPConnector, error) {
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
	defer func() { _ = rows.Close() }()

	var results []MCPConnector
	for rows.Next() {
		var c MCPConnector
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
// The WHERE clause includes user_id as defense-in-depth — even if a service
// bug passes the wrong connector, the DB won't modify another user's row.
func (r *postgresMCPConnectorRepository) Update(ctx context.Context, c *MCPConnector) error {
	c.UpdatedAt = time.Now()

	query := `
		UPDATE mcp_connectors SET
			title = $1, server_url = $2, status = $3, error_msg = $4,
			disconnect_reason = $5, disconnect_permanent = $6,
			oauth_client_id = $7, oauth_client_secret = $8, oauth_token_json = $9, oauth_server_meta = $10,
			oauth_state = $11, oauth_code_verifier = $12,
			server_name = $13, server_version = $14, capabilities = $15,
			updated_at = $16
		WHERE id = $17 AND user_id = $18
	`
	result, err := r.db.ExecContext(ctx, query,
		c.Title, c.ServerURL, c.Status, c.ErrorMsg,
		c.DisconnectReason, c.DisconnectPermanent,
		c.OAuthClientID, c.OAuthClientSecret, c.OAuthTokenJSON, c.OAuthServerMeta,
		c.OAuthState, c.OAuthCodeVerifier,
		c.ServerName, c.ServerVersion, c.Capabilities,
		c.UpdatedAt, c.ID, c.UserID,
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
func (r *postgresMCPConnectorRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM mcp_connectors WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting mcp connectors: %w", err)
	}
	return count, nil
}

// GetByOAuthState looks up a connector by its pending OAuth state parameter.
// Returns nil if no connector has this state (expired or invalid).
func (r *postgresMCPConnectorRepository) GetByOAuthState(ctx context.Context, state string) (*MCPConnector, error) {
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
	var c MCPConnector
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

// Delete removes a connector by ID, scoped to the owning user.
// The user_id filter is defense-in-depth — prevents accidental cross-user
// deletes even if a service bug passes the wrong connector.
func (r *postgresMCPConnectorRepository) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM mcp_connectors WHERE id = $1 AND user_id = $2`
	result, err := r.db.ExecContext(ctx, query, id, userID)
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
// It deletes all existing tools and batch-inserts the new set in a single
// multi-value INSERT — O(1) queries instead of O(N) per-tool writes.
func (r *postgresMCPConnectorRepository) UpsertTools(ctx context.Context, connectorID string, tools []MCPDiscoveredTool) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete existing tools
	if _, err := tx.ExecContext(ctx, `DELETE FROM mcp_discovered_tools WHERE connector_id = $1`, connectorID); err != nil {
		return fmt.Errorf("deleting existing tools: %w", err)
	}

	if len(tools) == 0 {
		return tx.Commit()
	}

	// Batch insert with multi-value INSERT — one query for all tools.
	// At 100K users × 10 connectors × avg 10 tools, this eliminates millions
	// of individual INSERT round-trips during peak connect/refresh cycles.
	now := time.Now()
	const cols = 8 // id, connector_id, name, description, input_schema, annotations, created_at, updated_at
	args := make([]any, 0, len(tools)*cols)
	valuesClauses := make([]string, 0, len(tools))

	for i := range tools {
		if tools[i].ID == "" {
			tools[i].ID = uuid.New().String()
		}
		tools[i].ConnectorID = connectorID
		if tools[i].CreatedAt.IsZero() {
			tools[i].CreatedAt = now
		}
		tools[i].UpdatedAt = now

		base := i * cols
		valuesClauses = append(valuesClauses, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8,
		))
		args = append(args,
			tools[i].ID, tools[i].ConnectorID, tools[i].Name, tools[i].Description,
			tools[i].InputSchema, tools[i].Annotations, tools[i].CreatedAt, tools[i].UpdatedAt,
		)
	}

	const insertPrefix = `INSERT INTO mcp_discovered_tools (id, connector_id, name, description, input_schema, annotations, created_at, updated_at) VALUES `
	var b strings.Builder
	b.Grow(len(insertPrefix) + len(valuesClauses)*32)
	b.WriteString(insertPrefix)
	for i, clause := range valuesClauses {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(clause)
	}
	query := b.String()

	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("batch inserting discovered tools: %w", err)
	}

	return tx.Commit()
}

// ListTools returns all discovered tools for a connector.
func (r *postgresMCPConnectorRepository) ListTools(ctx context.Context, connectorID string) ([]MCPDiscoveredTool, error) {
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
	defer func() { _ = rows.Close() }()

	var results []MCPDiscoveredTool
	for rows.Next() {
		var t MCPDiscoveredTool
		if err := rows.Scan(&t.ID, &t.ConnectorID, &t.Name, &t.Description, &t.InputSchema, &t.Annotations, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning discovered tool row: %w", err)
		}
		results = append(results, t)
	}
	return results, rows.Err()
}

// DeleteToolsByConnector removes all discovered tools for a connector.
func (r *postgresMCPConnectorRepository) DeleteToolsByConnector(ctx context.Context, connectorID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM mcp_discovered_tools WHERE connector_id = $1`, connectorID)
	if err != nil {
		return fmt.Errorf("deleting tools for connector %s: %w", connectorID, err)
	}
	return nil
}

// ListToolsByConnectors returns tools for multiple connectors in a single
// query, grouped by connector ID. This eliminates N+1 queries when resolving
// tools for all of a user's connected MCP servers.
func (r *postgresMCPConnectorRepository) ListToolsByConnectors(ctx context.Context, connectorIDs []string) (map[string][]MCPDiscoveredTool, error) {
	if len(connectorIDs) == 0 {
		return map[string][]MCPDiscoveredTool{}, nil
	}

	query := `
		SELECT id, connector_id, name, description, input_schema, annotations, created_at, updated_at
		FROM mcp_discovered_tools
		WHERE connector_id = ANY($1)
		ORDER BY connector_id, name ASC
	`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(connectorIDs))
	if err != nil {
		return nil, fmt.Errorf("querying tools by connectors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]MCPDiscoveredTool, len(connectorIDs))
	for rows.Next() {
		var t MCPDiscoveredTool
		if err := rows.Scan(&t.ID, &t.ConnectorID, &t.Name, &t.Description, &t.InputSchema, &t.Annotations, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning discovered tool row: %w", err)
		}
		result[t.ConnectorID] = append(result[t.ConnectorID], t)
	}
	return result, rows.Err()
}

// CountToolsByConnectors returns a map of connector ID → tool count.
// Uses a single GROUP BY query to avoid N+1 when listing connectors.
func (r *postgresMCPConnectorRepository) CountToolsByConnectors(ctx context.Context, connectorIDs []string) (map[string]int, error) {
	if len(connectorIDs) == 0 {
		return map[string]int{}, nil
	}

	query := `
		SELECT connector_id, COUNT(*) as cnt
		FROM mcp_discovered_tools
		WHERE connector_id = ANY($1)
		GROUP BY connector_id
	`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(connectorIDs))
	if err != nil {
		return nil, fmt.Errorf("counting tools by connectors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]int, len(connectorIDs))
	for rows.Next() {
		var id string
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, fmt.Errorf("scanning tool count row: %w", err)
		}
		result[id] = count
	}
	return result, rows.Err()
}
