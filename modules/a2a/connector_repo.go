package a2a

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

// postgresA2AConnectorRepository implements A2AConnectorRepository.
type postgresA2AConnectorRepository struct {
	db *sqlx.DB
}

func newPostgresA2AConnectorRepository(db *sqlx.DB) *postgresA2AConnectorRepository {
	return &postgresA2AConnectorRepository{db: db}
}

// Create inserts a new A2A connector.
func (r *postgresA2AConnectorRepository) Create(ctx context.Context, c *A2AConnector) error {
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

	// agent_card_json is JSONB — empty string is invalid JSON, default to "{}".
	if c.AgentCardJSON == "" {
		c.AgentCardJSON = "{}"
	}

	query := `
		INSERT INTO a2a_connectors (
			id, user_id, title, agent_url, status, error_msg,
			disconnect_reason, disconnect_permanent,
			agent_name, agent_description, agent_version, agent_provider,
			agent_card_json, capabilities, icon_url,
			auth_type, auth_token, auth_header,
			created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8,
			$9, $10, $11, $12,
			$13, $14, $15,
			$16, $17, $18,
			$19, $20
		)
	`
	_, err := r.db.ExecContext(ctx, query,
		c.ID, c.UserID, c.Title, c.AgentURL, c.Status, c.ErrorMsg,
		c.DisconnectReason, c.DisconnectPermanent,
		c.AgentName, c.AgentDescription, c.AgentVersion, c.AgentProvider,
		c.AgentCardJSON, c.Capabilities, c.IconURL,
		c.AuthType, c.AuthToken, c.AuthHeader,
		c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting a2a connector: %w", err)
	}
	return nil
}

// GetByID returns a connector by its ID.
func (r *postgresA2AConnectorRepository) GetByID(ctx context.Context, id string) (*A2AConnector, error) {
	query := `
		SELECT id, user_id, title, agent_url, status, error_msg,
		       disconnect_reason, disconnect_permanent,
		       agent_name, agent_description, agent_version, agent_provider,
		       agent_card_json, capabilities, icon_url,
		       auth_type, auth_token, auth_header,
		       created_at, updated_at
		FROM a2a_connectors
		WHERE id = $1
	`
	var c A2AConnector
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.UserID, &c.Title, &c.AgentURL, &c.Status, &c.ErrorMsg,
		&c.DisconnectReason, &c.DisconnectPermanent,
		&c.AgentName, &c.AgentDescription, &c.AgentVersion, &c.AgentProvider,
		&c.AgentCardJSON, &c.Capabilities, &c.IconURL,
		&c.AuthType, &c.AuthToken, &c.AuthHeader,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying a2a connector by id: %w", err)
	}
	return &c, nil
}

// GetByIDAndUser returns a connector only if it belongs to the given user.
func (r *postgresA2AConnectorRepository) GetByIDAndUser(ctx context.Context, id, userID string) (*A2AConnector, error) {
	query := `
		SELECT id, user_id, title, agent_url, status, error_msg,
		       disconnect_reason, disconnect_permanent,
		       agent_name, agent_description, agent_version, agent_provider,
		       agent_card_json, capabilities, icon_url,
		       auth_type, auth_token, auth_header,
		       created_at, updated_at
		FROM a2a_connectors
		WHERE id = $1 AND user_id = $2
	`
	var c A2AConnector
	err := r.db.QueryRowContext(ctx, query, id, userID).Scan(
		&c.ID, &c.UserID, &c.Title, &c.AgentURL, &c.Status, &c.ErrorMsg,
		&c.DisconnectReason, &c.DisconnectPermanent,
		&c.AgentName, &c.AgentDescription, &c.AgentVersion, &c.AgentProvider,
		&c.AgentCardJSON, &c.Capabilities, &c.IconURL,
		&c.AuthType, &c.AuthToken, &c.AuthHeader,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("querying a2a connector by id and user: %w", err)
	}
	return &c, nil
}

// ListByUser returns all connectors for a user.
func (r *postgresA2AConnectorRepository) ListByUser(ctx context.Context, userID string) ([]A2AConnector, error) {
	query := `
		SELECT id, user_id, title, agent_url, status, error_msg,
		       disconnect_reason, disconnect_permanent,
		       agent_name, agent_description, agent_version, agent_provider,
		       agent_card_json, capabilities, icon_url,
		       auth_type, auth_token, auth_header,
		       created_at, updated_at
		FROM a2a_connectors
		WHERE user_id = $1
		ORDER BY created_at DESC
	`
	var connectors []A2AConnector
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("listing a2a connectors: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var c A2AConnector
		if err := rows.Scan(
			&c.ID, &c.UserID, &c.Title, &c.AgentURL, &c.Status, &c.ErrorMsg,
			&c.DisconnectReason, &c.DisconnectPermanent,
			&c.AgentName, &c.AgentDescription, &c.AgentVersion, &c.AgentProvider,
			&c.AgentCardJSON, &c.Capabilities, &c.IconURL,
			&c.AuthType, &c.AuthToken, &c.AuthHeader,
			&c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning a2a connector: %w", err)
		}
		connectors = append(connectors, c)
	}
	return connectors, rows.Err()
}

// CountByUser returns the number of connectors for a user.
func (r *postgresA2AConnectorRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM a2a_connectors WHERE user_id = $1`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting a2a connectors: %w", err)
	}
	return count, nil
}

// Update persists all mutable fields of a connector.
func (r *postgresA2AConnectorRepository) Update(ctx context.Context, c *A2AConnector) error {
	c.UpdatedAt = time.Now()

	query := `
		UPDATE a2a_connectors SET
			title = $1, agent_url = $2, status = $3, error_msg = $4,
			disconnect_reason = $5, disconnect_permanent = $6,
			agent_name = $7, agent_description = $8, agent_version = $9, agent_provider = $10,
			agent_card_json = $11, capabilities = $12, icon_url = $13,
			auth_type = $14, auth_token = $15, auth_header = $16,
			updated_at = $17
		WHERE id = $18 AND user_id = $19
	`
	res, err := r.db.ExecContext(ctx, query,
		c.Title, c.AgentURL, c.Status, c.ErrorMsg,
		c.DisconnectReason, c.DisconnectPermanent,
		c.AgentName, c.AgentDescription, c.AgentVersion, c.AgentProvider,
		c.AgentCardJSON, c.Capabilities, c.IconURL,
		c.AuthType, c.AuthToken, c.AuthHeader,
		c.UpdatedAt, c.ID, c.UserID,
	)
	if err != nil {
		return fmt.Errorf("updating a2a connector: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrConnectorNotFound
	}
	return nil
}

// Delete removes a connector, scoped to the owning user.
func (r *postgresA2AConnectorRepository) Delete(ctx context.Context, id, userID string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM a2a_connectors WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("deleting a2a connector: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return ErrConnectorNotFound
	}
	return nil
}

// UpsertSkills replaces all skills for a connector in a single transaction.
func (r *postgresA2AConnectorRepository) UpsertSkills(ctx context.Context, connectorID string, skills []A2ADiscoveredSkill) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning skills upsert transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Delete existing skills
	if _, err := tx.ExecContext(ctx, `DELETE FROM a2a_discovered_skills WHERE connector_id = $1`, connectorID); err != nil {
		return fmt.Errorf("deleting existing skills: %w", err)
	}

	if len(skills) == 0 {
		return tx.Commit()
	}

	// Batch insert
	now := time.Now()
	valueStrings := make([]string, 0, len(skills))
	valueArgs := make([]interface{}, 0, len(skills)*10)

	for i, s := range skills {
		if s.ID == "" {
			s.ID = uuid.New().String()
		}
		base := i * 10
		valueStrings = append(valueStrings, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10,
		))
		valueArgs = append(valueArgs,
			s.ID, connectorID, s.SkillID, s.Name, s.Description,
			s.Tags, s.Examples, s.InputModes, s.OutputModes, now,
		)
	}

	const insertPrefix = `INSERT INTO a2a_discovered_skills (id, connector_id, skill_id, name, description, tags, examples, input_modes, output_modes, created_at) VALUES `
	var b strings.Builder
	b.Grow(len(insertPrefix) + len(valueStrings)*40)
	b.WriteString(insertPrefix)
	for i, clause := range valueStrings {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(clause)
	}
	query := b.String()

	if _, err := tx.ExecContext(ctx, query, valueArgs...); err != nil {
		return fmt.Errorf("inserting skills: %w", err)
	}

	return tx.Commit()
}

// ListSkills returns all discovered skills for a connector.
func (r *postgresA2AConnectorRepository) ListSkills(ctx context.Context, connectorID string) ([]A2ADiscoveredSkill, error) {
	query := `
		SELECT id, connector_id, skill_id, name, description, tags, examples, input_modes, output_modes, created_at, updated_at
		FROM a2a_discovered_skills
		WHERE connector_id = $1
		ORDER BY name
	`
	var skills []A2ADiscoveredSkill
	rows, err := r.db.QueryContext(ctx, query, connectorID)
	if err != nil {
		return nil, fmt.Errorf("listing a2a skills: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var s A2ADiscoveredSkill
		if err := rows.Scan(&s.ID, &s.ConnectorID, &s.SkillID, &s.Name, &s.Description, &s.Tags, &s.Examples, &s.InputModes, &s.OutputModes, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning a2a skill: %w", err)
		}
		skills = append(skills, s)
	}
	return skills, rows.Err()
}

// ListSkillsByConnectors returns skills for multiple connectors in a single query.
func (r *postgresA2AConnectorRepository) ListSkillsByConnectors(ctx context.Context, connectorIDs []string) (map[string][]A2ADiscoveredSkill, error) {
	if len(connectorIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT id, connector_id, skill_id, name, description, tags, examples, input_modes, output_modes, created_at, updated_at
		FROM a2a_discovered_skills
		WHERE connector_id = ANY($1)
		ORDER BY name
	`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(connectorIDs))
	if err != nil {
		return nil, fmt.Errorf("batch listing a2a skills: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string][]A2ADiscoveredSkill)
	for rows.Next() {
		var s A2ADiscoveredSkill
		if err := rows.Scan(&s.ID, &s.ConnectorID, &s.SkillID, &s.Name, &s.Description, &s.Tags, &s.Examples, &s.InputModes, &s.OutputModes, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning a2a skill: %w", err)
		}
		result[s.ConnectorID] = append(result[s.ConnectorID], s)
	}
	return result, rows.Err()
}

// DeleteSkillsByConnector removes all skills for a connector.
func (r *postgresA2AConnectorRepository) DeleteSkillsByConnector(ctx context.Context, connectorID string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM a2a_discovered_skills WHERE connector_id = $1`, connectorID)
	if err != nil {
		return fmt.Errorf("deleting a2a skills: %w", err)
	}
	return nil
}

// CountSkillsByConnectors returns a map of connector ID → skill count.
func (r *postgresA2AConnectorRepository) CountSkillsByConnectors(ctx context.Context, connectorIDs []string) (map[string]int, error) {
	if len(connectorIDs) == 0 {
		return nil, nil
	}

	query := `
		SELECT connector_id, COUNT(*) as skill_count
		FROM a2a_discovered_skills
		WHERE connector_id = ANY($1)
		GROUP BY connector_id
	`
	rows, err := r.db.QueryContext(ctx, query, pq.Array(connectorIDs))
	if err != nil {
		return nil, fmt.Errorf("counting a2a skills: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make(map[string]int)
	for rows.Next() {
		var connectorID string
		var count int
		if err := rows.Scan(&connectorID, &count); err != nil {
			return nil, fmt.Errorf("scanning skill count: %w", err)
		}
		result[connectorID] = count
	}
	return result, rows.Err()
}
