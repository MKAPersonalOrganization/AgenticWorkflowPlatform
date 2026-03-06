package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"school-with-ai/services/workspacepermissions"

	"github.com/jmoiron/sqlx"
)

// PostgresWorkspacePermissionsRepository implements Workspace Permissions Repository using PostgreSQL.
type PostgresWorkspacePermissionsRepository struct {
	db *sqlx.DB
}

// NewPostgresWorkspacePermissionsRepository creates a new PostgresWorkspacePermissionsRepository.
func NewPostgresWorkspacePermissionsRepository(db *sqlx.DB) *PostgresWorkspacePermissionsRepository {
	return &PostgresWorkspacePermissionsRepository{
		db: db,
	}
}

// GetPermission retrieves a specific agent-workspace permission.
func (r *PostgresWorkspacePermissionsRepository) GetPermission(ctx context.Context, userID, agentID, workspaceID string) (*workspacepermissions.WorkspacePermission, error) {
	const q = `
		SELECT * FROM agent_workspace_permissions 
		WHERE user_id = $1 AND agent_id = $2 AND workspace_id = $3
	`

	var dest struct {
		UserID      string       `db:"user_id"`
		AgentID     string       `db:"agent_id"`
		WorkspaceID string       `db:"workspace_id"`
		Enabled     bool         `db:"enabled"`
		CreatedAt   sql.NullTime `db:"created_at"`
		UpdatedAt   sql.NullTime `db:"updated_at"`
	}

	if err := r.db.GetContext(ctx, &dest, q, userID, agentID, workspaceID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil if not found, consistent with service expectation (check service.go)
		}
		return nil, fmt.Errorf("selecting workspace permission: %w", err)
	}

	perm := &workspacepermissions.WorkspacePermission{
		UserID:      dest.UserID,
		AgentID:     dest.AgentID,
		WorkspaceID: dest.WorkspaceID,
		Enabled:     dest.Enabled,
		CreatedAt:   dest.CreatedAt.Time,
		UpdatedAt:   dest.UpdatedAt.Time,
	}

	return perm, nil
}

// ListPermissionsByAgent returns all workspace permissions for an agent.
func (r *PostgresWorkspacePermissionsRepository) ListPermissionsByAgent(ctx context.Context, userID, agentID string) ([]*workspacepermissions.WorkspacePermission, error) {
	const q = `
		SELECT * FROM agent_workspace_permissions 
		WHERE user_id = $1 AND agent_id = $2
	`

	var dests []struct {
		UserID      string       `db:"user_id"`
		AgentID     string       `db:"agent_id"`
		WorkspaceID string       `db:"workspace_id"`
		Enabled     bool         `db:"enabled"`
		CreatedAt   sql.NullTime `db:"created_at"`
		UpdatedAt   sql.NullTime `db:"updated_at"`
	}

	if err := r.db.SelectContext(ctx, &dests, q, userID, agentID); err != nil {
		return nil, fmt.Errorf("selecting workspace permissions: %w", err)
	}

	perms := make([]*workspacepermissions.WorkspacePermission, len(dests))
	for i, dest := range dests {
		perms[i] = &workspacepermissions.WorkspacePermission{
			UserID:      dest.UserID,
			AgentID:     dest.AgentID,
			WorkspaceID: dest.WorkspaceID,
			Enabled:     dest.Enabled,
			CreatedAt:   dest.CreatedAt.Time,
			UpdatedAt:   dest.UpdatedAt.Time,
		}
	}

	return perms, nil
}

// SavePermission saves or updates a workspace permission.
func (r *PostgresWorkspacePermissionsRepository) SavePermission(ctx context.Context, perm *workspacepermissions.WorkspacePermission) error {
	const q = `
		INSERT INTO agent_workspace_permissions (
			user_id, agent_id, workspace_id, enabled, created_at, updated_at
		) VALUES (
			:user_id, :agent_id, :workspace_id, :enabled, :created_at, :updated_at
		) ON CONFLICT (user_id, agent_id, workspace_id) DO UPDATE SET
			enabled = EXCLUDED.enabled,
			updated_at = EXCLUDED.updated_at
	`

	// Use map for named parameters to avoid dependency on struct tags
	params := map[string]interface{}{
		"user_id":      perm.UserID,
		"agent_id":     perm.AgentID,
		"workspace_id": perm.WorkspaceID,
		"enabled":      perm.Enabled,
		"created_at":   perm.CreatedAt,
		"updated_at":   perm.UpdatedAt,
	}

	if _, err := r.db.NamedExecContext(ctx, q, params); err != nil {
		return fmt.Errorf("saving workspace permission: %w", err)
	}

	return nil
}

// DeletePermission removes a specific agent-workspace permission.
func (r *PostgresWorkspacePermissionsRepository) DeletePermission(ctx context.Context, userID, agentID, workspaceID string) error {
	const q = `
		DELETE FROM agent_workspace_permissions 
		WHERE user_id = $1 AND agent_id = $2 AND workspace_id = $3
	`

	if _, err := r.db.ExecContext(ctx, q, userID, agentID, workspaceID); err != nil {
		return fmt.Errorf("deleting workspace permission: %w", err)
	}

	return nil
}
