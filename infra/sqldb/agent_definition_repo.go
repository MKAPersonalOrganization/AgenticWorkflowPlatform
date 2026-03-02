package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"school-with-ai/domain"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// PostgresAgentDefinitionRepository implements domain.AgentDefinitionRepository using PostgreSQL.
type PostgresAgentDefinitionRepository struct {
	db *sqlx.DB
}

// NewPostgresAgentDefinitionRepository creates a new PostgreSQL-backed agent definition repository.
func NewPostgresAgentDefinitionRepository(db *sqlx.DB) *PostgresAgentDefinitionRepository {
	return &PostgresAgentDefinitionRepository{db: db}
}

// Create persists a new agent definition.
func (r *PostgresAgentDefinitionRepository) Create(ctx context.Context, agent *domain.AgentDefinition) error {
	if agent.ID == "" {
		agent.ID = uuid.New().String()
	}

	now := time.Now().UTC()
	agent.CreatedAt = now
	agent.UpdatedAt = now

	query := `
INSERT INTO user_agents (id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, icon, source_catalog_id, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`

	_, err := r.db.ExecContext(ctx, query,
		agent.ID,
		agent.UserID,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.ModelID,
		agent.MaxIterations,
		agent.Temperature,
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

// Update modifies an existing agent definition.
func (r *PostgresAgentDefinitionRepository) Update(ctx context.Context, agent *domain.AgentDefinition) error {
	agent.UpdatedAt = time.Now().UTC()

	query := `
UPDATE user_agents
SET name = $1, description = $2, system_prompt = $3, model_id = $4,
    max_iterations = $5, temperature = $6, icon = $7, updated_at = $8
WHERE id = $9 AND user_id = $10`

	result, err := r.db.ExecContext(ctx, query,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.ModelID,
		agent.MaxIterations,
		agent.Temperature,
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

// Delete removes an agent definition by ID. Any agent can be deleted.
func (r *PostgresAgentDefinitionRepository) Delete(ctx context.Context, id, userID string) error {
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

// GetByID retrieves a single agent by ID.
func (r *PostgresAgentDefinitionRepository) GetByID(ctx context.Context, id, userID string) (*domain.AgentDefinition, error) {
	query := `SELECT id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, icon, source_catalog_id, created_at, updated_at
FROM user_agents WHERE id = $1 AND user_id = $2`

	var agent domain.AgentDefinition
	if err := r.db.GetContext(ctx, &agent, query, id, userID); err != nil {
		return nil, fmt.Errorf("getting agent definition: %w", err)
	}

	return &agent, nil
}

// ListByUser returns all agents created by a specific user, ordered by creation time.
func (r *PostgresAgentDefinitionRepository) ListByUser(ctx context.Context, userID string) ([]domain.AgentDefinition, error) {
	query := `SELECT id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, icon, source_catalog_id, created_at, updated_at
FROM user_agents WHERE user_id = $1 ORDER BY created_at ASC`

	var agents []domain.AgentDefinition
	if err := r.db.SelectContext(ctx, &agents, query, userID); err != nil {
		return nil, fmt.Errorf("listing agent definitions: %w", err)
	}

	return agents, nil
}

// Search finds agents matching a query string within a user's agents.
func (r *PostgresAgentDefinitionRepository) Search(ctx context.Context, userID, query string) ([]domain.AgentDefinition, error) {
	sqlQuery := `SELECT id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, icon, source_catalog_id, created_at, updated_at
FROM user_agents
WHERE user_id = $1 AND (name ILIKE $2 OR description ILIKE $2)
ORDER BY created_at ASC`

	pattern := "%" + query + "%"

	var agents []domain.AgentDefinition
	if err := r.db.SelectContext(ctx, &agents, sqlQuery, userID, pattern); err != nil {
		return nil, fmt.Errorf("searching agent definitions: %w", err)
	}

	return agents, nil
}

// EnsureInitialAgent creates a starter agent for the user if they have zero
// agents. Returns the user's first agent (by creation time). There is no
// "default" concept — this is simply a convenience so new users don't start
// with an empty agent list.
func (r *PostgresAgentDefinitionRepository) EnsureInitialAgent(ctx context.Context, userID string) (*domain.AgentDefinition, error) {
	// Fast path — check if any agent exists.
	var count int
	err := r.db.GetContext(ctx, &count, `SELECT COUNT(*) FROM user_agents WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("counting agents: %w", err)
	}

	if count == 0 {
		// Create the starter agent using the template.
		tmpl := domain.InitialAgentTemplate(userID)
		if err := r.Create(ctx, &tmpl); err != nil {
			return nil, fmt.Errorf("creating initial agent: %w", err)
		}

		// Seed tool permissions so the agent is immediately usable.
		r.seedInitialToolPermissions(ctx, userID, tmpl.ID)

		return &tmpl, nil
	}

	// Return the first agent (oldest).
	first, err := r.getFirstAgent(ctx, userID)
	if err != nil {
		return nil, err
	}
	return first, nil
}

// getFirstAgent returns the oldest agent for a user.
func (r *PostgresAgentDefinitionRepository) getFirstAgent(ctx context.Context, userID string) (*domain.AgentDefinition, error) {
	query := `SELECT id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, icon, source_catalog_id, created_at, updated_at
FROM user_agents WHERE user_id = $1 ORDER BY created_at ASC LIMIT 1`

	var agent domain.AgentDefinition
	if err := r.db.GetContext(ctx, &agent, query, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("no agents found for user")
		}
		return nil, fmt.Errorf("getting first agent: %w", err)
	}
	return &agent, nil
}

// initialToolSets are the tool sets seeded for a user's first agent.
var initialToolSets = []string{
	"cognitive-set",
	"calculator-set",
	"memory-set",
	"document-set",
	"knowledge-base-set",
}

// seedInitialToolPermissions inserts tool set permissions for a newly created
// agent. Uses ON CONFLICT DO NOTHING for idempotency.
func (r *PostgresAgentDefinitionRepository) seedInitialToolPermissions(ctx context.Context, userID, agentID string) {
	query := `
INSERT INTO agent_tool_set_permissions (user_id, agent_id, tool_set_id, enabled, created_at, updated_at)
SELECT $1, $2, ts.id, TRUE, NOW(), NOW()
FROM tool_sets ts
WHERE ts.id = ANY($3)
ON CONFLICT (user_id, agent_id, tool_set_id) DO NOTHING`

	if _, err := r.db.ExecContext(ctx, query, userID, agentID, pq.Array(initialToolSets)); err != nil {
		// Non-fatal — tool_sets table may not have these IDs yet.
		_ = err
	}
}

// ListAddedCatalogIDs returns the distinct catalog entry IDs that the user has
// already forked. Empty source_catalog_id values are excluded.
func (r *PostgresAgentDefinitionRepository) ListAddedCatalogIDs(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT DISTINCT source_catalog_id FROM user_agents WHERE user_id = $1 AND source_catalog_id != ''`

	var ids []string
	if err := r.db.SelectContext(ctx, &ids, query, userID); err != nil {
		return nil, fmt.Errorf("listing added catalog IDs: %w", err)
	}
	return ids, nil
}

// ExistsByCatalogID checks whether the user already owns an agent forked from
// the given catalog entry.
func (r *PostgresAgentDefinitionRepository) ExistsByCatalogID(ctx context.Context, userID, catalogID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM user_agents WHERE user_id = $1 AND source_catalog_id = $2)`
	if err := r.db.QueryRowContext(ctx, query, userID, catalogID).Scan(&exists); err != nil {
		return false, fmt.Errorf("checking catalog agent existence: %w", err)
	}
	return exists, nil
}
