package threadservice

import (
	"context"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/threadmgmt"
)

// Service implements the services.ThreadCommandService interface.
type Service struct {
	log        *logger.Logger
	threadMgmt *threadmgmt.Manager
	config     core.AgentConfig
}

// New creates a new ThreadService implementation.
func New(log *logger.Logger, threadRepo *domain.ThreadRepository, config core.AgentConfig) *Service {
	return &Service{
		log:        log,
		threadMgmt: threadmgmt.New(log, threadRepo, config.Name),
		config:     config,
	}
}

// CreateThread creates a new chat thread using event sourcing.
func (s *Service) CreateThread(ctx context.Context, threadID, title string) (core.AgentThread, error) {
	agg, event, err := s.threadMgmt.CreateThread(ctx, threadID, title)
	if err != nil {
		return nil, err
	}
	s.notify(ctx, event)
	return s.threadMgmt.ToAgentThread(agg), nil
}

// AddUserMessage adds a user message to a thread using event sourcing.
func (s *Service) AddUserMessage(ctx context.Context, threadID, messageID, content string) (core.AgentThread, error) {
	agg, event, err := s.threadMgmt.AddUserMessage(ctx, threadID, messageID, content)
	if err != nil {
		return nil, err
	}
	s.notify(ctx, event)
	return s.threadMgmt.ToAgentThread(agg), nil
}

// UpdateThreadTitle updates the title of a thread using event sourcing.
func (s *Service) UpdateThreadTitle(ctx context.Context, threadID, newTitle string) (*domain.ThreadTitleUpdatedEvent, error) {
	return s.threadMgmt.UpdateThreadTitle(ctx, threadID, newTitle)
}

// DeleteThread deletes a thread using event sourcing.
func (s *Service) DeleteThread(ctx context.Context, threadID string) error {
	return s.threadMgmt.DeleteThread(ctx, threadID)
}

// notify emits a domain event if a notifier is configured.
func (s *Service) notify(ctx context.Context, event core.Event) {
	if s.config.Notifier != nil && event != nil {
		if err := s.config.Notifier.HandleDomainEvent(event); err != nil {
			s.log.Error(ctx, "failed to notify event", "error", err, "eventType", event.EventType())
		}
	}
}
