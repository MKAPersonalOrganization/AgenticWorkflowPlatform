package agentdefinition

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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

// jsonColumn describes a JSONB-backed column on user_agents: its SQL name
// and the accessor for the already-marshalled JSON string on the aggregate.
// agentJSONColumns is the single source of truth for the order and
// membership of JSON columns in SELECT / INSERT / UPDATE — adding a new
// policy means appending one entry here (plus the Marshal/Unmarshal pair
// in types.go) instead of editing three separate SQL literals.
type jsonColumn struct {
	name string
	get  func(*AgentDefinition) string
}

var agentJSONColumns = []jsonColumn{
	{"input_schema", func(a *AgentDefinition) string { return a.InputSchemaJSON }},
	{"output_schema", func(a *AgentDefinition) string { return a.OutputSchemaJSON }},
	{"graph", func(a *AgentDefinition) string { return a.GraphJSON }},
	{"sub_agents", func(a *AgentDefinition) string { return a.SubAgentsJSON }},
	{"agent_tools", func(a *AgentDefinition) string { return a.AgentToolsJSON }},
	{"model_config", func(a *AgentDefinition) string { return a.ModelConfigJSON }},
	{"model_override", func(a *AgentDefinition) string { return a.ModelOverrideJSON }},
	{"memory_policy", func(a *AgentDefinition) string { return a.MemoryPolicyJSON }},
	{"artifact_policy", func(a *AgentDefinition) string { return a.ArtifactPolicyJSON }},
	{"code_execution_policy", func(a *AgentDefinition) string { return a.CodeExecutionPolicyJSON }},
	{"streaming_policy", func(a *AgentDefinition) string { return a.StreamingPolicyJSON }},
}

// jsonColumnArgs returns the JSON column values as nullIfEmpty-wrapped
// ExecContext args, in agentJSONColumns order.
func jsonColumnArgs(a *AgentDefinition) []any {
	args := make([]any, len(agentJSONColumns))
	for i, c := range agentJSONColumns {
		args[i] = nullIfEmpty(c.get(a))
	}
	return args
}

// jsonSelectClause returns the SELECT fragment that decodes each JSONB
// column back to its text form with COALESCE-to-empty-string, so scan
// targets receive a plain string.
func jsonSelectClause() string {
	parts := make([]string, len(agentJSONColumns))
	for i, c := range agentJSONColumns {
		parts[i] = fmt.Sprintf("COALESCE(%s::text, '') as %s", c.name, c.name)
	}
	return strings.Join(parts, ",\n    ")
}

// jsonColumnList joins the column names with ", " for INSERT clauses.
func jsonColumnList() string {
	parts := make([]string, len(agentJSONColumns))
	for i, c := range agentJSONColumns {
		parts[i] = c.name
	}
	return strings.Join(parts, ", ")
}

// agentSelectColumns is the standard column set for agent queries.
// JSON columns are composed from agentJSONColumns so adding a policy
// automatically widens the SELECT.
var agentSelectColumns = `id, user_id, name, description, system_prompt, icon, source_catalog_id,
    COALESCE(parent_id, '') as parent_id,
    disallow_transfer_to_parent, disallow_transfer_to_peers, output_key, include_contents,
    global_instruction,
    ` + jsonSelectClause() + `,
    share_token, visibility,
    created_at, updated_at`

// postgresAgentDefinitionRepository implements AgentDefinitionRepository using PostgreSQL.
type postgresAgentDefinitionRepository struct {
	db *sqlx.DB
}

func newPostgresAgentDefinitionRepository(db *sqlx.DB) *postgresAgentDefinitionRepository {
	return &postgresAgentDefinitionRepository{db: db}
}

// hydrateAgent populates JSON-backed fields from the raw JSON columns after
// scanning. Delegates to AgentDefinition.HydrateJSON so new JSONB columns
// plug in via jsonUnmarshalers() without touching every caller.
func hydrateAgent(a *AgentDefinition) { a.HydrateJSON() }

// buildInsertQuery renders the fixed-shape INSERT into user_agents,
// weaving agentJSONColumns into the column list and $-placeholder tail.
func buildInsertQuery() string {
	// 13 leading scalar placeholders + N JSON + 4 trailing scalar.
	total := 13 + len(agentJSONColumns) + 4
	placeholders := make([]string, total)
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	return fmt.Sprintf(`
INSERT INTO user_agents (id, user_id, name, description, system_prompt, icon, source_catalog_id,
    parent_id,
    disallow_transfer_to_parent, disallow_transfer_to_peers, output_key, include_contents,
    global_instruction, %s,
    share_token, visibility, created_at, updated_at)
VALUES (%s)`, jsonColumnList(), strings.Join(placeholders, ", "))
}

var insertAgentQuery = buildInsertQuery()

func (r *postgresAgentDefinitionRepository) Create(ctx context.Context, agent *AgentDefinition) error {
	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}

	now := time.Now().UTC()
	agent.CreatedAt = now
	agent.UpdatedAt = now

	// Encode every JSONB column in one step — jsonMarshalers() on the
	// AgentDefinition is the single source of truth for which columns
	// exist, so adding a policy never requires editing this function.
	if err := agent.MarshalAllJSON(); err != nil {
		return err
	}

	args := make([]any, 0, 13+len(agentJSONColumns)+4)
	args = append(args,
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
	)
	args = append(args, jsonColumnArgs(agent)...)
	args = append(args,
		agent.ShareToken,
		coalesceVisibility(agent.Visibility),
		agent.CreatedAt,
		agent.UpdatedAt,
	)

	if _, err := r.db.ExecContext(ctx, insertAgentQuery, args...); err != nil {
		return fmt.Errorf("inserting agent definition: %w", err)
	}
	return nil
}

// buildUpdateQuery renders the UPDATE user_agents SET ... WHERE clause
// using agentJSONColumns for the JSON tail. Param positions: $1..$11 are
// the scalar SET assignments, $12..$12+N are the JSON assignments, and
// the final two are the WHERE id/user_id.
func buildUpdateQuery() string {
	jsonAssigns := make([]string, len(agentJSONColumns))
	for i, c := range agentJSONColumns {
		jsonAssigns[i] = fmt.Sprintf("%s = $%d", c.name, 12+i)
	}
	idParam := 12 + len(agentJSONColumns)
	userParam := idParam + 1
	return fmt.Sprintf(`
UPDATE user_agents
SET name = $1, description = $2, system_prompt = $3,
    icon = $4, parent_id = $5, updated_at = $6,
    disallow_transfer_to_parent = $7, disallow_transfer_to_peers = $8,
    output_key = $9, include_contents = $10, global_instruction = $11,
    %s
WHERE id = $%d AND user_id = $%d`, strings.Join(jsonAssigns, ",\n    "), idParam, userParam)
}

var updateAgentQuery = buildUpdateQuery()

func (r *postgresAgentDefinitionRepository) Update(ctx context.Context, agent *AgentDefinition) error {
	agent.UpdatedAt = time.Now().UTC()

	// Encode every JSONB column in one step — see Create.
	if err := agent.MarshalAllJSON(); err != nil {
		return err
	}

	args := make([]any, 0, 11+len(agentJSONColumns)+2)
	args = append(args,
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
	)
	args = append(args, jsonColumnArgs(agent)...)
	args = append(args, agent.ID, agent.UserID)

	result, err := r.db.ExecContext(ctx, updateAgentQuery, args...)
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
