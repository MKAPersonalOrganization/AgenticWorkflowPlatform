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
	"school-with-ai/services/edgequake"
	"school-with-ai/services/memory"
	memorytools "school-with-ai/services/memory/tools"
	"school-with-ai/services/permissions"
	smiddleware "school-with-ai/services/studentprofile/middleware"
	"school-with-ai/services/toolregistry"
	"school-with-ai/services/workspacepermissions"
)

// RunnerAdapter adapts the custom engine to the agent.Runner interface.
type RunnerAdapter struct {
	log                *logger.Logger
	threadMgr          ThreadManager // Interface for thread management
	engine             *engine.Engine
	agentProvider      agent.AgentProvider
	artifactStore      artifact.Service
	profileRepo        domain.StudentProfileRepository
	enableMemory       bool
	pipeline           *EventPipeline // Add event pipeline for observability
	toolRegistry       *toolregistry.Registry
	permissionsService *permissions.Service
	workspacePermSvc   *workspacepermissions.Service
	edgequakeService   *edgequake.Service
	edgequakeBaseURL   string
	edgequakeTenantID  string
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
	toolRegistry *toolregistry.Registry,
	permissionsService *permissions.Service,
	workspacePermSvc *workspacepermissions.Service,
	edgequakeService *edgequake.Service,
	edgequakeBaseURL string,
	edgequakeTenantID string,
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
		log:                log,
		threadMgr:          threadMgr,
		agentProvider:      agentProvider,
		artifactStore:      artifactStore,
		profileRepo:        profileRepo,
		enableMemory:       enableMemory,
		pipeline:           NewEventPipeline(log), // Initialize event pipeline
		toolRegistry:       toolRegistry,
		permissionsService: permissionsService,
		workspacePermSvc:   workspacePermSvc,
		edgequakeService:   edgequakeService,
		edgequakeBaseURL:   edgequakeBaseURL,
		edgequakeTenantID:  edgequakeTenantID,
	}

	provider := &RegistryAgentProvider{
		provider:           agentProvider,
		artifactStore:      r.artifactStore,
		profileRepo:        r.profileRepo,
		enableMemory:       r.enableMemory,
		modelProvider:      modelCfg.Provider,
		modelID:            modelCfg.Model,
		modelAPIKey:        modelCfg.APIKey,
		toolRegistry:       toolRegistry,
		permissionsService: permissionsService,
		workspacePermSvc:   workspacePermSvc,
		edgequakeService:   edgequakeService,
		edgequakeBaseURL:   edgequakeBaseURL,
		edgequakeTenantID:  edgequakeTenantID,
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

		// 5. Inject user into context for tools that need it (using domain.User)
		if runCtx.UserID != "" {
			ctx = context.WithValue(ctx, domain.UserContextKey, &domain.User{ID: runCtx.UserID})
		}

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
	provider           agent.AgentProvider
	artifactStore      artifact.Service
	enableMemory       bool
	profileRepo        domain.StudentProfileRepository
	modelProvider      string // LLM provider name (e.g., "openai", "gemini")
	modelID            string // LLM model name (e.g., "gpt-4o", "gemini-2.0-flash")
	modelAPIKey        string // LLM API key for sub-agent tool calls
	toolRegistry       *toolregistry.Registry
	permissionsService *permissions.Service
	workspacePermSvc   *workspacepermissions.Service
	edgequakeService   *edgequake.Service
	edgequakeBaseURL   string
	edgequakeTenantID  string
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

	// Get allowed tool sets for this agent from permissions service
	if p.permissionsService != nil && p.toolRegistry != nil {
		// Create tool factory
		factory := toolregistry.NewExecutableToolFactory(p.artifactStore)
		factory.SetToolRegistry(p.toolRegistry)

		// Wire edgequake config for KB tools
		if p.edgequakeBaseURL != "" {
			factory.SetEdgequakeConfig(p.edgequakeBaseURL, p.modelAPIKey, p.modelID)
		}

		// Get user from context to check permissions
		user := domain.GetUserFromContext(ctx)
		var userID string
		if user != nil {
			userID = user.ID
		}

		// Get enabled tool sets for this agent
		enabledToolSetIDs, err := p.permissionsService.GetEnabledToolSets(ctx, userID, name)
		if err != nil {
			// Log error but continue with no tool sets
			enabledToolSetIDs = []string{}
		}

		// Add tools from enabled tool sets
		for _, toolSetID := range enabledToolSetIDs {
			toolsFromSet, err := factory.CreateExecutableToolsFromSet(ctx, toolSetID)
			if err != nil {
				// Skip tool sets that can't be processed
				continue
			}
			config.Tools = append(config.Tools, toolsFromSet...)
		}
	}

	// Inject attached workspace info into system prompt as Knowledge Bases section
	if p.workspacePermSvc != nil && p.edgequakeService != nil {
		// Get user from context to check permissions
		user := domain.GetUserFromContext(ctx)
		var userID string
		if user != nil {
			userID = user.ID
		}

		enabledWorkspaceIDs, err := p.workspacePermSvc.GetEnabledWorkspaces(ctx, userID, name)
		if err == nil && len(enabledWorkspaceIDs) > 0 {
			kbSection := "\n\n## Knowledge Bases\n\nYou have access to the following knowledge base workspaces. Use the query_knowledge_base tool to search them:\n"
			for _, wsID := range enabledWorkspaceIDs {
				kbSection += fmt.Sprintf("- Workspace ID: %s\n", wsID)
			}
			kbSection += "\nWhen the user asks about topics that may be covered by these knowledge bases, use the query_knowledge_base tool with the appropriate namespace (workspace ID) and a relevant query to retrieve information."
			config.SystemPrompt += kbSection
		}
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

	// Inject Memory Context if available and enabled (Tools are now handled via factory/registry)
	if p.artifactStore != nil && p.enableMemory {
		memoryService := memory.NewArtifactMemoryService(p.artifactStore)
		memoryMiddleware := memorytools.NewMemoryInjectorMiddleware(memoryService)
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
