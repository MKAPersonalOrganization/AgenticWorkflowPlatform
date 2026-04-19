package toolreqs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// =============================================================================
// PostgresRepository — PostgreSQL implementation of [Repository]
// =============================================================================

// PostgresRepository implements [Repository] using PostgreSQL.
type PostgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository creates a new PostgresRepository.
func NewPostgresRepository(db *sqlx.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// =============================================================================
// Individual Tool Permission Methods
// =============================================================================

// GetPermission retrieves a specific agent-tool permission within a tool set.
func (r *PostgresRepository) GetPermission(ctx context.Context, userID, agentID, toolSetID, toolID string) (*Permission, error) {
	const q = `
		SELECT
			user_id, agent_id, tool_set_id, tool_id,
			COALESCE(config, '{}') as config, created_at, updated_at
		FROM agent_tool_permissions
		WHERE user_id = $1 AND agent_id = $2 AND tool_set_id = $3 AND tool_id = $4
	`

	var dest struct {
		UserID    string          `db:"user_id"`
		AgentID   string          `db:"agent_id"`
		ToolSetID string          `db:"tool_set_id"`
		ToolID    string          `db:"tool_id"`
		Config    json.RawMessage `db:"config"`
		CreatedAt sql.NullTime    `db:"created_at"`
		UpdatedAt sql.NullTime    `db:"updated_at"`
	}

	if err := r.db.GetContext(ctx, &dest, q, userID, agentID, toolSetID, toolID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("permission not found")
		}
		return nil, fmt.Errorf("selecting permission: %w", err)
	}

	perm := &Permission{
		UserID:    dest.UserID,
		AgentID:   dest.AgentID,
		ToolSetID: dest.ToolSetID,
		ToolID:    dest.ToolID,
		CreatedAt: dest.CreatedAt.Time,
		UpdatedAt: dest.UpdatedAt.Time,
	}

	if len(dest.Config) > 0 {
		perm.Config = string(dest.Config)
	}

	return perm, nil
}

// ListPermissions returns all individual tool permissions for an agent within a tool set.
func (r *PostgresRepository) ListPermissions(ctx context.Context, userID, agentID, toolSetID string) ([]Permission, error) {
	const q = `
		SELECT
			user_id, agent_id, tool_set_id, tool_id,
			COALESCE(config, '{}') as config, created_at, updated_at
		FROM agent_tool_permissions
		WHERE user_id = $1 AND agent_id = $2 AND tool_set_id = $3
	`

	var dests []struct {
		UserID    string          `db:"user_id"`
		AgentID   string          `db:"agent_id"`
		ToolSetID string          `db:"tool_set_id"`
		ToolID    string          `db:"tool_id"`
		Config    json.RawMessage `db:"config"`
		CreatedAt sql.NullTime    `db:"created_at"`
		UpdatedAt sql.NullTime    `db:"updated_at"`
	}

	if err := r.db.SelectContext(ctx, &dests, q, userID, agentID, toolSetID); err != nil {
		return nil, fmt.Errorf("selecting permissions: %w", err)
	}

	perms := make([]Permission, len(dests))
	for i, dest := range dests {
		perms[i] = Permission{
			UserID:    dest.UserID,
			AgentID:   dest.AgentID,
			ToolSetID: dest.ToolSetID,
			ToolID:    dest.ToolID,
			Config:    string(dest.Config),
			CreatedAt: dest.CreatedAt.Time,
			UpdatedAt: dest.UpdatedAt.Time,
		}
	}

	return perms, nil
}

// SavePermission saves or updates an individual tool permission.
func (r *PostgresRepository) SavePermission(ctx context.Context, permission *Permission) error {
	const q = `
		INSERT INTO agent_tool_permissions (
			user_id, agent_id, tool_set_id, tool_id, config, created_at, updated_at
		) VALUES (
			:user_id, :agent_id, :tool_set_id, :tool_id, :config, :created_at, :updated_at
		) ON CONFLICT (user_id, agent_id, tool_set_id, tool_id) DO UPDATE SET
			config = EXCLUDED.config,
			updated_at = EXCLUDED.updated_at
	`

	var config interface{} = permission.Config
	if permission.Config == "" {
		config = "{}"
	}

	params := map[string]interface{}{
		"user_id":     permission.UserID,
		"agent_id":    permission.AgentID,
		"tool_set_id": permission.ToolSetID,
		"tool_id":     permission.ToolID,
		"config":      config,
		"created_at":  permission.CreatedAt,
		"updated_at":  permission.UpdatedAt,
	}

	if _, err := r.db.NamedExecContext(ctx, q, params); err != nil {
		return fmt.Errorf("saving permission: %w", err)
	}

	return nil
}

// DeletePermission removes a specific agent-tool permission.
func (r *PostgresRepository) DeletePermission(ctx context.Context, userID, agentID, toolSetID, toolID string) error {
	const q = `
		DELETE FROM agent_tool_permissions
		WHERE user_id = $1 AND agent_id = $2 AND tool_set_id = $3 AND tool_id = $4
	`

	if _, err := r.db.ExecContext(ctx, q, userID, agentID, toolSetID, toolID); err != nil {
		return fmt.Errorf("deleting permission: %w", err)
	}

	return nil
}

// =============================================================================
// Tool Set Permission Methods
// =============================================================================

// GetToolSetPermission retrieves a specific agent-toolset permission.
func (r *PostgresRepository) GetToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) (*ToolSetPermission, error) {
	const q = `
		SELECT
			user_id, agent_id, tool_set_id,
			COALESCE(config, '{}') as config, created_at, updated_at
		FROM agent_tool_set_permissions
		WHERE user_id = $1 AND agent_id = $2 AND tool_set_id = $3
	`

	var dest struct {
		UserID    string          `db:"user_id"`
		AgentID   string          `db:"agent_id"`
		ToolSetID string          `db:"tool_set_id"`
		Config    json.RawMessage `db:"config"`
		CreatedAt sql.NullTime    `db:"created_at"`
		UpdatedAt sql.NullTime    `db:"updated_at"`
	}

	if err := r.db.GetContext(ctx, &dest, q, userID, agentID, toolSetID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("tool set permission not found")
		}
		return nil, fmt.Errorf("selecting tool set permission: %w", err)
	}

	perm := &ToolSetPermission{
		UserID:    dest.UserID,
		AgentID:   dest.AgentID,
		ToolSetID: dest.ToolSetID,
		CreatedAt: dest.CreatedAt.Time,
		UpdatedAt: dest.UpdatedAt.Time,
	}

	if len(dest.Config) > 0 {
		perm.Config = string(dest.Config)
	}

	return perm, nil
}

// ListToolSetPermissions returns all tool set permissions for an agent.
func (r *PostgresRepository) ListToolSetPermissions(ctx context.Context, userID, agentID string) ([]ToolSetPermission, error) {
	const q = `
		SELECT
			user_id, agent_id, tool_set_id,
			COALESCE(config, '{}') as config, created_at, updated_at
		FROM agent_tool_set_permissions
		WHERE user_id = $1 AND agent_id = $2
	`

	var dests []struct {
		UserID    string          `db:"user_id"`
		AgentID   string          `db:"agent_id"`
		ToolSetID string          `db:"tool_set_id"`
		Config    json.RawMessage `db:"config"`
		CreatedAt sql.NullTime    `db:"created_at"`
		UpdatedAt sql.NullTime    `db:"updated_at"`
	}

	if err := r.db.SelectContext(ctx, &dests, q, userID, agentID); err != nil {
		return nil, fmt.Errorf("selecting tool set permissions: %w", err)
	}

	perms := make([]ToolSetPermission, len(dests))
	for i, dest := range dests {
		perms[i] = ToolSetPermission{
			UserID:    dest.UserID,
			AgentID:   dest.AgentID,
			ToolSetID: dest.ToolSetID,
			Config:    string(dest.Config),
			CreatedAt: dest.CreatedAt.Time,
			UpdatedAt: dest.UpdatedAt.Time,
		}
	}

	return perms, nil
}

// SaveToolSetPermission saves or updates a tool set permission.
func (r *PostgresRepository) SaveToolSetPermission(ctx context.Context, permission *ToolSetPermission) error {
	const q = `
		INSERT INTO agent_tool_set_permissions (
			user_id, agent_id, tool_set_id, config, created_at, updated_at
		) VALUES (
			:user_id, :agent_id, :tool_set_id, :config, :created_at, :updated_at
		) ON CONFLICT (user_id, agent_id, tool_set_id) DO UPDATE SET
			config = EXCLUDED.config,
			updated_at = EXCLUDED.updated_at
	`

	var config interface{} = permission.Config
	if permission.Config == "" {
		config = "{}"
	}

	params := map[string]interface{}{
		"user_id":     permission.UserID,
		"agent_id":    permission.AgentID,
		"tool_set_id": permission.ToolSetID,
		"config":      config,
		"created_at":  permission.CreatedAt,
		"updated_at":  permission.UpdatedAt,
	}

	if _, err := r.db.NamedExecContext(ctx, q, params); err != nil {
		return fmt.Errorf("saving tool set permission: %w", err)
	}

	return nil
}

// DeleteToolSetPermission removes a specific agent-toolset permission.
func (r *PostgresRepository) DeleteToolSetPermission(ctx context.Context, userID, agentID, toolSetID string) error {
	const q = `
		DELETE FROM agent_tool_set_permissions
		WHERE user_id = $1 AND agent_id = $2 AND tool_set_id = $3
	`

	if _, err := r.db.ExecContext(ctx, q, userID, agentID, toolSetID); err != nil {
		return fmt.Errorf("deleting tool set permission: %w", err)
	}

	return nil
}

// Verify interface compliance.
var _ Repository = (*PostgresRepository)(nil)
