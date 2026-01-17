package agentorchestrator

import (
	"context"
	"fmt"
	"iter"
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/agentorchestrator/agents/shared"
	"school-with-ai/services/agentorchestrator/engine"
	"school-with-ai/services/agentorchestrator/rag"
	"school-with-ai/services/agentorchestrator/types"
	"school-with-ai/services/threadmgmt"
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

// New creates a new agent orchestration service.
func New(
	log *logger.Logger,
	chatClient ChatClient,
	eventLog core.EventStore,
	opts ...Option,
) *Service {
	s := &Service{
		log:          log,
		chatClient:   chatClient,
		writeModel:   eventLog,
		toolRegistry: core.NewToolRegistry(),
		threadRepo:   domain.NewThreadRepository(threadmgmt.NewEventStoreAdapter(eventLog)),
		threadMgmt:   threadmgmt.New(log, domain.NewThreadRepository(threadmgmt.NewEventStoreAdapter(eventLog))),
	}

	for _, opt := range opts {
		opt(s)
	}

	// Initialize Provider and Engine
	provider := &RegistryAgentProvider{
		initialConfig: s.config,
		registry:      shared.GlobalAgents,
	}

	s.engine = engine.New(
		log,
		s.chatClient,
		s.threadMgmt,
		provider,
		s.reqCtxCapture,
		s.toolTracer,
	)

	return s
}

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

// RegistryAgentProvider implements engine.AgentProvider using GlobalAgents.
type RegistryAgentProvider struct {
	initialConfig core.AgentConfig
	registry      *shared.AgentRegistry
}

func (p *RegistryAgentProvider) GetConfig(name string) (core.AgentConfig, error) {
	if name == p.initialConfig.Name {
		return p.initialConfig, nil
	}

	// Use provided registry
	factory, ok := p.registry.Get(name)
	if !ok {
		return core.AgentConfig{}, fmt.Errorf("agent %s not found", name)
	}

	// Create new agent config using factory
	// Note: We pass nil for notifier as events are yielded by the engine directly
	return factory(nil), nil
}

// Run processes the LLM response using an AgentThread and yields events.
func (s *Service) Run(ctx context.Context, agentName string, runCtx core.RunContext) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		for event, err := range s.engine.Run(ctx, agentName, runCtx) {
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

// ProcessUserMessage handles the full flow of processing a user message:
// 1. Adds the message to the thread.
// 2. Generates/Updates the thread title (if new).
// 3. Runs the agent execution loop.
func (s *Service) ProcessUserMessage(ctx context.Context, threadID, content, userID string) (iter.Seq2[core.Event, error], error) {
	// Delegate to Engine
	return s.engine.ProcessUserMessage(ctx, "student-facing-agent", threadID, content, userID)
}
