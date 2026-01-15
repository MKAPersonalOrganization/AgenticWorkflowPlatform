package agentorchestrator

import (
	"context"
	"iter"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator/engine"
	"school-with-ai/services/agentorchestrator/rag"
	"school-with-ai/services/agentorchestrator/threadmgmt"
	"school-with-ai/services/agentorchestrator/types"
)

// Use central types
type ChatClient = types.ChatClient

// Service handles processing of user messages using pure event sourcing.
// It acts as an orchestrator, delegating specific responsibilities to internal components.
type Service struct {
	log           *logger.Logger
	config        core.AgentConfig
	chatClient    ChatClient
	writeModel    core.EventStore
	threadRepo    *domain.ThreadRepository
	toolRegistry  *core.ToolRegistry
	reqCtxCapture **core.RequestContext
	ragService    rag.RAG
	toolTracer    core.ToolTracer

	// Internal components
	threadMgmt *threadmgmt.Manager
	engine     *engine.Engine
}

// Option is a functional option for configuring Service.
type Option func(*Service)

// WithConfig configures the agent with a complete configuration.
func WithConfig(config core.AgentConfig) Option {
	return func(s *Service) {
		s.config = config
	}
}

// WithRequestContextCapture configures the agent to capture the request context for handoff detection.
func WithRequestContextCapture(capture **core.RequestContext) Option {
	return func(s *Service) {
		s.reqCtxCapture = capture
	}
}

// WithRAG configures the agent with a RAG service for context retrieval.
func WithRAG(ragService rag.RAG) Option {
	return func(s *Service) {
		s.ragService = ragService
	}
}

// WithToolTracer configures the agent with a tool tracer for observability.
func WithToolTracer(tracer core.ToolTracer) Option {
	return func(s *Service) {
		s.toolTracer = tracer
	}
}

// New creates a new agent orchestrator service.
func New(log *logger.Logger, chatClient ChatClient, writeModel core.EventStore, opts ...Option) *Service {
	s := &Service{
		log:          log,
		chatClient:   chatClient,
		writeModel:   writeModel,
		threadRepo:   domain.NewThreadRepository(NewEventStoreAdapter(writeModel)),
		config:       core.DefaultConfig("agent"),
		toolRegistry: core.NewToolRegistry(),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// Register tools
	for _, tool := range s.config.Tools {
		s.toolRegistry.Register(tool)
	}

	// Initialize components
	s.threadMgmt = threadmgmt.New(log, s.threadRepo, s.config.Name)
	s.engine = engine.New(
		log,
		chatClient,
		s.threadMgmt,
		s.config,
		s.reqCtxCapture,
		s.toolTracer,
	)

	return s
}

// GetNewThread creates a new conversation thread.
func (s *Service) GetNewThread() core.AgentThread {
	return core.NewInMemoryThread()
}

// DeserializeThread restores an agent thread from state.
func (s *Service) DeserializeThread(data []byte) (core.AgentThread, error) {
	return core.NewInMemoryThreadFromState(data)
}

// Run processes the LLM response using an AgentThread and yields events.
func (s *Service) Run(ctx context.Context, runCtx core.RunContext) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		for event, err := range s.engine.Run(ctx, runCtx) {
			if s.config.Notifier != nil && event != nil {
				// We ignore the error here to keep the stream moving, but log it
				if nErr := s.config.Notifier.HandleDomainEvent(event); nErr != nil {
					s.log.Error(ctx, "failed to notify event", "error", nErr, "eventType", event.EventType())
				}
			}
			if !yield(event, err) {
				return
			}
		}
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

// notify emits a domain event if a notifier is configured.
func (s *Service) notify(ctx context.Context, event core.Event) {
	if s.config.Notifier != nil && event != nil {
		if err := s.config.Notifier.HandleDomainEvent(event); err != nil {
			s.log.Error(ctx, "failed to notify event", "error", err, "eventType", event.EventType())
		}
	}
}

// UpdateThreadTitle updates the title of a thread using event sourcing.
func (s *Service) UpdateThreadTitle(ctx context.Context, threadID, newTitle string) (*domain.ThreadTitleUpdatedEvent, error) {
	return s.threadMgmt.UpdateThreadTitle(ctx, threadID, newTitle)
}

// DeleteThread deletes a thread using event sourcing.
func (s *Service) DeleteThread(ctx context.Context, threadID string) error {
	return s.threadMgmt.DeleteThread(ctx, threadID)
}
