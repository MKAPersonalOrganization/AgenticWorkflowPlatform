package custom

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/engine"
	"school-with-ai/foundation/agentsdk/llm"
	agentio "school-with-ai/foundation/agentsdk/io"
	smiddleware "school-with-ai/foundation/agentsdk/middleware"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/thread"
	"school-with-ai/modules/userprofile/identity"
	"school-with-ai/shared"
)

// ModelConfigResolver resolves LLM configuration for a given user.
// This is used for per-user AI settings.
type ModelConfigResolver interface {
	GetModelConfigForUser(ctx context.Context, userID string) (*agent.ModelConfig, error)
}

// TTSConfigResolver resolves TTS configuration for a given user.
type TTSConfigResolver interface {
	// GetTTSConfigForUserFlat returns the API key, model, and voice for a user's TTS config.
	// Returns empty strings (no error) if no TTS config is set.
	GetTTSConfigForUserFlat(ctx context.Context, userID string) (apiKey, model, voice string, err error)
}

// RunnerAdapter adapts the custom agent engine to the agent.Runner interface.
// All external capabilities are injected as narrow interfaces defined in
// interfaces.go — the adapter never imports module or service packages.
type RunnerAdapter struct {
	log                    *logger.Logger
	agentProvider          agent.AgentProvider
	threadMgr              ThreadManager
	pipeline               *EventPipeline
	profileRepo            identity.StudentProfileRepository
	configResolver         ModelConfigResolver
	ttsResolver            TTSConfigResolver
	pluginSettingsResolver PluginSettingsResolver
	permChecker            ToolPermissionChecker
	workspacePermChecker   WorkspacePermissionChecker
	mcpResolver            MCPToolResolver
	newToolSetFactory      NewToolSetFactoryFunc
	skillInjector          SkillPromptInjector
	edgequakeEnabled       bool
	edgequakeBaseURL       string
}

// RunnerAdapterConfig holds all dependencies for creating a RunnerAdapter.
type RunnerAdapterConfig struct {
	Log                    *logger.Logger
	ThreadMgr              ThreadManager
	AgentProvider          agent.AgentProvider
	ConfigResolver         ModelConfigResolver
	TTSResolver            TTSConfigResolver
	PluginSettingsResolver PluginSettingsResolver
	ProfileRepo            identity.StudentProfileRepository
	PermChecker            ToolPermissionChecker
	WorkspacePermChecker   WorkspacePermissionChecker
	MCPResolver            MCPToolResolver
	NewToolSetFactory      NewToolSetFactoryFunc
	SkillInjector          SkillPromptInjector
	EdgequakeEnabled       bool
	EdgequakeBaseURL       string
}

// NewRunnerAdapter creates a new runner adapter from the given config.
func NewRunnerAdapter(ctx context.Context, cfg RunnerAdapterConfig) (*RunnerAdapter, error) {
	if cfg.ConfigResolver == nil {
		return nil, fmt.Errorf("model config resolver is required")
	}

	r := &RunnerAdapter{
		log:                    cfg.Log,
		threadMgr:              cfg.ThreadMgr,
		agentProvider:          cfg.AgentProvider,
		profileRepo:            cfg.ProfileRepo,
		pipeline:               NewEventPipeline(cfg.Log),
		permChecker:            cfg.PermChecker,
		workspacePermChecker:   cfg.WorkspacePermChecker,
		mcpResolver:            cfg.MCPResolver,
		newToolSetFactory:      cfg.NewToolSetFactory,
		skillInjector:          cfg.SkillInjector,
		edgequakeEnabled:       cfg.EdgequakeEnabled,
		edgequakeBaseURL:       cfg.EdgequakeBaseURL,
		configResolver:         cfg.ConfigResolver,
		ttsResolver:            cfg.TTSResolver,
		pluginSettingsResolver: cfg.PluginSettingsResolver,
	}

	return r, nil
}

// buildEngine creates a per-request Engine using the user's model config.
func (r *RunnerAdapter) buildEngine(ctx context.Context, modelCfg *agent.ModelConfig, userID string) (*engine.Engine, error) {
	chatClient, err := createLLMClient(ctx, modelCfg)
	if err != nil {
		return nil, fmt.Errorf("creating LLM client: %w", err)
	}

	// Resolve TTS config for this user (non-fatal if unavailable)
	var ttsAPIKey, ttsModel, ttsVoice string
	if r.ttsResolver != nil && userID != "" {
		if key, model, voice, ttsErr := r.ttsResolver.GetTTSConfigForUserFlat(ctx, userID); ttsErr == nil {
			ttsAPIKey = key
			ttsModel = model
			ttsVoice = voice
		}
	}

	// Build per-request tool set factory with user's LLM and TTS config.
	var toolFactory ToolSetFactory
	if r.newToolSetFactory != nil {
		// Resolve per-user plugin settings (non-fatal if unavailable).
		var pluginSettings map[string]map[string]string
		if r.pluginSettingsResolver != nil && userID != "" {
			if ps, psErr := r.pluginSettingsResolver.GetAllPluginSettingsForUser(ctx, userID); psErr == nil {
				pluginSettings = ps
			}
		}

		toolFactory = r.newToolSetFactory(ToolFactoryConfig{
			EdgequakeBaseURL: r.edgequakeBaseURL,
			ModelAPIKey:      modelCfg.APIKey,
			ModelID:          modelCfg.Model,
			TTSAPIKey:        ttsAPIKey,
			TTSModel:         ttsModel,
			TTSVoice:         ttsVoice,
			PluginSettings:   pluginSettings,
		})
	}

	provider := &RegistryAgentProvider{
		provider:             r.agentProvider,
		profileRepo:          r.profileRepo,
		modelProvider:        modelCfg.Provider,
		modelID:              modelCfg.Model,
		temperature:          modelCfg.Temperature,
		maxIterations:        modelCfg.MaxIterations,
		maxTokens:            modelCfg.MaxTokens,
		toolFactory:          toolFactory,
		permChecker:          r.permChecker,
		workspacePermChecker: r.workspacePermChecker,
		edgequakeEnabled:     r.edgequakeEnabled,
		mcpResolver:          r.mcpResolver,
		skillInjector:        r.skillInjector,
	}

	return engine.New(chatClient, provider, nil, nil), nil
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

	case "anthropic":
		if cfg.APIKey == "" {
			return nil, fmt.Errorf("Anthropic API key is required")
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
func (r *RunnerAdapter) Run(ctx context.Context, req *agent.RunRequest) iter.Seq2[thread.Event, error] {
	return func(yield func(thread.Event, error) bool) {
		// 0. Resolve per-user model config
		modelCfg, err := r.configResolver.GetModelConfigForUser(ctx, req.UserID)
		if err != nil {
			yield(nil, fmt.Errorf("resolving AI settings for user: %w", err))
			return
		}

		// Build a per-request engine with the user's config
		eng, err := r.buildEngine(ctx, modelCfg, req.UserID)
		if err != nil {
			yield(nil, fmt.Errorf("building engine for user: %w", err))
			return
		}

		// 1. Facts First: Initialize Thread and Message
		var agg *thread.ThreadAggregate
		var initialEvents []thread.Event

		userContent := extractUserPrompt(req.Message)
		userAttachments := extractUserAttachments(req.Message)

		// Extract artifact attachment refs passed from orchestrator (for event persistence)
		var attachmentRefs []thread.Attachment
		if req.Context != nil {
			if refs, ok := req.Context["attachmentRefs"].([]thread.Attachment); ok {
				attachmentRefs = refs
			}
		}

		if req.Message != nil {
			messageID := req.Message.ID
			if messageID == "" {
				messageID = core.GenerateMessageID()
			}

			// AddUserMessage handles both thread creation and message persistence
			var uEvent *thread.UserMessageAdded
			var sideEffects []thread.Event
			var pErr error
			agg, uEvent, sideEffects, pErr = r.threadMgr.AddUserMessage(ctx, req.SessionID, messageID, userContent, req.UserID, attachmentRefs...)
			if pErr != nil {
				yield(nil, fmt.Errorf("failed to persist user message: %w", pErr))
				return
			}
			if uEvent != nil {
				initialEvents = append(initialEvents, uEvent)
			}
			// Include side-effect events (e.g. ThreadTitleUpdated from auto-titling)
			initialEvents = append(initialEvents, sideEffects...)
		} else {
			// Resuming without a new message
			var lErr error
			agg, lErr = r.threadMgr.Load(ctx, req.SessionID)
			if lErr != nil && !errors.Is(lErr, thread.ErrThreadNotFound) {
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

			// Inject attachment metadata into the latest user message so the LLM sees them
			if len(userAttachments) > 0 && len(messages) > 0 {
				for i := len(messages) - 1; i >= 0; i-- {
					if messages[i].Role == "user" {
						if messages[i].Metadata == nil {
							messages[i].Metadata = make(map[string]any)
						}
						messages[i].Metadata["attachments"] = userAttachments
						break
					}
				}
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
			MessageID:      core.GenerateMessageID(), // Always generate a fresh ID for the agent's first response
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
		// Fallback: empty agentName triggers the initial agent resolution in GetForUser.
		// No need to set a specific name — the provider handles it.

		// 5. Inject user into context for tools that need it (using shared.User)
		if runCtx.UserID != "" {
			ctx = context.WithValue(ctx, shared.UserContextKey, &shared.User{ID: runCtx.UserID})
		}

		// 6. Inject IO PauseHandler so tools can request user input/auth.
		// When a tool calls io.Read() or io.Confirm(), the PauseHandler returns
		// sentinel errors (ErrInputRequired / ErrAuthRequired) that cause the
		// engine to pause and emit ToolNeedsInput / ToolNeedsAuth events.
		ctx = agentio.WithHandler(ctx, agentio.NewPauseHandler())

		coreEvents := eng.Run(ctx, agentName, runCtx)

		// Convert core events to domain events
		engineDomainEvents := r.pipeline.ProcessEventStream(ctx, coreEvents)

		// 7. Yield initial events (UserMessageAdded, ThreadTitleUpdated, etc.)
		for _, ie := range initialEvents {
			if !yield(ie, nil) {
				return
			}
		}

		// 8. Yield all engine domain events to caller
		for de, err := range engineDomainEvents {
			if !yield(de, err) {
				return
			}
		}
	}
}

func (r *RunnerAdapter) toAgentThread(aggregate *thread.ThreadAggregate) core.AgentThread {
	agentThread := core.NewInMemoryThreadWithID(aggregate.ID)
	messages := r.toCoreMessages(aggregate)
	if len(messages) > 0 {
		_ = agentThread.AddMessages(messages)
	}
	return agentThread
}

func (r *RunnerAdapter) toCoreMessages(aggregate *thread.ThreadAggregate) []core.Message {
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
			toolStates := make(map[string]thread.ToolCallState)
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

func (r *RunnerAdapter) toCoreToolCalls(dtcs []thread.ToolCall) []core.ToolCall {
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
	provider             agent.AgentProvider
	profileRepo          identity.StudentProfileRepository
	modelProvider        string   // LLM provider name (e.g., "openai", "gemini")
	modelID              string   // LLM model name (e.g., "gpt-4o", "gemini-2.0-flash")
	temperature          *float64 // Global temperature from AI settings
	maxIterations        int      // Global max iterations from AI settings
	maxTokens            *int     // Global max tokens from AI settings
	toolFactory          ToolSetFactory
	permChecker          ToolPermissionChecker
	workspacePermChecker WorkspacePermissionChecker
	edgequakeEnabled     bool
	mcpResolver          MCPToolResolver
	skillInjector        SkillPromptInjector
}

func (p *RegistryAgentProvider) GetConfig(ctx context.Context, name string) (core.AgentConfig, error) {
	// Try user-aware provider first if available
	var a agent.Agent
	var err error

	type userAware interface {
		GetForUser(ctx context.Context, name, userID string) (agent.Agent, error)
	}

	if uap, ok := p.provider.(userAware); ok {
		// Extract userID from context if available
		userID := ""
		if u, ok := ctx.Value(shared.UserContextKey).(*shared.User); ok && u != nil {
			userID = u.ID
		}
		a, err = uap.GetForUser(ctx, name, userID)
		if err != nil {
			// Fallback to standard provider
			a, err = p.provider.Get(name)
			if err != nil {
				return core.AgentConfig{}, fmt.Errorf("agent %s not found: %w", name, err)
			}
		}
	} else {
		a, err = p.provider.Get(name)
		if err != nil {
			return core.AgentConfig{}, fmt.Errorf("agent %s not found: %w", name, err)
		}
	}

	// Extract config from the agent — all agents are dbAgent wrappers around CustomAgent.
	var agentConfig agent.Config
	if da, ok := a.(*dbAgent); ok {
		if inner, ok := da.Agent.(*CustomAgent); ok {
			agentConfig = inner.Config()
			agentConfig.Name = da.id
			agentConfig.Description = da.Description()
		} else {
			return core.AgentConfig{}, fmt.Errorf("agent %s has unsupported inner type", name)
		}
	} else if ca, ok := a.(*CustomAgent); ok {
		// Fallback for any non-DB agent (e.g. tests)
		agentConfig = ca.Config()
	} else {
		return core.AgentConfig{}, fmt.Errorf("agent %s is not a custom agent", name)
	}

	// Map agent.Config to core.AgentConfig — model, temperature, and max iterations
	// are always from global AI settings; agents only carry behavioral config (prompt, tools).
	maxIter := p.maxIterations
	if maxIter <= 0 {
		maxIter = agentConfig.MaxIterations // fallback to framework default
	}
	config := core.AgentConfig{
		Name:          agentConfig.Name,
		SystemPrompt:  agentConfig.SystemPrompt,
		MaxIterations: maxIter,
		Temperature:   p.temperature,
		MaxTokens:     p.maxTokens,
		ModelID:       p.modelID,
		Provider:      p.modelProvider,
	}

	// Get allowed tool sets for this agent from permissions
	if p.permChecker != nil && p.toolFactory != nil {
		// Get user from context to check permissions
		user := shared.GetUserFromContext(ctx)
		var userID string
		if user != nil {
			userID = user.ID
		}

		// Get enabled tool sets for this agent
		enabledToolSetIDs, err := p.permChecker.GetEnabledToolSets(ctx, userID, name)
		if err != nil {
			// Log error but continue with no tool sets
			enabledToolSetIDs = []string{}
		}

		// Add tools from enabled tool sets (skip MCP sets — handled separately below)
		for _, toolSetID := range enabledToolSetIDs {
			if strings.HasPrefix(toolSetID, "mcp-") {
				continue // MCP toolsets handled below
			}
			toolsFromSet, err := p.toolFactory.CreateExecutableToolsFromSet(ctx, toolSetID)
			if err != nil {
				// Skip tool sets that can't be processed
				continue
			}
			config.Tools = append(config.Tools, toolsFromSet...)
		}

		// Resolve MCP tools from user's connected MCP servers
		if p.mcpResolver != nil && userID != "" {
			mcpTools := p.mcpResolver.ResolveEnabledTools(ctx, userID,
				func(toolSetID string) bool {
					return p.permChecker.HasToolSetPermission(ctx, userID, name, toolSetID)
				},
				func(toolSetID, toolID string) bool {
					return p.permChecker.HasPermission(ctx, userID, name, toolSetID, toolID)
				},
			)
			config.Tools = append(config.Tools, mcpTools...)
		}
	}

	// Inject attached workspace info into system prompt as Knowledge Bases section
	if p.workspacePermChecker != nil && p.edgequakeEnabled {
		// Get user from context to check permissions
		user := shared.GetUserFromContext(ctx)
		var userID string
		if user != nil {
			userID = user.ID
		}

		enabledWorkspaceIDs, err := p.workspacePermChecker.GetEnabledWorkspaces(ctx, userID, name)
		if err == nil && len(enabledWorkspaceIDs) > 0 {
			// Enforce allowed namespaces at the tool level (runtime security).
			if p.toolFactory != nil {
				p.toolFactory.SetAllowedKBNamespaces(enabledWorkspaceIDs)
			}

			kbSection := "\n\n## Knowledge Bases\n\nYou have access to the following knowledge base workspaces. Use the query_knowledge_base tool to search them:\n"
			for _, wsID := range enabledWorkspaceIDs {
				kbSection += fmt.Sprintf("- Workspace ID: %s\n", wsID)
			}
			kbSection += "\nWhen the user asks about topics that may be covered by these knowledge bases, use the query_knowledge_base tool with the appropriate namespace (workspace ID) and a relevant query to retrieve information."
			config.SystemPrompt += kbSection
		}
	}

	// Inject activated skill instructions into system prompt.
	// Skills are activated via the activate_skill tool and stored in the tracker.
	// By injecting here (instead of returning as tool result), instructions stay
	// pinned at the system prompt level and don't drift down the conversation.
	if p.skillInjector != nil {
		threadID := shared.GetThreadIDFromContext(ctx)
		if addition := p.skillInjector.BuildPromptAddition(threadID); addition != "" {
			config.SystemPrompt += "\n\n" + addition
		}
	}

	// Map Middlewares to their respective phases
	config.AgentMiddleware = append(config.AgentMiddleware, agentConfig.AgentMiddlewares...)
	config.ChatMiddleware = append(config.ChatMiddleware, agentConfig.ChatMiddlewares...)
	config.FunctionMiddleware = append(config.FunctionMiddleware, agentConfig.FunctionMiddlewares...)

	// =================================================================
	// Declarative Requirements System
	// =================================================================
	// Auto-collect tools from Requirements that implement ToolProvider.
	for _, req := range agentConfig.Requirements {
		if tp, ok := req.(agent.ToolProvider); ok {
			config.Tools = append(config.Tools, tp.ProvidedTools()...)
		}
	}

	// Auto-collect prompt additions from Requirements that implement PromptProvider.
	for _, req := range agentConfig.Requirements {
		if pp, ok := req.(agent.PromptProvider); ok {
			config.SystemPrompt += "\n\n" + pp.PromptAddition()
		}
	}

	// Auto-collect FunctionMiddleware from Requirements that implement FunctionMiddlewareProvider.
	// This enables requirements like AskPermissionRequirement to inject tool-execution
	// middleware without users manually adding it to FunctionMiddlewares.
	for _, req := range agentConfig.Requirements {
		if fmp, ok := req.(agent.FunctionMiddlewareProvider); ok {
			config.FunctionMiddleware = append(config.FunctionMiddleware, fmp.FunctionMiddleware())
		}
	}

	// Wire up the RequirementMiddleware if we have any requirements.
	if len(agentConfig.Requirements) > 0 {
		config.ChatMiddleware = append(config.ChatMiddleware, agent.RequirementMiddleware(agentConfig.Requirements...))
	}

	// Inject Student Profile as a Chat-level middleware
	if p.profileRepo != nil {
		config.ChatMiddleware = append(config.ChatMiddleware, smiddleware.NewStudentProfileInjectorMiddleware(p.profileRepo))
	}

	return config, nil
}

// GetAgent returns an agent by name, supporting user-aware providers.
func (r *RunnerAdapter) GetAgent(name string) (agent.Agent, error) {
	return r.agentProvider.Get(name)
}

// GetAgentForUser returns an agent by name for a specific user.
func (r *RunnerAdapter) GetAgentForUser(ctx context.Context, name, userID string) (agent.Agent, error) {
	type userAware interface {
		GetForUser(ctx context.Context, name, userID string) (agent.Agent, error)
	}

	if uap, ok := r.agentProvider.(userAware); ok {
		return uap.GetForUser(ctx, name, userID)
	}
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

// extractUserAttachments extracts multimodal content from a message as a
// []map[string]any suitable for storing in core.Message.Metadata["attachments"].
// Handles images, audio, and file attachments uniformly.
func extractUserAttachments(msg *agent.Message) []map[string]any {
	if msg == nil {
		return nil
	}

	var attachments []map[string]any
	for _, c := range msg.Content {
		switch c.Type {
		case agent.ContentTypeImageURL:
			if c.ImageURL != nil {
				detail := c.ImageURL.Detail
				if detail == "" {
					detail = "auto"
				}
				m := map[string]any{
					"url":      c.ImageURL.URL,
					"mimeType": mimeFromDataURL(c.ImageURL.URL),
					"detail":   detail,
				}
				attachments = append(attachments, m)
			}
		case agent.ContentTypeInputAudio:
			if c.AudioInput != nil {
				mimeType := "audio/" + c.AudioInput.Format
				dataURL := "data:" + mimeType + ";base64," + c.AudioInput.Data
				attachments = append(attachments, map[string]any{
					"url":      dataURL,
					"mimeType": mimeType,
				})
			}
		case agent.ContentTypeFile:
			if c.FileInput != nil {
				mimeType := c.FileInput.MimeType
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}
				dataURL := "data:" + mimeType + ";base64," + c.FileInput.Data
				m := map[string]any{
					"url":      dataURL,
					"mimeType": mimeType,
				}
				if c.FileInput.FileName != "" {
					m["fileName"] = c.FileInput.FileName
				}
				attachments = append(attachments, m)
			}
		}
	}
	return attachments
}

// mimeFromDataURL extracts the MIME type from a data URL.
// Returns "image/png" as default if parsing fails.
func mimeFromDataURL(dataURL string) string {
	// data:image/png;base64,...
	if !strings.HasPrefix(dataURL, "data:") {
		return "image/png"
	}
	rest := dataURL[5:]
	if idx := strings.Index(rest, ";"); idx > 0 {
		return rest[:idx]
	}
	return "image/png"
}
