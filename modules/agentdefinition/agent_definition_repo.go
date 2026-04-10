package agentdefinition

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

// nullIfEmpty returns a sql.NullString that is NULL when the input is empty.
// Used for JSONB columns that should be NULL rather than empty string.
func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// agentSelectColumns is the standard column set for agent queries.
// graph, model_config, and model_override are included so composed and configured agents are fully loaded.
const agentSelectColumns = `id, user_id, name, description, system_prompt, icon, source_catalog_id,
    disallow_transfer_to_parent, disallow_transfer_to_peers, output_key, include_contents,
    global_instruction, COALESCE(input_schema::text, '') as input_schema,
    COALESCE(output_schema::text, '') as output_schema,
    COALESCE(graph::text, '') as graph,
    COALESCE(model_config::text, '') as model_config,
    COALESCE(model_override::text, '') as model_override,
    share_token, visibility,
    created_at, updated_at`

// postgresAgentDefinitionRepository implements AgentDefinitionRepository using PostgreSQL.
type postgresAgentDefinitionRepository struct {
	db *sqlx.DB
}

func newPostgresAgentDefinitionRepository(db *sqlx.DB) *postgresAgentDefinitionRepository {
	return &postgresAgentDefinitionRepository{db: db}
}

// hydrateAgent populates Graph, ModelConfig, and ModelOverride from their JSON columns after scanning.
func hydrateAgent(a *AgentDefinition) {
	_ = a.UnmarshalGraph()
	_ = a.UnmarshalModelConfig()
	_ = a.UnmarshalModelOverride()
}

func (r *postgresAgentDefinitionRepository) Create(ctx context.Context, agent *AgentDefinition) error {
	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}

	now := time.Now().UTC()
	agent.CreatedAt = now
	agent.UpdatedAt = now

	// Marshal composition graph, model config, and model override to JSON for storage.
	if err := agent.MarshalGraph(); err != nil {
		return err
	}
	if err := agent.MarshalModelConfig(); err != nil {
		return err
	}
	if err := agent.MarshalModelOverride(); err != nil {
		return err
	}

	query := `
INSERT INTO user_agents (id, user_id, name, description, system_prompt, icon, source_catalog_id,
    disallow_transfer_to_parent, disallow_transfer_to_peers, output_key, include_contents,
    global_instruction, input_schema, output_schema, graph, model_config, model_override,
    share_token, visibility, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`

	_, err := r.db.ExecContext(ctx, query,
		agent.ID,
		agent.UserID,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.Icon,
		agent.SourceCatalogID,
		agent.DisallowTransferToParent,
		agent.DisallowTransferToPeers,
		agent.OutputKey,
		agent.IncludeContents,
		agent.GlobalInstruction,
		nullIfEmpty(agent.InputSchemaJSON),
		nullIfEmpty(agent.OutputSchemaJSON),
		nullIfEmpty(agent.GraphJSON),
		nullIfEmpty(agent.ModelConfigJSON),
		nullIfEmpty(agent.ModelOverrideJSON),
		agent.ShareToken,
		coalesceVisibility(agent.Visibility),
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

	// Marshal composition graph, model config, and model override to JSON for storage.
	if err := agent.MarshalGraph(); err != nil {
		return err
	}
	if err := agent.MarshalModelConfig(); err != nil {
		return err
	}
	if err := agent.MarshalModelOverride(); err != nil {
		return err
	}

	query := `
UPDATE user_agents
SET name = $1, description = $2, system_prompt = $3,
    icon = $4, updated_at = $5,
    disallow_transfer_to_parent = $6, disallow_transfer_to_peers = $7,
    output_key = $8, include_contents = $9, global_instruction = $10,
    input_schema = $11, output_schema = $12, graph = $13, model_config = $14,
    model_override = $15
WHERE id = $16 AND user_id = $17`

	result, err := r.db.ExecContext(ctx, query,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.Icon,
		agent.UpdatedAt,
		agent.DisallowTransferToParent,
		agent.DisallowTransferToPeers,
		agent.OutputKey,
		agent.IncludeContents,
		agent.GlobalInstruction,
		nullIfEmpty(agent.InputSchemaJSON),
		nullIfEmpty(agent.OutputSchemaJSON),
		nullIfEmpty(agent.GraphJSON),
		nullIfEmpty(agent.ModelConfigJSON),
		nullIfEmpty(agent.ModelOverrideJSON),
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
	query := `SELECT ` + agentSelectColumns + `
FROM user_agents WHERE id = $1 AND user_id = $2`

	var agent AgentDefinition
	if err := r.db.GetContext(ctx, &agent, query, id, userID); err != nil {
		return nil, fmt.Errorf("getting agent definition: %w", err)
	}
	hydrateAgent(&agent)
	return &agent, nil
}

func (r *postgresAgentDefinitionRepository) ListByUser(ctx context.Context, userID string) ([]AgentDefinition, error) {
	query := `SELECT ` + agentSelectColumns + `
FROM user_agents WHERE user_id = $1 ORDER BY created_at ASC`

	var agents []AgentDefinition
	if err := r.db.SelectContext(ctx, &agents, query, userID); err != nil {
		return nil, fmt.Errorf("listing agent definitions: %w", err)
	}
	for i := range agents {
		hydrateAgent(&agents[i])
	}
	return agents, nil
}

func (r *postgresAgentDefinitionRepository) Search(ctx context.Context, userID, query string) ([]AgentDefinition, error) {
	sqlQuery := `SELECT ` + agentSelectColumns + `
FROM user_agents
WHERE user_id = $1 AND (name ILIKE $2 OR description ILIKE $2)
ORDER BY created_at ASC`

	pattern := "%" + query + "%"

	var agents []AgentDefinition
	if err := r.db.SelectContext(ctx, &agents, sqlQuery, userID, pattern); err != nil {
		return nil, fmt.Errorf("searching agent definitions: %w", err)
	}
	for i := range agents {
		hydrateAgent(&agents[i])
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

func (r *postgresAgentDefinitionRepository) GetByShareToken(ctx context.Context, token string) (*AgentDefinition, error) {
	if token == "" {
		return nil, fmt.Errorf("share token is required")
	}
	query := `SELECT ` + agentSelectColumns + `
FROM user_agents WHERE share_token = $1 AND visibility = 'shared'`

	var agent AgentDefinition
	if err := r.db.GetContext(ctx, &agent, query, token); err != nil {
		return nil, fmt.Errorf("agent not found for share token: %w", err)
	}
	hydrateAgent(&agent)
	return &agent, nil
}

func (r *postgresAgentDefinitionRepository) UpdateShareToken(ctx context.Context, id, userID, token, visibility string) error {
	query := `UPDATE user_agents SET share_token = $1, visibility = $2, updated_at = $3
WHERE id = $4 AND user_id = $5`

	result, err := r.db.ExecContext(ctx, query, token, visibility, time.Now().UTC(), id, userID)
	if err != nil {
		return fmt.Errorf("updating share token: %w", err)
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

// coalesceVisibility returns "private" for empty visibility values.
func coalesceVisibility(v string) string {
	if v == "" {
		return "private"
	}
	return v
}
