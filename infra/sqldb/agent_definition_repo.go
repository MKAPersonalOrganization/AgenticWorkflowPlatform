package sqldb

import (
	"context"
	"fmt"
	"time"

	"school-with-ai/domain"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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
		INSERT INTO user_agents (id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`

	_, err := r.db.ExecContext(ctx, query,
		agent.ID,
		agent.UserID,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.ModelID,
		agent.MaxIterations,
		agent.Temperature,
		agent.IsDefault,
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
		    max_iterations = $5, temperature = $6, is_default = $7, updated_at = $8
		WHERE id = $9 AND user_id = $10`

	result, err := r.db.ExecContext(ctx, query,
		agent.Name,
		agent.Description,
		agent.SystemPrompt,
		agent.ModelID,
		agent.MaxIterations,
		agent.Temperature,
		agent.IsDefault,
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

// Delete removes an agent definition by ID. Does not delete default agents.
func (r *PostgresAgentDefinitionRepository) Delete(ctx context.Context, id, userID string) error {
	query := `DELETE FROM user_agents WHERE id = $1 AND user_id = $2 AND is_default = FALSE`

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
	query := `SELECT id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, is_default, created_at, updated_at
		FROM user_agents WHERE id = $1 AND user_id = $2`

	var agent domain.AgentDefinition
	if err := r.db.GetContext(ctx, &agent, query, id, userID); err != nil {
		return nil, fmt.Errorf("getting agent definition: %w", err)
	}

	return &agent, nil
}

// ListByUser returns all agents created by a specific user, ordered by creation time.
func (r *PostgresAgentDefinitionRepository) ListByUser(ctx context.Context, userID string) ([]domain.AgentDefinition, error) {
	query := `SELECT id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, is_default, created_at, updated_at
		FROM user_agents WHERE user_id = $1 ORDER BY created_at DESC`

	var agents []domain.AgentDefinition
	if err := r.db.SelectContext(ctx, &agents, query, userID); err != nil {
		return nil, fmt.Errorf("listing agent definitions: %w", err)
	}

	return agents, nil
}

// Search finds agents matching a query string within a user's agents.
func (r *PostgresAgentDefinitionRepository) Search(ctx context.Context, userID, query string) ([]domain.AgentDefinition, error) {
	sqlQuery := `SELECT id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, is_default, created_at, updated_at
		FROM user_agents
		WHERE user_id = $1 AND (name ILIKE $2 OR description ILIKE $2)
		ORDER BY created_at DESC`

	pattern := "%" + query + "%"

	var agents []domain.AgentDefinition
	if err := r.db.SelectContext(ctx, &agents, sqlQuery, userID, pattern); err != nil {
		return nil, fmt.Errorf("searching agent definitions: %w", err)
	}

	return agents, nil
}

// EnsureDefaultAgent atomically creates a default agent for the user if one
// does not already exist. Returns the default agent definition.
func (r *PostgresAgentDefinitionRepository) EnsureDefaultAgent(ctx context.Context, userID string) (*domain.AgentDefinition, error) {
	defaults := domain.DefaultAgentDefinition(userID)
	id := uuid.New().String()
	now := time.Now().UTC()

	// INSERT … ON CONFLICT DO NOTHING uses the unique partial index on
	// (user_id) WHERE is_default = TRUE — so only one default per user.
	query := `
		INSERT INTO user_agents (id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, is_default, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, TRUE, $9, $10)
		ON CONFLICT (user_id) WHERE is_default = TRUE DO NOTHING`

	_, err := r.db.ExecContext(ctx, query,
		id, userID,
		defaults.Name, defaults.Description, defaults.SystemPrompt,
		defaults.ModelID, defaults.MaxIterations, defaults.Temperature,
		now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("ensuring default agent: %w", err)
	}

	// Always fetch the actual row (may be the one we just inserted or a pre-existing one).
	return r.GetDefaultAgent(ctx, userID)
}

// GetDefaultAgent returns the default agent for a user, or nil if none exists.
func (r *PostgresAgentDefinitionRepository) GetDefaultAgent(ctx context.Context, userID string) (*domain.AgentDefinition, error) {
	query := `SELECT id, user_id, name, description, system_prompt, model_id, max_iterations, temperature, is_default, created_at, updated_at
		FROM user_agents WHERE user_id = $1 AND is_default = TRUE`

	var agent domain.AgentDefinition
	if err := r.db.GetContext(ctx, &agent, query, userID); err != nil {
		return nil, fmt.Errorf("getting default agent: %w", err)
	}
	return &agent, nil
}
