package agentorchestrator

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/engine"
	"school-with-ai/foundation/agentsdk/llm"
	"school-with-ai/foundation/agentsdk/registry"
	"school-with-ai/foundation/agentsdk/tools"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/artifact"
	"school-with-ai/services/threadmgmt"
)

// ChatClient is the LLM client interface.
type ChatClient = llm.ChatClient

// EventEmitter is the interface for broadcasting events.
type EventEmitter interface {
	Dispatch(ctx context.Context, event domain.Event) error
}

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
	ragService    domain.RAG
	toolTracer    core.ToolTracer
	emitter       EventEmitter

	artifactStore artifact.Service
	enableMemory  bool
	profileRepo   domain.StudentProfileRepository

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
		registry:      registry.GlobalAgents,
		artifactStore: s.artifactStore,
		profileRepo:   s.profileRepo,
	}

	s.engine = engine.New(
		log,
		s.chatClient,
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
func WithRAG(ragService domain.RAG) Option {
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

// WithMemoryEnabled configures whether the memory tool should be enabled.
func WithMemoryEnabled(enabled bool) Option {
	return func(s *Service) {
		s.enableMemory = enabled
	}
}

// WithStudentProfileRepository configures the agent with a student profile repository.
func WithStudentProfileRepository(repo domain.StudentProfileRepository) Option {
	return func(s *Service) {
		s.profileRepo = repo
	}
}

// RegistryAgentProvider implements engine.AgentProvider using GlobalAgents.
type RegistryAgentProvider struct {
	initialConfig core.AgentConfig
	registry      *registry.AgentRegistry
	artifactStore artifact.Service
	enableMemory  bool
	profileRepo   domain.StudentProfileRepository
}

func (p *RegistryAgentProvider) GetConfig(ctx context.Context, name string) (core.AgentConfig, error) {
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
	config := factory(ctx, nil)

	// Inject Memory Tool and Context if available and enabled
	if p.artifactStore != nil && p.enableMemory {
		memoryTool := tools.NewMemoryTool(p.artifactStore)
		config.Tools = append(config.Tools, memoryTool.Definition()...)

		// Load long-term memory for context injection
		// We use a middleware to inject memory dynamically based on the user context
		memoryMiddleware := tools.NewMemoryInjectorMiddleware(memoryTool)
		config.Middleware = append(config.Middleware, memoryMiddleware)
	}

	// Inject Student Profile Middleware
	if p.profileRepo != nil {
		profileMiddleware := tools.NewStudentProfileInjectorMiddleware(p.profileRepo)
		config.Middleware = append(config.Middleware, profileMiddleware)
	}

	return config, nil
}

// Run processes the LLM response using an AgentThread and yields events.
func (s *Service) Run(ctx context.Context, agentName string, runCtx core.RunContext) iter.Seq2[core.Event, error] {
	return func(yield func(core.Event, error) bool) {
		for event, err := range s.engine.Run(ctx, agentName, runCtx) {
			if err != nil {
				yield(nil, err)
				return
			}

			// Handle persistence
			durable := s.isDurable(event)
			if durable {
				if pErr := s.persistDurableEvent(ctx, event); pErr != nil {
					yield(event, fmt.Errorf("persisting event: %w", pErr))
					return
				}
			}

			// Handle broadcasting
			if s.emitter != nil {
				// We only broadcast TRANSIENT events from here.
				// DURABLE events are broadcast by the projection updater to ensure consistency
				// and prevent double-broadcasting to the UI.
				if !durable {
					if de, ok := event.(domain.Event); ok {
						if bErr := s.emitter.Dispatch(ctx, de); bErr != nil {
							s.log.Error(ctx, "failed to dispatch event", "error", bErr)
						}
					}
				}
			}

			if !yield(event, nil) {
				return
			}
		}
	}
}

// isDurable returns true if the event should be persisted to the durable store.
func (s *Service) isDurable(event core.Event) bool {
	if event == nil {
		return false
	}

	switch event.(type) {
	case *domain.UserMessageAdded,
		*domain.AssistantMessageAdded,
		*domain.ToolResultAdded,
		*domain.TokenUsageReported,
		domain.EngineStateChanged,
		*domain.ExecutionStateReported:
		return true
	default:
		return false
	}
}

// persistDurableEvent handles persistence for events that must be durable.
func (s *Service) persistDurableEvent(ctx context.Context, event core.Event) error {
	if event == nil {
		return nil
	}

	switch e := event.(type) {
	case *domain.UserMessageAdded:
		_, _, err := s.threadMgmt.AddUserMessage(ctx, e.ThreadID, e.MessageID, e.Content, e.UserID)
		if err != nil {
			return err
		}
		// Handle title generation for first message
		agg, err := s.threadMgmt.Load(ctx, e.ThreadID)
		if err == nil && len(agg.Messages) == 1 {
			title := engine.GenerateThreadTitle(e.Content)
			_, _ = s.threadMgmt.UpdateThreadTitle(ctx, e.ThreadID, title)
		}
	case *domain.AssistantMessageAdded:
		_, _, err := s.threadMgmt.AddAssistantMessage(ctx, e.ThreadID, e.MessageID, e.Content, e.ToolCalls, e.AgentName)
		return err
	case *domain.ToolResultAdded:
		_, _, err := s.threadMgmt.AddToolResultMessage(ctx, e.ThreadID, e.MessageID, e.ToolCallID, e.ToolName, e.Result, e.AgentName)
		return err
	case *domain.TokenUsageReported:
		_, err := s.threadMgmt.AddTokenUsage(ctx, *e)
		return err
	case domain.EngineStateChanged:
		_, err := s.threadMgmt.RecordEngineStateChange(ctx, e)
		return err
	case *domain.ExecutionStateReported:
		_, err := s.threadMgmt.RecordExecutionState(ctx, *e)
		return err
	}

	return nil
}

// ProcessUserMessage handles a new user message submission.
func (s *Service) ProcessUserMessage(ctx context.Context, agentName, threadID, content, userID string) error {
	runID := domain.GenerateMessageID()

	// 1. Thread Management (History Loading)
	agg, err := s.threadMgmt.Load(ctx, threadID)
	if err != nil && !errors.Is(err, domain.ErrThreadNotFound) {
		return fmt.Errorf("failed to load thread: %w", err)
	}

	var thread core.AgentThread
	var messages []core.Message

	if agg != nil {
		thread = s.threadMgmt.ToAgentThread(agg)
		messages, err = thread.RetrieveFullConversationHistory(ctx)
		if err != nil {
			return fmt.Errorf("failed to retrieve conversation history: %w", err)
		}
	} else {
		// New thread - empty history
		thread = core.NewInMemoryThreadWithID(threadID)
		messages = []core.Message{}
	}

	// 2. Prepare Run Context
	runCtx := core.RunContext{
		ThreadID:       threadID,
		MessageID:      runID,
		Thread:         thread,
		InitialHistory: messages,
		Options:        core.DefaultRunOptions(),
		UserContent:    content,
		UserID:         userID,
	}

	// Restore execution state if resuming
	if agg != nil && agg.ExecutionState != nil {
		runCtx.Iteration = agg.ExecutionState.Iteration
		runCtx.Total = agg.ExecutionState.Total
		runCtx.LoopState = agg.ExecutionState.LoopStateHex
	}

	// 3. Delegation to Engine
	runIter := s.Run(ctx, agentName, runCtx)

	// 4. Consumption and Error Handling
	for _, err := range runIter {
		if err != nil {
			return err
		}
	}

	return nil
}

// GetAvailableAgents returns a list of available agents
func (s *Service) GetAvailableAgents() []string {
	return []string{
		"math-instructor",
		"physics-instructor",
		"chemistry-instructor",
		"statistics-instructor",
		"diagramming-instructor",
	}
}

// GetThreadManager returns the underlying thread manager.
func (s *Service) GetThreadManager() *threadmgmt.Manager {
	return s.threadMgmt
}

// GetEngineGraph returns the engine state machine graph in DOT format.
func (s *Service) GetEngineGraph() string {
	return s.engine.GetGraph()
}

// GetToolGraph returns the tool state machine graph in DOT format.
func (s *Service) GetToolGraph() string {
	return domain.GetToolStateMachineGraph()
}
