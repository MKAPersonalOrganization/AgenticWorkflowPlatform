package skillstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// ─────────────────────────────────────────────────────────────────────────────
// User Skill Repository (PostgreSQL)
// ─────────────────────────────────────────────────────────────────────────────

type postgresUserSkillRepository struct {
	db *sqlx.DB
}

func newPostgresUserSkillRepository(db *sqlx.DB) *postgresUserSkillRepository {
	return &postgresUserSkillRepository{db: db}
}

func (r *postgresUserSkillRepository) Create(ctx context.Context, skill *UserSkill) error {
	if skill.ID == "" {
		skill.ID = uuid.New().String()
	}
	now := time.Now()
	skill.CreatedAt = now
	skill.UpdatedAt = now

	if skill.Resources == "" {
		skill.Resources = "{}"
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_skills (id, user_id, name, description, instructions, category, icon, is_custom, source_catalog_id, resources, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		skill.ID, skill.UserID, skill.Name, skill.Description, skill.Instructions,
		skill.Category, skill.Icon, skill.IsCustom, skill.SourceCatalogID,
		skill.Resources, skill.CreatedAt, skill.UpdatedAt)
	if err != nil {
		return fmt.Errorf("inserting user skill: %w", err)
	}
	return nil
}

func (r *postgresUserSkillRepository) Update(ctx context.Context, skill *UserSkill) error {
	skill.UpdatedAt = time.Now()

	result, err := r.db.ExecContext(ctx, `
		UPDATE user_skills
		SET name = $1, description = $2, instructions = $3, category = $4, icon = $5, resources = $6, updated_at = $7
		WHERE id = $8 AND user_id = $9`,
		skill.Name, skill.Description, skill.Instructions, skill.Category,
		skill.Icon, skill.Resources, skill.UpdatedAt, skill.ID, skill.UserID)
	if err != nil {
		return fmt.Errorf("updating user skill: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("skill not found or not owned by user")
	}
	return nil
}

func (r *postgresUserSkillRepository) Delete(ctx context.Context, id, userID string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM user_skills WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("deleting user skill: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("skill not found or not owned by user")
	}
	return nil
}

func (r *postgresUserSkillRepository) GetByID(ctx context.Context, id, userID string) (*UserSkill, error) {
	var skill UserSkill
	err := r.db.QueryRowxContext(ctx, `
		SELECT id, user_id, name, description, instructions, category, icon, is_custom, source_catalog_id, resources, created_at, updated_at
		FROM user_skills WHERE id = $1 AND user_id = $2`, id, userID).
		Scan(&skill.ID, &skill.UserID, &skill.Name, &skill.Description, &skill.Instructions,
			&skill.Category, &skill.Icon, &skill.IsCustom, &skill.SourceCatalogID,
			&skill.Resources, &skill.CreatedAt, &skill.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("skill not found: %s", id)
		}
		return nil, fmt.Errorf("getting user skill: %w", err)
	}
	return &skill, nil
}

func (r *postgresUserSkillRepository) GetByName(ctx context.Context, userID, name string) (*UserSkill, error) {
	var skill UserSkill
	err := r.db.QueryRowxContext(ctx, `
		SELECT id, user_id, name, description, instructions, category, icon, is_custom, source_catalog_id, resources, created_at, updated_at
		FROM user_skills WHERE user_id = $1 AND name = $2`, userID, name).
		Scan(&skill.ID, &skill.UserID, &skill.Name, &skill.Description, &skill.Instructions,
			&skill.Category, &skill.Icon, &skill.IsCustom, &skill.SourceCatalogID,
			&skill.Resources, &skill.CreatedAt, &skill.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("skill not found: %s", name)
		}
		return nil, fmt.Errorf("getting user skill by name: %w", err)
	}
	return &skill, nil
}

func (r *postgresUserSkillRepository) ListByUser(ctx context.Context, userID string) ([]UserSkill, error) {
	var skills []UserSkill
	err := r.db.SelectContext(ctx, &skills, `
		SELECT id, user_id, name, description, instructions, category, icon, is_custom, source_catalog_id, resources, created_at, updated_at
		FROM user_skills WHERE user_id = $1 ORDER BY name ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing user skills: %w", err)
	}
	return skills, nil
}

func (r *postgresUserSkillRepository) Search(ctx context.Context, userID, query string) ([]UserSkill, error) {
	pattern := "%" + query + "%"
	var skills []UserSkill
	err := r.db.SelectContext(ctx, &skills, `
		SELECT id, user_id, name, description, instructions, category, icon, is_custom, source_catalog_id, resources, created_at, updated_at
		FROM user_skills WHERE user_id = $1 AND (name ILIKE $2 OR description ILIKE $2 OR category ILIKE $2)
		ORDER BY name ASC`, userID, pattern)
	if err != nil {
		return nil, fmt.Errorf("searching user skills: %w", err)
	}
	return skills, nil
}

func (r *postgresUserSkillRepository) ListAddedCatalogIDs(ctx context.Context, userID string) ([]string, error) {
	var ids []string
	err := r.db.SelectContext(ctx, &ids, `
		SELECT source_catalog_id FROM user_skills
		WHERE user_id = $1 AND source_catalog_id != ''`, userID)
	if err != nil {
		return nil, fmt.Errorf("listing added catalog IDs: %w", err)
	}
	return ids, nil
}

func (r *postgresUserSkillRepository) ExistsByCatalogID(ctx context.Context, userID, catalogID string) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `
		SELECT EXISTS(SELECT 1 FROM user_skills WHERE user_id = $1 AND source_catalog_id = $2)`,
		userID, catalogID)
	if err != nil {
		return false, fmt.Errorf("checking catalog ID existence: %w", err)
	}
	return exists, nil
}

func (r *postgresUserSkillRepository) CountByUser(ctx context.Context, userID string) (int, error) {
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM user_skills WHERE user_id = $1`, userID)
	if err != nil {
		return 0, fmt.Errorf("counting user skills: %w", err)
	}
	return count, nil
}
