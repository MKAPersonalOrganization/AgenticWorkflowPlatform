// Package agentstore provides CRUD operations for user-created agent definitions.
package agentstore

import (
	"context"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
)

// ErrDeleteDefault is returned when a user attempts to delete the default agent.
var ErrDeleteDefault = fmt.Errorf("cannot delete the default agent")

// Service manages user-created agent definitions.
type Service struct {
	log  *logger.Logger
	repo domain.AgentDefinitionRepository
}

// New creates a new agent store service.
func New(log *logger.Logger, repo domain.AgentDefinitionRepository) *Service {
	return &Service{
		log:  log,
		repo: repo,
	}
}

// Create creates a new agent definition for a user.
func (s *Service) Create(ctx context.Context, agent *domain.AgentDefinition) error {
	if agent.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if agent.UserID == "" {
		return fmt.Errorf("user ID is required")
	}

	// Set sensible defaults
	if agent.MaxIterations <= 0 {
		agent.MaxIterations = 10
	}
	if agent.Temperature <= 0 {
		agent.Temperature = 0.7
	}

	s.log.Info(ctx, "creating agent definition", "userID", agent.UserID, "name", agent.Name)

	if err := s.repo.Create(ctx, agent); err != nil {
		return fmt.Errorf("creating agent: %w", err)
	}

	return nil
}

// Update modifies an existing agent definition.
func (s *Service) Update(ctx context.Context, agent *domain.AgentDefinition) error {
	if agent.ID == "" {
		return fmt.Errorf("agent ID is required")
	}
	if agent.UserID == "" {
		return fmt.Errorf("user ID is required")
	}
	if agent.Name == "" {
		return fmt.Errorf("agent name is required")
	}

	if agent.MaxIterations <= 0 {
		agent.MaxIterations = 10
	}
	if agent.Temperature <= 0 {
		agent.Temperature = 0.7
	}

	s.log.Info(ctx, "updating agent definition", "agentID", agent.ID, "userID", agent.UserID)

	if err := s.repo.Update(ctx, agent); err != nil {
		return fmt.Errorf("updating agent: %w", err)
	}

	return nil
}

// Delete removes an agent definition. Cannot delete the default agent.
func (s *Service) Delete(ctx context.Context, id, userID string) error {
	// Fetch the agent to check if it's the default
	agent, err := s.repo.GetByID(ctx, id, userID)
	if err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}
	if agent.IsDefault {
		return ErrDeleteDefault
	}

	s.log.Info(ctx, "deleting agent definition", "agentID", id, "userID", userID)

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}

	return nil
}

// GetByID retrieves a single agent by ID.
func (s *Service) GetByID(ctx context.Context, id, userID string) (*domain.AgentDefinition, error) {
	return s.repo.GetByID(ctx, id, userID)
}

// ListByUser returns all agents created by a specific user.
// Lazily ensures the default agent exists before listing.
func (s *Service) ListByUser(ctx context.Context, userID string) ([]domain.AgentDefinition, error) {
	// Ensure the default agent exists for this user
	if _, err := s.repo.EnsureDefaultAgent(ctx, userID); err != nil {
		s.log.Error(ctx, "failed to ensure default agent", "userID", userID, "error", err)
		// Continue anyway — list whatever exists
	}
	return s.repo.ListByUser(ctx, userID)
}

// Search finds agents matching a query string within a user's agents.
func (s *Service) Search(ctx context.Context, userID, query string) ([]domain.AgentDefinition, error) {
	return s.repo.Search(ctx, userID, query)
}

// EnsureDefaultAgent guarantees a default agent exists for the given user,
// creating one atomically if missing. Returns the default agent definition.
func (s *Service) EnsureDefaultAgent(ctx context.Context, userID string) (*domain.AgentDefinition, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}
	return s.repo.EnsureDefaultAgent(ctx, userID)
}
