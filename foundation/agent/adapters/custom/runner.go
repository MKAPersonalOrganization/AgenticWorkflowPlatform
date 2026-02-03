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
	"school-with-ai/foundation/agentsdk/registry"
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
	artifactStore artifact.Service
	profileRepo   domain.StudentProfileRepository
	enableMemory  bool
	pipeline      *EventPipeline // Add event pipeline for observability
}

// NewRunnerAdapter creates a new runner adapter.
func NewRunnerAdapter(
	log *logger.Logger,
	threadMgr ThreadManager,
	chatClient engine.ChatClient,
	artifactStore artifact.Service,
	profileRepo domain.StudentProfileRepository,
	enableMemory bool,
) *RunnerAdapter {
	r := &RunnerAdapter{
		log:           log,
		threadMgr:     threadMgr,
		artifactStore: artifactStore,
		profileRepo:   profileRepo,
		enableMemory:  enableMemory,
		pipeline:      NewEventPipeline(log), // Initialize event pipeline
	}

	provider := &RegistryAgentProvider{
		registry:      registry.GlobalAgents,
		artifactStore: r.artifactStore,
		profileRepo:   r.profileRepo,
		enableMemory:  r.enableMemory,
	}

	r.engine = engine.New(
		chatClient,
		provider,
		nil, // reqCtxCapture
		nil, // toolTracer
	)

	return r
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

		// 5. Run engine and process events
		coreEvents := r.engine.Run(ctx, req.AgentName, runCtx)

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

// RegistryAgentProvider implements engine.AgentProvider using GlobalAgents.
type RegistryAgentProvider struct {
	registry      *registry.AgentRegistry
	artifactStore artifact.Service
	enableMemory  bool
	profileRepo   domain.StudentProfileRepository
}

func (p *RegistryAgentProvider) GetConfig(ctx context.Context, name string) (core.AgentConfig, error) {
	// Use provided registry
	factory, ok := p.registry.Get(name)
	if !ok {
		return core.AgentConfig{}, fmt.Errorf("agent %s not found", name)
	}

	// Create new agent config using factory
	agentConfig := factory(ctx)

	// Map agent.Config to core.AgentConfig
	config := core.AgentConfig{
		Name:          agentConfig.Name,
		SystemPrompt:  agentConfig.SystemPrompt,
		MaxIterations: agentConfig.MaxIterations,
		ModelID:       agentConfig.ModelID,
	}

	// Map Tools
	for _, t := range agentConfig.Tools {
		config.Tools = append(config.Tools, WrapTool(t))
	}

	// Map Handoffs
	for _, h := range agentConfig.Handoffs {
		config.Handoffs = append(config.Handoffs, core.Handoff{
			AgentName:   h.AgentName,
			Description: h.Description,
		})
	}

	// Add Capabilities (Middleware)
	if agentConfig.Capabilities.InputElicitation {
		config.Middleware = append(config.Middleware, middleware.NewElicitationMiddleware())
	}

	// Inject Memory Tool and Context if available and enabled
	if p.artifactStore != nil && p.enableMemory {
		memoryTool := memorytools.NewMemoryTool(p.artifactStore)
		// We need to wrap these core tools if they are agent.Tools?
		// memorytools.NewMemoryTool returns core.Tool or agent.Tool?
		// Currently services/memory/tools/memory_tool.go imports core.
		// So it returns core.Tool.
		// We can append directly to config.Tools (which is []core.Tool).
		config.Tools = append(config.Tools, memoryTool.Definition()...)

		memoryMiddleware := memorytools.NewMemoryInjectorMiddleware(memoryTool)
		config.Middleware = append(config.Middleware, memoryMiddleware)
	}

	// Inject Student Profile Middleware
	if p.profileRepo != nil {
		profileMiddleware := smiddleware.NewStudentProfileInjectorMiddleware(p.profileRepo)
		config.Middleware = append(config.Middleware, profileMiddleware)
	}

	return config, nil
}

// GetAgent returns an agent by name.
func (r *RunnerAdapter) GetAgent(name string) (agent.Agent, error) {
	factory, ok := registry.GlobalAgents.Get(name)
	if !ok {
		return nil, fmt.Errorf("agent not found: %s", name)
	}

	config := factory(context.Background())
	return NewAgentAdapter(config), nil
}

// ListAgents returns all available agent names.
func (r *RunnerAdapter) ListAgents() []string {
	return registry.GlobalAgents.List()
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
