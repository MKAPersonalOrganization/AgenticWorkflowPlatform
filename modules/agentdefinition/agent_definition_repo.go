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
    COALESCE(parent_id, '') as parent_id,
    disallow_transfer_to_parent, disallow_transfer_to_peers, output_key, include_contents,
    global_instruction, COALESCE(input_schema::text, '') as input_schema,
    COALESCE(output_schema::text, '') as output_schema,
    COALESCE(graph::text, '') as graph,
    COALESCE(sub_agents::text, '') as sub_agents,
    COALESCE(agent_tools::text, '') as agent_tools,
    COALESCE(model_config::text, '') as model_config,
    COALESCE(model_override::text, '') as model_override,
    COALESCE(memory_policy::text, '') as memory_policy,
    COALESCE(artifact_policy::text, '') as artifact_policy,
    COALESCE(code_execution_policy::text, '') as code_execution_policy,
    COALESCE(streaming_policy::text, '') as streaming_policy,
    share_token, visibility,
    created_at, updated_at`

// postgresAgentDefinitionRepository implements AgentDefinitionRepository using PostgreSQL.
type postgresAgentDefinitionRepository struct {
	db *sqlx.DB
}

func newPostgresAgentDefinitionRepository(db *sqlx.DB) *postgresAgentDefinitionRepository {
	return &postgresAgentDefinitionRepository{db: db}
}

// hydrateAgent populates JSON-backed fields from the raw JSON columns after scanning.
func hydrateAgent(a *AgentDefinition) {
	_ = a.UnmarshalGraph()
	_ = a.UnmarshalSubAgents()
	_ = a.UnmarshalAgentTools()
	_ = a.UnmarshalModelConfig()
	_ = a.UnmarshalModelOverride()
	_ = a.UnmarshalMemoryPolicy()
	_ = a.UnmarshalArtifactPolicy()
	_ = a.UnmarshalCodeExecutionPolicy()
	_ = a.UnmarshalStreamingPolicy()
}

func (r *postgresAgentDefinitionRepository) Create(ctx context.Context, agent *AgentDefinition) error {
	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}

	now := time.Now().UTC()
	agent.CreatedAt = now
	agent.UpdatedAt = now

	// Marshal composition graph, sub_agents, agent_tools, model config, and model override to JSON for storage.
	if err := agent.MarshalGraph(); err != nil {
		return err
	}
	if err := agent.MarshalSubAgents(); err != nil {
		return err
	}
	if err := agent.MarshalAgentTools(); err != nil {
		return err
	}
	if err := agent.MarshalModelConfig(); err != nil {
		return err
	}
	if err := agent.MarshalModelOverride(); err != nil {
		return err
	}
	if err := agent.MarshalMemoryPolicy(); err != nil {
		return err
	}
	if err := agent.MarshalArtifactPolicy(); err != nil {
		return err
	}
	if err := agent.MarshalCodeExecutionPolicy(); err != nil {
		return err
	}
	if err := agent.MarshalStreamingPolicy(); err != nil {
		return err
	}

	query := `
INSERT INTO user_agents (id, user_id, name, description, system_prompt, icon, source_catalog_id,
    parent_id,
    disallow_transfer_to_parent, disallow_transfer_to_peers, output_key, include_contents,
    global_instruction, input_schema, output_schema, graph, sub_agents, agent_tools, model_config, model_override,
    memory_policy, artifact_policy, code_execution_policy, streaming_policy,
    share_token, visibility, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28)`

	_, err := r.db.ExecContext(ctx, query,
		agent.ID,
		agent.UserID,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.Icon,
		agent.SourceCatalogID,
		nullIfEmpty(agent.ParentID),
		agent.DisallowTransferToParent,
		agent.DisallowTransferToPeers,
		agent.OutputKey,
		agent.IncludeContents,
		agent.GlobalInstruction,
		nullIfEmpty(agent.InputSchemaJSON),
		nullIfEmpty(agent.OutputSchemaJSON),
		nullIfEmpty(agent.GraphJSON),
		nullIfEmpty(agent.SubAgentsJSON),
		nullIfEmpty(agent.AgentToolsJSON),
		nullIfEmpty(agent.ModelConfigJSON),
		nullIfEmpty(agent.ModelOverrideJSON),
		nullIfEmpty(agent.MemoryPolicyJSON),
		nullIfEmpty(agent.ArtifactPolicyJSON),
		nullIfEmpty(agent.CodeExecutionPolicyJSON),
		nullIfEmpty(agent.StreamingPolicyJSON),
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

	// Marshal composition graph, sub_agents, agent_tools, model config, and model override to JSON for storage.
	if err := agent.MarshalGraph(); err != nil {
		return err
	}
	if err := agent.MarshalSubAgents(); err != nil {
		return err
	}
	if err := agent.MarshalAgentTools(); err != nil {
		return err
	}
	if err := agent.MarshalModelConfig(); err != nil {
		return err
	}
	if err := agent.MarshalModelOverride(); err != nil {
		return err
	}
	if err := agent.MarshalMemoryPolicy(); err != nil {
		return err
	}
	if err := agent.MarshalArtifactPolicy(); err != nil {
		return err
	}
	if err := agent.MarshalCodeExecutionPolicy(); err != nil {
		return err
	}
	if err := agent.MarshalStreamingPolicy(); err != nil {
		return err
	}

	query := `
UPDATE user_agents
SET name = $1, description = $2, system_prompt = $3,
    icon = $4, parent_id = $5, updated_at = $6,
    disallow_transfer_to_parent = $7, disallow_transfer_to_peers = $8,
    output_key = $9, include_contents = $10, global_instruction = $11,
    input_schema = $12, output_schema = $13, graph = $14, sub_agents = $15,
    agent_tools = $16, model_config = $17, model_override = $18,
    memory_policy = $19, artifact_policy = $20, code_execution_policy = $21,
    streaming_policy = $22
WHERE id = $23 AND user_id = $24`

	result, err := r.db.ExecContext(ctx, query,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.Icon,
		nullIfEmpty(agent.ParentID),
		agent.UpdatedAt,
		agent.DisallowTransferToParent,
		agent.DisallowTransferToPeers,
		agent.OutputKey,
		agent.IncludeContents,
		agent.GlobalInstruction,
		nullIfEmpty(agent.InputSchemaJSON),
		nullIfEmpty(agent.OutputSchemaJSON),
		nullIfEmpty(agent.GraphJSON),
		nullIfEmpty(agent.SubAgentsJSON),
		nullIfEmpty(agent.AgentToolsJSON),
		nullIfEmpty(agent.ModelConfigJSON),
		nullIfEmpty(agent.ModelOverrideJSON),
		nullIfEmpty(agent.MemoryPolicyJSON),
		nullIfEmpty(agent.ArtifactPolicyJSON),
		nullIfEmpty(agent.CodeExecutionPolicyJSON),
		nullIfEmpty(agent.StreamingPolicyJSON),
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

// SetParent sets the parent_id on a sub-agent to establish workflow ownership.
func (r *postgresAgentDefinitionRepository) SetParent(ctx context.Context, agentID, parentID, userID string) error {
	query := `UPDATE user_agents SET parent_id = $1, updated_at = $2
WHERE id = $3 AND user_id = $4`
	_, err := r.db.ExecContext(ctx, query, parentID, time.Now().UTC(), agentID, userID)
	if err != nil {
		return fmt.Errorf("setting parent_id: %w", err)
	}
	return nil
}

// DeleteByParent deletes all agents owned by a workflow (parent_id = parentID).
func (r *postgresAgentDefinitionRepository) DeleteByParent(ctx context.Context, parentID, userID string) error {
	query := `DELETE FROM user_agents WHERE parent_id = $1 AND user_id = $2`
	_, err := r.db.ExecContext(ctx, query, parentID, userID)
	if err != nil {
		return fmt.Errorf("deleting sub-agents by parent_id: %w", err)
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
