package custom

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/agentsdk/engine"
	"school-with-ai/foundation/agentsdk/internal/llm"
	"school-with-ai/foundation/logger"
	"school-with-ai/services/artifact"
	memorytools "school-with-ai/services/memory/tools"
	smiddleware "school-with-ai/services/studentprofile/middleware"
)

// RunnerAdapter adapts the custom engine to the agent.Runner interface.
type RunnerAdapter struct {
	log           *logger.Logger
	threadMgr     ThreadManager // Interface for thread management
	engine        *engine.Engine
	agentProvider agent.AgentProvider
	artifactStore artifact.Service
	profileRepo   domain.StudentProfileRepository
	enableMemory  bool
	pipeline      *EventPipeline // Add event pipeline for observability
}

// NewRunnerAdapter creates a new runner adapter.
// It creates its own LLM client based on the ModelConfig.
// NewRunnerAdapter creates a new runner adapter.
// It creates its own LLM client based on the ModelConfig.
func NewRunnerAdapter(
	ctx context.Context,
	log *logger.Logger,
	threadMgr ThreadManager,
	agentProvider agent.AgentProvider, // Injected provider
	modelCfg *agent.ModelConfig,
	artifactStore artifact.Service,
	profileRepo domain.StudentProfileRepository,
	enableMemory bool,
) (*RunnerAdapter, error) {
	if modelCfg == nil {
		return nil, fmt.Errorf("model config is required")
	}

	// Create LLM client based on provider - this is now internal to the adapter
	chatClient, err := createLLMClient(ctx, modelCfg)
	if err != nil {
		return nil, fmt.Errorf("creating LLM client: %w", err)
	}

	r := &RunnerAdapter{
		log:           log,
		threadMgr:     threadMgr,
		agentProvider: agentProvider,
		artifactStore: artifactStore,
		profileRepo:   profileRepo,
		enableMemory:  enableMemory,
		pipeline:      NewEventPipeline(log), // Initialize event pipeline
	}

	provider := &RegistryAgentProvider{
		provider:      agentProvider,
		artifactStore: r.artifactStore,
		profileRepo:   r.profileRepo,
		enableMemory:  r.enableMemory,
		modelProvider: modelCfg.Provider,
		modelID:       modelCfg.Model,
	}

	r.engine = engine.New(
		chatClient,
		provider,
		nil, // reqCtxCapture
		nil, // toolTracer
	)

	return r, nil
}

// createLLMClient creates an LLM client based on the model configuration.
// This is an internal implementation detail of the custom adapter.
func createLLMClient(ctx context.Context, cfg *agent.ModelConfig) (llm.LLM, error) {
	switch cfg.Provider {
	case "openai":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("OpenAI API key is required")
		}
		return llm.NewOpenAIClientWithAPIKey(cfg.APIKey, cfg.Model), nil

	case "gemini":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("Gemini API key is required")
		}
		client, err := llm.NewGeminiClient(ctx, cfg.APIKey, cfg.Model)
		if err != nil {
			return nil, fmt.Errorf("initializing Gemini client: %w", err)
		}
		return client, nil

	default:
		return nil, fmt.Errorf("unsupported AI provider: %s", cfg.Provider)
	}
}

// Run executes an agent with the given request.
func (r *RunnerAdapter) Run(ctx context.Context, req *agent.RunRequest) iter.Seq2[domain.Event, error] {
	return func(yield func(domain.Event, error) bool) {
		// 1. Facts First: Initialize Thread and Message
		threadMgmt := r.threadMgr.GetThreadManagement()
		var agg *domain.ThreadAggregate
		var initialEvents []domain.Event

		userContent := extractUserPrompt(req.Message)
		if req.Message != nil {
			messageID := req.Message.ID
			if messageID == "" {
				messageID = domain.GenerateMessageID()
			}

			// AddUserMessage handles both thread creation and message persistence
			var uEvent *domain.UserMessageAdded
			var pErr error
			agg, uEvent, pErr = threadMgmt.AddUserMessage(ctx, req.SessionID, messageID, userContent, req.UserID)
			if pErr != nil {
				yield(nil, fmt.Errorf("failed to persist user message: %w", pErr))
				return
			}
			if uEvent != nil {
				initialEvents = append(initialEvents, uEvent)
			}
		} else {
			// Resuming without a new message
			var lErr error
			agg, lErr = threadMgmt.Load(ctx, req.SessionID)
			if lErr != nil && !errors.Is(lErr, domain.ErrThreadNotFound) {
				yield(nil, fmt.Errorf("failed to load thread: %w", lErr))
				return
			}
		}

		// 2. Map Domain Aggregate to SDK Thread (Naturally includes the new message)
		var thread core.AgentThread
		var messages []core.Message

		if agg != nil {
			thread = r.toAgentThread(agg)
			var hErr error
			messages, hErr = thread.RetrieveFullConversationHistory(ctx)
			if hErr != nil {
				yield(nil, fmt.Errorf("failed to retrieve history: %w", hErr))
				return
			}
		} else {
			thread = core.NewInMemoryThreadWithID(req.SessionID)
			messages = []core.Message{}
		}

		// 3. Prepare Run Context
		runCtx := core.RunContext{
			ThreadID:       req.SessionID,
			UserID:         req.UserID,
			UserContent:    userContent,
			Thread:         thread,
			InitialHistory: messages,
			MessageID:      domain.GenerateMessageID(), // Always generate a fresh ID for the agent's first response
		}

		// 4. Resolve Agent Name
		agentName := req.AgentName
		if agentName == "" {
			// Try to infer from history
			if agg != nil {
				for i := len(agg.Messages) - 1; i >= 0; i-- {
					if agg.Messages[i].AgentName != "" {
						agentName = agg.Messages[i].AgentName
						break
					}
				}
			}
		}
		// Fallback to default if still empty
		if agentName == "" {
			agentName = "math-instructor"
		}

		// 5. Run engine and process events
		coreEvents := r.engine.Run(ctx, agentName, runCtx)

		// Convert core events to domain events
		engineDomainEvents := r.pipeline.ProcessEventStream(ctx, coreEvents)

		// 6. Yield all raw domain events to caller
		for de, err := range engineDomainEvents {
			if !yield(de, err) {
				return
			}
		}
	}
}

func (r *RunnerAdapter) toAgentThread(aggregate *domain.ThreadAggregate) core.AgentThread {
	agentThread := core.NewInMemoryThreadWithID(aggregate.ID)
	messages := r.toCoreMessages(aggregate)
	if len(messages) > 0 {
		_ = agentThread.AddMessages(messages)
	}
	return agentThread
}

func (r *RunnerAdapter) toCoreMessages(aggregate *domain.ThreadAggregate) []core.Message {
	if len(aggregate.Messages) == 0 {
		return nil
	}
	messages := make([]core.Message, len(aggregate.Messages))
	for i, dm := range aggregate.Messages {
		messages[i] = core.Message{
			ID:         dm.ID,
			Role:       dm.Role,
			Content:    dm.Content,
			Timestamp:  dm.Timestamp,
			ToolCalls:  r.toCoreToolCalls(dm.ToolCalls),
			ToolCallID: dm.ToolCallID,
			AgentName:  dm.AgentName,
			Metadata:   dm.Metadata,
		}

		// Inject active tool states if present
		if len(dm.ToolCalls) > 0 && len(aggregate.ActiveToolStates) > 0 {
			toolStates := make(map[string]domain.ToolCallState)
			for _, tc := range dm.ToolCalls {
				if state, ok := aggregate.ActiveToolStates[tc.ID]; ok {
					toolStates[tc.ID] = state
				}
			}

			if len(toolStates) > 0 {
				if messages[i].Metadata == nil {
					messages[i].Metadata = make(map[string]any)
				}
				messages[i].Metadata["tool_states"] = toolStates
			}
		}
	}
	return messages
}

func (r *RunnerAdapter) toCoreToolCalls(dtcs []domain.ToolCall) []core.ToolCall {
	if dtcs == nil {
		return nil
	}
	result := make([]core.ToolCall, len(dtcs))
	for i, dtc := range dtcs {
		result[i] = core.ToolCall{
			ID:        dtc.ID,
			Type:      dtc.Type,
			Name:      dtc.Name,
			Arguments: dtc.Arguments,
		}
	}
	return result
}

// RegistryAgentProvider implements engine.AgentProvider using agent.AgentProvider.
type RegistryAgentProvider struct {
	provider      agent.AgentProvider
	artifactStore artifact.Service
	enableMemory  bool
	profileRepo   domain.StudentProfileRepository
	modelProvider string // LLM provider name (e.g., "openai", "gemini")
	modelID       string // LLM model name (e.g., "gpt-4o", "gemini-2.0-flash")
}

func (p *RegistryAgentProvider) GetConfig(ctx context.Context, name string) (core.AgentConfig, error) {
	// Use provided provider
	a, err := p.provider.Get(name)
	if err != nil {
		return core.AgentConfig{}, fmt.Errorf("agent %s not found: %w", name, err)
	}

	// Assert that it is a CustomAgent to access the config
	// This is required because the generic agent.Agent interface hides implementation details
	// like SystemPrompt which the engine needs.
	customAgent, ok := a.(*CustomAgent)
	if !ok {
		return core.AgentConfig{}, fmt.Errorf("agent %s is not a custom agent", name)
	}

	agentConfig := customAgent.Config()

	// Map agent.Config to core.AgentConfig
	modelID := agentConfig.ModelID
	if modelID == "" {
		modelID = p.modelID // Fall back to runner-level model
	}
	config := core.AgentConfig{
		Name:          agentConfig.Name,
		SystemPrompt:  agentConfig.SystemPrompt,
		MaxIterations: agentConfig.MaxIterations,
		ModelID:       modelID,
		Provider:      p.modelProvider,
	}

	// Map Tools
	for _, t := range agentConfig.Tools {
		config.Tools = append(config.Tools, NewExecutableToolAdapter(t))
	}

	// Map Handoffs
	for _, h := range agentConfig.Handoffs {
		config.Handoffs = append(config.Handoffs, core.Handoff{
			AgentName:   h.AgentName,
			Description: h.Description,
		})
	}

	// Map Middlewares to their respective phases
	config.AgentMiddleware = append(config.AgentMiddleware, agentConfig.AgentMiddlewares...)
	config.ChatMiddleware = append(config.ChatMiddleware, agentConfig.ChatMiddlewares...)
	config.FunctionMiddleware = append(config.FunctionMiddleware, agentConfig.FunctionMiddlewares...)

	// Add Capabilities (Middleware)
	if agentConfig.Capabilities.InputElicitation {
		elicitMw := middleware.NewElicitationMiddleware()
		config.AgentMiddleware = append(config.AgentMiddleware, elicitMw.Middleware())
	}

	// Inject Memory Tool and Context if available and enabled
	if p.artifactStore != nil && p.enableMemory {
		memoryTool := memorytools.NewMemoryTool(p.artifactStore)
		config.Tools = append(config.Tools, memoryTool)

		memoryMiddleware := memorytools.NewMemoryInjectorMiddleware(memoryTool)
		config.AgentMiddleware = append(config.AgentMiddleware, AdaptRequestMiddlewareToUnified(memoryMiddleware))
	}

	// Inject Student Profile Middleware
	if p.profileRepo != nil {
		profileMiddleware := smiddleware.NewStudentProfileInjectorMiddleware(p.profileRepo)
		config.AgentMiddleware = append(config.AgentMiddleware, AdaptRequestMiddlewareToUnified(profileMiddleware))
	}

	return config, nil
}

// GetAgent returns an agent by name.
func (r *RunnerAdapter) GetAgent(name string) (agent.Agent, error) {
	return r.agentProvider.Get(name)
}

// ListAgents returns all available agent names.
func (r *RunnerAdapter) ListAgents() []string {
	agents := r.agentProvider.List()
	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name()
	}
	return names
}

// extractUserPrompt extracts the text from a message.
func extractUserPrompt(msg *agent.Message) string {
	if msg == nil {
		return ""
	}

	for _, c := range msg.Content {
		if c.Type == agent.ContentTypeText {
			return c.Text
		}
	}

	return ""
}
