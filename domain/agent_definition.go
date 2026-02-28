package domain

import (
	"context"
	"time"
)

// AgentDefinition represents a user-created agent definition.
// Each user can create agents that are scoped only to them.
type AgentDefinition struct {
	ID            string    `json:"id" db:"id"`
	UserID        string    `json:"user_id" db:"user_id"`
	Name          string    `json:"name" db:"name"`
	Description   string    `json:"description" db:"description"`
	SystemPrompt  string    `json:"system_prompt" db:"system_prompt"`
	ModelID       string    `json:"model_id,omitempty" db:"model_id"`
	MaxIterations int       `json:"max_iterations" db:"max_iterations"`
	Temperature   float64   `json:"temperature" db:"temperature"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time `json:"updated_at" db:"updated_at"`
}

// InitialAgentTemplate returns the template used to seed a user's first agent.
// There is no "default" concept — this is simply the starter agent.
func InitialAgentTemplate(userID string) AgentDefinition {
	return AgentDefinition{
		UserID:        userID,
		Name:          "Assistant",
		Description:   "A general-purpose AI assistant that can help with any topic.",
		SystemPrompt:  "You are a helpful, knowledgeable AI assistant. You can help with a wide range of topics including math, science, writing, coding, and more. Be clear, accurate, and supportive in your responses.",
		MaxIterations: 20,
		Temperature:   0.7,
	}
}

// AgentDefinitionRepository defines the storage contract for user-created agents.
type AgentDefinitionRepository interface {
	// Create persists a new agent definition.
	Create(ctx context.Context, agent *AgentDefinition) error

	// Update modifies an existing agent definition (must be owned by user).
	Update(ctx context.Context, agent *AgentDefinition) error

	// Delete removes an agent definition by ID (must be owned by user).
	Delete(ctx context.Context, id, userID string) error

	// GetByID retrieves a single agent by ID (must be owned by user).
	GetByID(ctx context.Context, id, userID string) (*AgentDefinition, error)

	// ListByUser returns all agents created by a specific user.
	ListByUser(ctx context.Context, userID string) ([]AgentDefinition, error)

	// Search finds agents matching a query string within a user's agents.
	Search(ctx context.Context, userID, query string) ([]AgentDefinition, error)

	// EnsureInitialAgent creates a starter agent for the user if they have
	// zero agents. Returns the first agent (by creation time). This is NOT
	// a "default" — the user can rename, edit, or delete it like any other.
	EnsureInitialAgent(ctx context.Context, userID string) (*AgentDefinition, error)
}
