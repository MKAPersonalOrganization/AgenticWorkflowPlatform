package skillstore

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// ─────────────────────────────────────────────────────────────────────────────
// Skill-Agent Permission Repository (PostgreSQL)
// ─────────────────────────────────────────────────────────────────────────────

type postgresSkillPermissionRepository struct {
	db *sqlx.DB
}

func newPostgresSkillPermissionRepository(db *sqlx.DB) *postgresSkillPermissionRepository {
	return &postgresSkillPermissionRepository{db: db}
}

func (r *postgresSkillPermissionRepository) Grant(ctx context.Context, perm *SkillAgentPermission) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO skill_agent_permissions (user_id, agent_id, skill_id, enabled, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id, agent_id, skill_id)
		DO UPDATE SET enabled = EXCLUDED.enabled`,
		perm.UserID, perm.AgentID, perm.SkillID, perm.Enabled)
	if err != nil {
		return fmt.Errorf("granting skill permission: %w", err)
	}
	return nil
}

func (r *postgresSkillPermissionRepository) Revoke(ctx context.Context, userID, agentID, skillID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM skill_agent_permissions
		WHERE user_id = $1 AND agent_id = $2 AND skill_id = $3`,
		userID, agentID, skillID)
	if err != nil {
		return fmt.Errorf("revoking skill permission: %w", err)
	}
	return nil
}

func (r *postgresSkillPermissionRepository) ListBySkill(ctx context.Context, userID, skillID string) ([]SkillAgentPermission, error) {
	var perms []SkillAgentPermission
	err := r.db.SelectContext(ctx, &perms, `
		SELECT user_id, agent_id, skill_id, enabled, created_at
		FROM skill_agent_permissions
		WHERE user_id = $1 AND skill_id = $2
		ORDER BY created_at ASC`, userID, skillID)
	if err != nil {
		return nil, fmt.Errorf("listing skill permissions: %w", err)
	}
	return perms, nil
}

func (r *postgresSkillPermissionRepository) ListByAgent(ctx context.Context, userID, agentID string) ([]SkillAgentPermission, error) {
	var perms []SkillAgentPermission
	err := r.db.SelectContext(ctx, &perms, `
		SELECT user_id, agent_id, skill_id, enabled, created_at
		FROM skill_agent_permissions
		WHERE user_id = $1 AND agent_id = $2 AND enabled = TRUE
		ORDER BY created_at ASC`, userID, agentID)
	if err != nil {
		return nil, fmt.Errorf("listing agent skill permissions: %w", err)
	}
	return perms, nil
}

func (r *postgresSkillPermissionRepository) IsGranted(ctx context.Context, userID, agentID, skillID string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
		SELECT EXISTS(
			SELECT 1 FROM skill_agent_permissions
			WHERE user_id = $1 AND agent_id = $2 AND skill_id = $3 AND enabled = TRUE
		)`, userID, agentID, skillID)
	if err != nil {
		return false, fmt.Errorf("checking skill permission: %w", err)
	}
	return exists, nil
}

func (r *postgresSkillPermissionRepository) ListGrantedSkillIDs(ctx context.Context, userID, agentID string) ([]string, error) {
	var ids []string
	err := r.db.SelectContext(ctx, &ids, `
		SELECT skill_id FROM skill_agent_permissions
		WHERE user_id = $1 AND agent_id = $2 AND enabled = TRUE
		ORDER BY created_at ASC`, userID, agentID)
	if err != nil {
		return nil, fmt.Errorf("listing granted skill IDs: %w", err)
	}
	return ids, nil
}
