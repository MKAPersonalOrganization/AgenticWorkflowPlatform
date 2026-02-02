package agentorchestrator

import (
	"context"
	"fmt"
	"iter"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/artifact"
	"school-with-ai/services/threadmgmt"
)

// EventEmitter is the interface for broadcasting events.
type EventEmitter interface {
	Dispatch(ctx context.Context, event domain.Event) error
}

// Service handles orchestration of agent events, providing persistence and broadcasting.
type Service struct {
	log           *logger.Logger
	writeModel    domain.EventStore
	threadRepo    *domain.ThreadRepository
	emitter       EventEmitter
	artifactStore artifact.Service
	profileRepo   domain.StudentProfileRepository
	runner        agent.Runner // Framework runner interface (foundation layer)

	// Internal components
	threadMgmt *threadmgmt.Manager
}

// Option is a functional option for configuring Service.
type Option func(*Service)

// New creates a new framework-agnostic agent orchestration service.
func New(
	log *logger.Logger,
	eventLog domain.EventStore,
	opts ...Option,
) *Service {
	s := &Service{
		log:        log,
		writeModel: eventLog,
		threadRepo: domain.NewThreadRepository(threadmgmt.NewEventStoreAdapter(eventLog)),
		threadMgmt: threadmgmt.New(log, domain.NewThreadRepository(threadmgmt.NewEventStoreAdapter(eventLog))),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

// WithEmitter configures the agent with an event emitter for broadcasting.
func WithEmitter(emitter EventEmitter) Option {
	return func(s *Service) {
		s.emitter = emitter
	}
}

// WithArtifactService configures the agent with an artifact service.
func WithArtifactService(store artifact.Service) Option {
	return func(s *Service) {
		s.artifactStore = store
	}
}

// WithStudentProfileRepository configures the agent with a student profile repository.
func WithStudentProfileRepository(repo domain.StudentProfileRepository) Option {
	return func(s *Service) {
		s.profileRepo = repo
	}
}

// WithRunner configures the service with an agent runner.
func WithRunner(runner agent.Runner) Option {
	return func(s *Service) {
		s.runner = runner
	}
}

// SetRunner sets the agent runner after construction.
// This allows breaking circular dependencies during initialization.
func (s *Service) SetRunner(runner agent.Runner) {
	s.runner = runner
}

// convertAgentEventToDomain converts foundation agent events to domain events.
// Returns nil for events that should not be converted (e.g., framework-specific events).
func convertAgentEventToDomain(ae agent.Event) domain.Event {
	// The agent events are already domain events wrapped in the agent.Event interface
	// We just need to extract the underlying domain event
	if domainEvt, ok := ae.(domain.Event); ok {
		return domainEvt
	}
	return nil
}

// ProcessEvents orchestrates a stream of domain events, providing persistence and broadcasting.
func (s *Service) ProcessEvents(ctx context.Context, events iter.Seq2[domain.Event, error]) iter.Seq2[domain.Event, error] {
	return func(yield func(domain.Event, error) bool) {
		eventCount := 0
		durableCount := 0
		transientCount := 0

		for event, err := range events {
			eventCount++

			if err != nil {
				s.log.Error(ctx, "Orchestrator: Event stream error", "eventNumber", eventCount, "error", err)
				yield(nil, err)
				return
			}

			// Handle persistence
			durable := s.isDurable(event)
			s.log.Debug(ctx, "Orchestrator: Processing event", "eventNumber", eventCount, "eventType", event.EventType(), "isDurable", durable)

			if durable {
				durableCount++
				s.log.Debug(ctx, "Orchestrator: Persisting durable event", "eventType", event.EventType())
				if pErr := s.persistDurableEvent(ctx, event); pErr != nil {
					s.log.Error(ctx, "Orchestrator: Failed to persist durable event", "eventType", event.EventType(), "error", pErr)
					yield(event, fmt.Errorf("persisting event: %w", pErr))
					return
				}
				s.log.Debug(ctx, "Orchestrator: Successfully persisted durable event", "eventType", event.EventType())
			} else {
				transientCount++
				s.log.Debug(ctx, "Orchestrator: Processing transient event", "eventType", event.EventType())
			}

			// Handle broadcasting
			if s.emitter != nil {
				// We only broadcast TRANSIENT events from here.
				// DURABLE events are broadcast by the projection updater to ensure consistency
				// and prevent double-broadcasting to the UI.
				if !durable {
					if bErr := s.emitter.Dispatch(ctx, event); bErr != nil {
						s.log.Error(ctx, "Orchestrator: Failed to dispatch transient event", "eventType", event.EventType(), "error", bErr)
					} else {
						s.log.Debug(ctx, "Orchestrator: Successfully dispatched transient event", "eventType", event.EventType())
					}
				}
			}

			if !yield(event, nil) {
				s.log.Info(ctx, "Orchestrator: Event stream terminated early", "totalEvents", eventCount, "durable", durableCount, "transient", transientCount)
				return
			}
		}

		s.log.Info(ctx, "Orchestrator: Event stream completed", "totalEvents", eventCount, "durable", durableCount, "transient", transientCount)
	}
}

// isDurable returns true if the event should be persisted to the event store.
// Per EVENT_CONTRACTS.md: only business facts are persisted, not framework lifecycle events.
func (s *Service) isDurable(event domain.Event) bool {
	switch event.(type) {
	// Application Service Events (Persisted)
	case *domain.ThreadCreated,
		*domain.ThreadDeleted,
		*domain.ThreadTitleUpdated,
		*domain.UserMessageAdded:
		return true

	// Agent Framework Events - Persisted
	case *domain.AssistantMessageAdded,
		*domain.ToolResultAdded,
		*domain.TokenUsageReported,
		*domain.ToolNeedsInput,
		*domain.ToolNeedsAuth,
		*domain.InputProvided,
		*domain.AuthGranted,
		*domain.AuthDenied,
		*domain.ToolCancelled:
		return true

	// Agent Framework Events - Transient (NOT persisted)
	// Run lifecycle events are for frontend UI only, not thread state
	case *domain.RunStarted,
		*domain.RunFinished,
		*domain.RunError,
		*domain.AssistantMessageStarted,
		*domain.AssistantContentStreamed,
		*domain.AssistantMessageCompleted,
		*domain.ToolStart:
		return false

	default:
		return false
	}
}

// persistDurableEvent persists a domain event based on its type.
func (s *Service) persistDurableEvent(ctx context.Context, event domain.Event) error {
	switch e := event.(type) {
	case *domain.ThreadCreated:
		_, _, err := s.threadMgmt.CreateThread(ctx, e.ThreadID, e.UserID, e.Title)
		return err
	case *domain.ThreadDeleted:
		_, err := s.threadMgmt.DeleteThread(ctx, e.ThreadID)
		return err
	case *domain.UserMessageAdded:
		_, _, err := s.threadMgmt.AddUserMessage(ctx, e.ThreadID, e.MessageID, e.Content, e.UserID)
		return err
	case *domain.AssistantMessageAdded:
		_, _, err := s.threadMgmt.AddAssistantMessage(ctx, e.ThreadID, e.MessageID, e.Content, e.ToolCalls, e.AgentName)
		return err
	case *domain.ToolResultAdded:
		_, _, err := s.threadMgmt.AddToolResultMessage(ctx, e.ThreadID, e.MessageID, e.ToolCallID, e.ToolName, e.Result, e.AgentName)
		return err
	case *domain.ThreadTitleUpdated:
		_, err := s.threadMgmt.UpdateThreadTitle(ctx, e.ThreadID, e.Title)
		return err
	case *domain.TokenUsageReported:
		_, err := s.threadMgmt.AddTokenUsage(ctx, *e)
		return err
	case *domain.ToolNeedsInput,
		*domain.ToolNeedsAuth,
		*domain.InputProvided,
		*domain.AuthGranted,
		*domain.AuthDenied,
		*domain.ToolCancelled:
		_, err := s.threadMgmt.RecordToolEvent(ctx, event.AggregateID(), event)
		return err
	default:
		return nil
	}
}

// GetAvailableAgents returns a list of available agents
func (s *Service) GetAvailableAgents() []string {
	// Note: In framework-agnostic version, this should probably come from a central registry.
	// For now we return empty or implement a registry injection.
	return nil
}

// GetThreadManagement returns the thread management component.
func (s *Service) GetThreadManagement() *threadmgmt.Manager {
	return s.threadMgmt
}

// RunExecutionRequest contains parameters for executing a run.
type RunExecutionRequest struct {
	SessionID   string // ID of the conversation thread
	UserID      string // User making the request
	AgentName   string // Agent to execute
	Message     string // User message content
	MessageID   string // Message ID (used as RunID)
	ParentRunID string // (Optional) Lineage pointer for branching/time travel
}

// ExecuteRun orchestrates a framework-agnostic run execution with lifecycle events.
// This method emits RunStarted, calls the runner, and emits RunFinished or RunError.
// It provides a consistent run lifecycle regardless of which framework adapter is used.
func (s *Service) ExecuteRun(
	ctx context.Context,
	req RunExecutionRequest,
) iter.Seq2[domain.Event, error] {
	return func(yield func(domain.Event, error) bool) {
		// 1. Emit RunStarted
		runStartedEvent := &domain.RunStarted{
			ThreadID:    req.SessionID,
			RunID:       req.MessageID,
			ParentRunID: req.ParentRunID,
			Input:       req.Message,
			Timestamp:   time.Now(),
		}

		runStartedIter := func(y func(domain.Event, error) bool) {
			y(runStartedEvent, nil)
		}
		processedStart := s.ProcessEvents(ctx, runStartedIter)
		for evt, err := range processedStart {
			if err != nil {
				yield(nil, fmt.Errorf("failed to emit RunStarted: %w", err))
				return
			}
			if !yield(evt, nil) {
				return
			}
		}

		// 2. Call the framework runner to get events
		runnerReq := &agent.RunRequest{
			SessionID: req.SessionID,
			UserID:    req.UserID,
			AgentName: req.AgentName,
			Message: &agent.Message{
				ID: req.MessageID,
				Content: []agent.Content{
					{Type: agent.ContentTypeText, Text: req.Message},
				},
			},
		}
		agentEvents := s.runner.Run(ctx, runnerReq)

		// 3. Process agent events, convert to domain events, and check for errors
		var executionError error
		for ae, err := range agentEvents {
			if err != nil {
				executionError = err
				break
			}

			// Convert agent.Event to domain.Event
			de := convertAgentEventToDomain(ae)
			if de != nil {
				if !yield(de, nil) {
					return
				}
			}
		}

		// 4. Emit RunError if execution failed, otherwise emit RunFinished
		if executionError != nil {
			runErrorEvent := &domain.RunError{
				ThreadID:  req.SessionID,
				RunID:     req.MessageID,
				Message:   executionError.Error(),
				Code:      "RUN_ERROR",
				Timestamp: time.Now(),
			}

			runErrorIter := func(y func(domain.Event, error) bool) {
				y(runErrorEvent, nil)
			}
			processedError := s.ProcessEvents(ctx, runErrorIter)
			for evt, err := range processedError {
				if err != nil {
					yield(nil, fmt.Errorf("failed to emit RunError: %w", err))
					return
				}
				if !yield(evt, nil) {
					return
				}
			}

			// Yield the original execution error
			yield(nil, executionError)
			return
		}

		// 5. Emit RunFinished on success
		runFinishedEvent := &domain.RunFinished{
			ThreadID:  req.SessionID,
			RunID:     req.MessageID,
			Result:    nil, // TODO: Capture result from framework events if available
			Timestamp: time.Now(),
		}

		runFinishedIter := func(y func(domain.Event, error) bool) {
			y(runFinishedEvent, nil)
		}
		processedFinish := s.ProcessEvents(ctx, runFinishedIter)
		for evt, err := range processedFinish {
			if err != nil {
				yield(nil, fmt.Errorf("failed to emit RunFinished: %w", err))
				return
			}
			if !yield(evt, nil) {
				return
			}
		}
	}
}
