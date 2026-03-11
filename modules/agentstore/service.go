package agentstore

import (
	"context"
	"fmt"

	"school-with-ai/foundation/logger"
)

// service manages user-created agent definitions.
type service struct {
	log  *logger.Logger
	repo AgentDefinitionRepository
}

// newService creates a new agent store service.
func newService(log *logger.Logger, repo AgentDefinitionRepository) *service {
	return &service{
		log:  log,
		repo: repo,
	}
}

// create creates a new agent definition for a user.
func (s *service) create(ctx context.Context, agent *AgentDefinition) error {
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

// update modifies an existing agent definition.
func (s *service) update(ctx context.Context, agent *AgentDefinition) error {
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

// delete removes an agent definition.
func (s *service) delete(ctx context.Context, id, userID string) error {
	s.log.Info(ctx, "deleting agent definition", "agentID", id, "userID", userID)

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return fmt.Errorf("deleting agent: %w", err)
	}

	return nil
}

// getByID retrieves a single agent by ID.
func (s *service) getByID(ctx context.Context, id, userID string) (*AgentDefinition, error) {
	return s.repo.GetByID(ctx, id, userID)
}

// listByUser returns all agents created by a specific user.
func (s *service) listByUser(ctx context.Context, userID string) ([]AgentDefinition, error) {
	return s.repo.ListByUser(ctx, userID)
}

// search finds agents matching a query string within a user's agents.
func (s *service) search(ctx context.Context, userID, query string) ([]AgentDefinition, error) {
	return s.repo.Search(ctx, userID, query)
}
