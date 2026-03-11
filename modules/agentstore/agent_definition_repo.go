package agentstore

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// postgresAgentDefinitionRepository implements AgentDefinitionRepository using PostgreSQL.
type postgresAgentDefinitionRepository struct {
	db *sqlx.DB
}

func newPostgresAgentDefinitionRepository(db *sqlx.DB) *postgresAgentDefinitionRepository {
	return &postgresAgentDefinitionRepository{db: db}
}

func (r *postgresAgentDefinitionRepository) Create(ctx context.Context, agent *AgentDefinition) error {
	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}

	now := time.Now().UTC()
	agent.CreatedAt = now
	agent.UpdatedAt = now

	query := `
INSERT INTO user_agents (id, user_id, name, description, system_prompt, icon, source_catalog_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.ExecContext(ctx, query,
		agent.ID,
		agent.UserID,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.Icon,
		agent.SourceCatalogID,
		agent.CreatedAt,
		agent.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting agent definition: %w", err)
	}

	return nil
}

func (r *postgresAgentDefinitionRepository) Update(ctx context.Context, agent *AgentDefinition) error {
	agent.UpdatedAt = time.Now().UTC()

	query := `
UPDATE user_agents
SET name = $1, description = $2, system_prompt = $3,
    icon = $4, updated_at = $5
WHERE id = $6 AND user_id = $7`

	result, err := r.db.ExecContext(ctx, query,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.Icon,
		agent.UpdatedAt,
		agent.ID,
		agent.UserID,
	)
	if err != nil {
		return fmt.Errorf("updating agent definition: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("agent not found or not owned by user")
	}

	return nil
}

func (r *postgresAgentDefinitionRepository) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM user_agents WHERE id = $1 AND user_id = $2`

	result, err := r.db.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("deleting agent definition: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("agent not found or not owned by user")
	}

	return nil
}

func (r *postgresAgentDefinitionRepository) GetByID(ctx context.Context, id, userID string) (*AgentDefinition, error) {
	query := `SELECT id, user_id, name, description, system_prompt, icon, source_catalog_id, created_at, updated_at
FROM user_agents WHERE id = $1 AND user_id = $2`

	var agent AgentDefinition
	if err := r.db.GetContext(ctx, &agent, query, id, userID); err != nil {
		return nil, fmt.Errorf("getting agent definition: %w", err)
	}

	return &agent, nil
}

func (r *postgresAgentDefinitionRepository) ListByUser(ctx context.Context, userID string) ([]AgentDefinition, error) {
	query := `SELECT id, user_id, name, description, system_prompt, icon, source_catalog_id, created_at, updated_at
FROM user_agents WHERE user_id = $1 ORDER BY created_at ASC`

	var agents []AgentDefinition
	if err := r.db.SelectContext(ctx, &agents, query, userID); err != nil {
		return nil, fmt.Errorf("listing agent definitions: %w", err)
	}

	return agents, nil
}

func (r *postgresAgentDefinitionRepository) Search(ctx context.Context, userID, query string) ([]AgentDefinition, error) {
	sqlQuery := `SELECT id, user_id, name, description, system_prompt, icon, source_catalog_id, created_at, updated_at
FROM user_agents
WHERE user_id = $1 AND (name ILIKE $2 OR description ILIKE $2)
ORDER BY created_at ASC`

	pattern := "%" + query + "%"

	var agents []AgentDefinition
	if err := r.db.SelectContext(ctx, &agents, sqlQuery, userID, pattern); err != nil {
		return nil, fmt.Errorf("searching agent definitions: %w", err)
	}

	return agents, nil
}




func (r *postgresAgentDefinitionRepository) ListAddedCatalogIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT DISTINCT source_catalog_id FROM user_agents WHERE user_id = $1 AND source_catalog_id != ''`

	var ids []string
	if err := r.db.SelectContext(ctx, &ids, query, userID); err != nil {
		return nil, fmt.Errorf("listing added catalog IDs: %w", err)
	}
	return ids, nil
}

func (r *postgresAgentDefinitionRepository) ExistsByCatalogID(ctx context.Context, userID, catalogID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM user_agents WHERE user_id = $1 AND source_catalog_id = $2)`
	if err := r.db.QueryRowContext(ctx, query, userID, catalogID).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking catalog agent existence: %w", err)
	}
	return exists, nil
}
