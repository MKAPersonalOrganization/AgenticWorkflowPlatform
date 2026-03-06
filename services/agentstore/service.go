// Package agentstore provides CRUD operations for user-created agent definitions.
package agentstore

import (
	"context"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/foundation/logger"
)

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

	s.log.Info(ctx, "updating agent definition", "agentID", agent.ID, "userID", agent.UserID)

	if err := s.repo.Update(ctx, agent); err != nil {
		return fmt.Errorf("updating agent: %w", err)
	}

	return nil
}

// Delete removes an agent definition.
func (s *Service) Delete(ctx context.Context, id, userID string) error {
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
// Lazily ensures the initial agent exists before listing.
func (s *Service) ListByUser(ctx context.Context, userID string) ([]domain.AgentDefinition, error) {
	// Ensure at least one agent exists for this user
	if _, err := s.repo.EnsureInitialAgent(ctx, userID); err != nil {
		s.log.Error(ctx, "failed to ensure initial agent", "userID", userID, "error", err)
		// Continue anyway — list whatever exists
	}
	return s.repo.ListByUser(ctx, userID)
}

// Search finds agents matching a query string within a user's agents.
func (s *Service) Search(ctx context.Context, userID, query string) ([]domain.AgentDefinition, error) {
	return s.repo.Search(ctx, userID, query)
}

// EnsureInitialAgent guarantees at least one agent exists for the given user,
// creating a starter agent if the user has none. Returns the first agent.
func (s *Service) EnsureInitialAgent(ctx context.Context, userID string) (*domain.AgentDefinition, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}
	return s.repo.EnsureInitialAgent(ctx, userID)
}
