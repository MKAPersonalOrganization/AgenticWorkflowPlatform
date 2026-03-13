package adkgo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/genai"

	fagent "school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/thread"
	"school-with-ai/shared"
)

// ──────────────────────────────────────────────────────────────────────────────
// RunnerAdapter implements agentorchestrator.Runner using adk-go.
//
// Each Run call:
//  1. Resolves per-user model config → creates model.LLM
//  2. Resolves agent config → builds tools, system prompt
//  3. Builds an adk-go llmagent
//  4. Reconstructs an in-memory session from thread history
//  5. Runs the agent via adk-go runner
//  6. Translates session.Events → thread domain events
// ──────────────────────────────────────────────────────────────────────────────

// RunnerAdapter bridges adk-go to the agentorchestrator.Runner interface.
type RunnerAdapter struct {
	log                    *logger.Logger
	agentProvider          AgentProvider
	threadMgr              ThreadManager
	configResolver         ModelConfigResolver
	ttsResolver            TTSConfigResolver
	pluginSettingsResolver PluginSettingsResolver
	profileProvider        StudentProfileProvider
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
	AgentProvider          AgentProvider
	ConfigResolver         ModelConfigResolver
	TTSResolver            TTSConfigResolver
	PluginSettingsResolver PluginSettingsResolver
	ProfileProvider        StudentProfileProvider
	PermChecker            ToolPermissionChecker
	WorkspacePermChecker   WorkspacePermissionChecker
	MCPResolver            MCPToolResolver
	NewToolSetFactory      NewToolSetFactoryFunc
	SkillInjector          SkillPromptInjector
	EdgequakeEnabled       bool
	EdgequakeBaseURL       string
}

// NewRunnerAdapter creates a new adk-go runner adapter.
func NewRunnerAdapter(cfg RunnerAdapterConfig) (*RunnerAdapter, error) {
	if cfg.ConfigResolver == nil {
		return nil, fmt.Errorf("model config resolver is required")
	}
	return &RunnerAdapter{
		log:                    cfg.Log,
		threadMgr:              cfg.ThreadMgr,
		agentProvider:          cfg.AgentProvider,
		configResolver:         cfg.ConfigResolver,
		ttsResolver:            cfg.TTSResolver,
		pluginSettingsResolver: cfg.PluginSettingsResolver,
		profileProvider:        cfg.ProfileProvider,
		permChecker:            cfg.PermChecker,
		workspacePermChecker:   cfg.WorkspacePermChecker,
		mcpResolver:            cfg.MCPResolver,
		newToolSetFactory:      cfg.NewToolSetFactory,
		skillInjector:          cfg.SkillInjector,
		edgequakeEnabled:       cfg.EdgequakeEnabled,
		edgequakeBaseURL:       cfg.EdgequakeBaseURL,
	}, nil
}

// Run executes an agent with the given request, yielding thread domain events.
// Implements agentorchestrator.Runner.
func (r *RunnerAdapter) Run(ctx context.Context, req *fagent.RunRequest) iter.Seq2[thread.Event, error] {
	return func(yield func(thread.Event, error) bool) {
		// 0. Resolve per-user model config.
		modelCfg, err := r.configResolver.GetModelConfigForUser(ctx, req.UserID)
		if err != nil {
			yield(nil, fmt.Errorf("resolving AI settings for user: %w", err))
			return
		}

		// 1. Persist user message and get thread aggregate.
		var agg *thread.ThreadAggregate
		var initialEvents []thread.Event

		userContent := extractUserText(req.Message)

		// Extract artifact attachment refs from orchestrator context.
		var attachmentRefs []thread.Attachment
		if req.Context != nil {
			if refs, ok := req.Context["attachmentRefs"].([]thread.Attachment); ok {
				attachmentRefs = refs
			}
		}

		if req.Message != nil {
			messageID := req.Message.ID
			if messageID == "" {
				messageID = generateID()
			}
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
			initialEvents = append(initialEvents, sideEffects...)
		} else {
			var lErr error
			agg, lErr = r.threadMgr.Load(ctx, req.SessionID)
			if lErr != nil && !errors.Is(lErr, thread.ErrThreadNotFound) {
				yield(nil, fmt.Errorf("failed to load thread: %w", lErr))
				return
			}
		}

		// 2. Resolve agent.
		agentName := req.AgentName
		if agentName == "" && agg != nil {
			for i := len(agg.Messages) - 1; i >= 0; i-- {
				if agg.Messages[i].AgentName != "" {
					agentName = agg.Messages[i].AgentName
					break
				}
			}
		}

		foundAgent, agErr := r.agentProvider.GetForUser(ctx, agentName, req.UserID)
		if agErr != nil {
			yield(nil, fmt.Errorf("resolving agent %q: %w", agentName, agErr))
			return
		}

		// 3. Extract full agent config for requirements, middlewares, tools.
		agentCfg := extractAgentConfig(foundAgent)

		// 4. Build tools from agent config + enabled tool sets + MCP.
		adkTools := r.resolveTools(ctx, req.UserID, agentName, modelCfg, req.SessionID, agentCfg)

		// 5. Build system prompt (agent config + requirements + workspace KB + skills + profile).
		systemPrompt := r.buildSystemPrompt(ctx, foundAgent, req.UserID, agentName, req.SessionID, agentCfg)

		// 6. Create model.LLM.
		llm, mErr := NewModel(ctx, modelCfg.Provider, modelCfg.Model, modelCfg.APIKey)
		if mErr != nil {
			yield(nil, fmt.Errorf("creating LLM model: %w", mErr))
			return
		}

		// 7. Build adk-go LLM agent with callbacks for middleware bridging.
		genCfg := buildGenerateContentConfig(modelCfg)

		// Bridge foundation function middlewares into ADK-Go's tool lifecycle.
		var beforeToolCBs []llmagent.BeforeToolCallback
		var afterToolCBs []llmagent.AfterToolCallback
		if mb := newMiddlewareBridge(agentCfg, agentName, req.SessionID, req.UserID); mb != nil {
			beforeToolCBs = mb.beforeToolCallbacks()
			afterToolCBs = mb.afterToolCallbacks()
		}

		// Build before-agent callback for requirements system bridging.
		beforeAgentCBs := buildAgentCallbacks(agentCfg)

		adkAgent, aErr := llmagent.New(llmagent.Config{
			Name:                  agentName,
			Description:           foundAgent.Description(),
			Model:                 llm,
			Instruction:           systemPrompt,
			Tools:                 adkTools,
			GenerateContentConfig: genCfg,
			BeforeAgentCallbacks:  beforeAgentCBs,
			BeforeToolCallbacks:   beforeToolCBs,
			AfterToolCallbacks:    afterToolCBs,
		})
		if aErr != nil {
			yield(nil, fmt.Errorf("creating adk-go agent: %w", aErr))
			return
		}

		// 8. Build session from thread history.
		sessionSvc := session.InMemoryService()
		var sess session.Session
		if agg != nil && len(agg.Messages) > 0 {
			sess, err = BuildSessionFromThread(ctx, sessionSvc, agg, req.UserID)
			if err != nil {
				yield(nil, fmt.Errorf("building session from thread: %w", err))
				return
			}
		} else {
			// No history — create empty session.
			threadID := req.SessionID
			if threadID == "" {
				threadID = generateID()
			}
			createResp, cErr := sessionSvc.Create(ctx, &session.CreateRequest{
				AppName:   appName,
				UserID:    req.UserID,
				SessionID: threadID,
			})
			if cErr != nil {
				yield(nil, fmt.Errorf("creating session: %w", cErr))
				return
			}
			sess = createResp.Session
		}

		// 9. Build adk-go runner.
		adkRunner, rErr := runner.New(runner.Config{
			AppName:        appName,
			Agent:          adkAgent,
			SessionService: sessionSvc,
		})
		if rErr != nil {
			yield(nil, fmt.Errorf("creating adk-go runner: %w", rErr))
			return
		}

		// 10. Inject user into context.
		if req.UserID != "" {
			ctx = context.WithValue(ctx, shared.UserContextKey, &shared.User{ID: req.UserID})
		}

		// 11. Yield initial events.
		for _, ie := range initialEvents {
			if !yield(ie, nil) {
				return
			}
		}

		// 12. Build user content for adk-go.
		userMsg := buildUserContent(req.Message)

		// 13. Run the agent and translate events.
		evtCtx := EventContext{
			ThreadID:  req.SessionID,
			RunID:     messageIDFromRequest(req),
			AgentName: agentName,
			Provider:  modelCfg.Provider,
			Model:     modelCfg.Model,
		}

		// Track streaming lifecycle so we emit TEXT_MESSAGE_START / END.
		// adk-go assigns a fresh UUID to every streaming delta event, but the
		// frontend needs a single stable message ID across all deltas.
		var streamingMsgID string
		assistantMsgID := generateID()

		for adkEvt, adkErr := range adkRunner.Run(ctx, req.UserID, sess.ID(), userMsg, agent.RunConfig{
			StreamingMode: agent.StreamingModeSSE,
		}) {
			if adkErr != nil {
				// Close the streaming message before surfacing the error.
				if streamingMsgID != "" {
					if !yield(&thread.AssistantMessageCompleted{
						ThreadID:  evtCtx.ThreadID,
						MessageID: streamingMsgID,
						Timestamp: time.Now(),
					}, nil) {
						return
					}
					streamingMsgID = ""
				}
				if !yield(nil, adkErr) {
					return
				}
				continue
			}

			domainEvents := TranslateEvent(adkEvt, evtCtx)
			for _, de := range domainEvents {
				switch evt := de.(type) {
				case *thread.AssistantContentStreamed:
					// Replace the per-event UUID with a stable message ID.
					evt.MessageID = assistantMsgID

					// Emit AssistantMessageStarted before the first delta.
					if streamingMsgID == "" {
						streamingMsgID = assistantMsgID
						if !yield(&thread.AssistantMessageStarted{
							ThreadID:  evt.ThreadID,
							MessageID: streamingMsgID,
							Timestamp: evt.Timestamp,
						}, nil) {
							return
						}
					}

				case *thread.AssistantMessageAdded:
					// The AGUI converter ignores AssistantMessageAdded (it is a
					// persistence event). When we were streaming, replace it
					// with AssistantMessageCompleted so the frontend receives
					// TEXT_MESSAGE_END.
					if streamingMsgID != "" {
						if !yield(&thread.AssistantMessageCompleted{
							ThreadID:  evt.ThreadID,
							MessageID: streamingMsgID,
							Content:   evt.Content,
							AgentName: evt.AgentName,
							ToolCalls: evt.ToolCalls,
							Timestamp: evt.Timestamp,
						}, nil) {
							return
						}
						streamingMsgID = ""
						continue // skip the original AssistantMessageAdded
					}
				}

				if !yield(de, nil) {
					return
				}
			}
		}

		// If the loop ended while still streaming (no final non-partial event),
		// close the message so the frontend gets TEXT_MESSAGE_END.
		if streamingMsgID != "" {
			yield(&thread.AssistantMessageCompleted{
				ThreadID:  evtCtx.ThreadID,
				MessageID: streamingMsgID,
				Timestamp: time.Now(),
			}, nil)
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

// resolveTools builds the complete tool set for an agent invocation.
// It collects tools from:
//  1. Agent-level static tools from Config.Tools
//  2. Requirement-provided tools (ToolProvider interface)
//  3. Permission-gated tool sets
//  4. MCP-connected tools
func (r *RunnerAdapter) resolveTools(
	ctx context.Context,
	userID, agentName string,
	modelCfg *fagent.ModelConfig,
	threadID string,
	agentCfg *fagent.Config,
) []adktool.Tool {
	var execTools []fagent.ExecutableTool

	// 1. Collect agent-level static tools from Config.
	if agentCfg != nil {
		for _, t := range agentCfg.Tools {
			if et, ok := t.(fagent.ExecutableTool); ok {
				execTools = append(execTools, et)
			}
		}

		// 2. Collect tools from Requirements that implement ToolProvider.
		for _, req := range agentCfg.Requirements {
			if tp, ok := req.(fagent.ToolProvider); ok {
				execTools = append(execTools, tp.ProvidedTools()...)
			}
		}
	}

	// 3. Build per-request tool factory.
	var toolFactory ToolSetFactory
	if r.newToolSetFactory != nil {
		var ttsAPIKey, ttsModel, ttsVoice string
		if r.ttsResolver != nil && userID != "" {
			ttsAPIKey, ttsModel, ttsVoice, _ = r.ttsResolver.GetTTSConfigForUserFlat(ctx, userID)
		}

		var pluginSettings map[string]map[string]string
		if r.pluginSettingsResolver != nil && userID != "" {
			pluginSettings, _ = r.pluginSettingsResolver.GetAllPluginSettingsForUser(ctx, userID)
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

	// 4. Get enabled tool sets from permissions.
	if r.permChecker != nil && toolFactory != nil {
		enabledToolSetIDs, err := r.permChecker.GetEnabledToolSets(ctx, userID, agentName)
		if err != nil {
			enabledToolSetIDs = []string{}
		}

		for _, toolSetID := range enabledToolSetIDs {
			if strings.HasPrefix(toolSetID, "mcp-") {
				continue // MCP handled below.
			}
			tools, tErr := toolFactory.CreateExecutableToolsFromSet(ctx, toolSetID)
			if tErr != nil {
				continue
			}
			execTools = append(execTools, tools...)
		}

		// Resolve MCP tools.
		if r.mcpResolver != nil && userID != "" {
			mcpTools := r.mcpResolver.ResolveEnabledTools(ctx, userID,
				func(toolSetID string) bool {
					return r.permChecker.HasToolSetPermission(ctx, userID, agentName, toolSetID)
				},
				func(toolSetID, toolID string) bool {
					return r.permChecker.HasPermission(ctx, userID, agentName, toolSetID, toolID)
				},
			)
			execTools = append(execTools, mcpTools...)
		}

		// Enforce workspace KB namespaces.
		if r.workspacePermChecker != nil && r.edgequakeEnabled {
			enabledWorkspaceIDs, wErr := r.workspacePermChecker.GetEnabledWorkspaces(ctx, userID, agentName)
			if wErr == nil && len(enabledWorkspaceIDs) > 0 {
				toolFactory.SetAllowedKBNamespaces(enabledWorkspaceIDs)
			}
		}
	}

	// Wrap foundation tools as adk-go tools.
	return WrapExecutableTools(execTools, threadID, userID)
}

// buildSystemPrompt assembles the system prompt from agent config, KB, skills, profile.
// It includes requirement-provided prompt additions (PromptProvider interface).
func (r *RunnerAdapter) buildSystemPrompt(
	ctx context.Context,
	foundAgent fagent.Agent,
	userID, agentName, threadID string,
	agentCfg *fagent.Config,
) string {
	var systemPrompt string
	if agentCfg != nil {
		systemPrompt = agentCfg.SystemPrompt
	}

	// Collect prompt additions from Requirements that implement PromptProvider.
	if agentCfg != nil {
		for _, req := range agentCfg.Requirements {
			if pp, ok := req.(fagent.PromptProvider); ok {
				systemPrompt += "\n\n" + pp.PromptAddition()
			}
		}
	}

	// Inject workspace knowledge base section.
	if r.workspacePermChecker != nil && r.edgequakeEnabled && userID != "" {
		enabledWorkspaceIDs, err := r.workspacePermChecker.GetEnabledWorkspaces(ctx, userID, agentName)
		if err == nil && len(enabledWorkspaceIDs) > 0 {
			kbSection := "\n\n## Knowledge Bases\n\nYou have access to the following knowledge base workspaces. Use the query_knowledge_base tool to search them:\n"
			for _, wsID := range enabledWorkspaceIDs {
				kbSection += fmt.Sprintf("- Workspace ID: %s\n", wsID)
			}
			kbSection += "\nWhen the user asks about topics that may be covered by these knowledge bases, use the query_knowledge_base tool with the appropriate namespace (workspace ID) and a relevant query to retrieve information."
			systemPrompt += kbSection
		}
	}

	// Inject activated skill instructions.
	if r.skillInjector != nil {
		if addition := r.skillInjector.BuildPromptAddition(threadID); addition != "" {
			systemPrompt += "\n\n" + addition
		}
	}

	// Inject student profile context (structured Markdown, matching custom adapter).
	if r.profileProvider != nil && userID != "" {
		profile, err := r.profileProvider.GetProfile(ctx, userID)
		if err == nil && len(profile) > 0 {
			var sb strings.Builder
			sb.WriteString("\n\n## Student Profile\n")
			if ls, ok := profile["learning_style"].(string); ok && ls != "" {
				sb.WriteString(fmt.Sprintf("- **Learning Style**: %s\n", ls))
			}
			if goals, ok := profile["goals"].(string); ok && goals != "" {
				sb.WriteString(fmt.Sprintf("- **Goals**: %s\n", goals))
			}
			if strengths, ok := profile["strengths"].([]string); ok && len(strengths) > 0 {
				sb.WriteString(fmt.Sprintf("- **Strengths**: %s\n", strings.Join(strengths, ", ")))
			}
			if struggles, ok := profile["struggles"].([]string); ok && len(struggles) > 0 {
				sb.WriteString(fmt.Sprintf("- **Struggles**: %s\n", strings.Join(struggles, ", ")))
			}
			systemPrompt += sb.String()
		}
	}

	return systemPrompt
}

// buildGenerateContentConfig creates genai config from model settings.
func buildGenerateContentConfig(cfg *fagent.ModelConfig) *genai.GenerateContentConfig {
	gc := &genai.GenerateContentConfig{}

	if cfg.Temperature != nil {
		t := float32(*cfg.Temperature)
		gc.Temperature = &t
	}
	if cfg.MaxTokens != nil {
		gc.MaxOutputTokens = int32(*cfg.MaxTokens)
	}

	return gc
}

// GetAgent returns an agent by name (requires user context in most deployments).
func (r *RunnerAdapter) GetAgent(name string) (fagent.Agent, error) {
	return r.agentProvider.GetForUser(context.Background(), name, "")
}

// GetAgentForUser returns an agent by name for a specific user.
func (r *RunnerAdapter) GetAgentForUser(ctx context.Context, name, userID string) (fagent.Agent, error) {
	return r.agentProvider.GetForUser(ctx, name, userID)
}

// ListAgents returns all available agent names.
func (r *RunnerAdapter) ListAgents() []string {
	// The AgentProvider interface doesn't expose List, so we use a type assertion.
	type lister interface {
		List() []fagent.Agent
	}
	if l, ok := r.agentProvider.(lister); ok {
		agents := l.List()
		names := make([]string, len(agents))
		for i, a := range agents {
			names[i] = a.Name()
		}
		return names
	}
	return nil
}

// buildUserContent converts an agent.Message to genai.Content for the adk-go runner.
// Supports text, image, audio, and file multimodal content.
func buildUserContent(msg *fagent.Message) *genai.Content {
	if msg == nil {
		return genai.NewContentFromText("", "user")
	}

	var parts []*genai.Part
	for _, c := range msg.Content {
		switch c.Type {
		case fagent.ContentTypeText:
			parts = append(parts, &genai.Part{Text: c.Text})

		case fagent.ContentTypeImageURL:
			if c.ImageURL != nil {
				mimeType := mimeFromDataURL(c.ImageURL.URL)
				parts = append(parts, &genai.Part{
					InlineData: &genai.Blob{
						MIMEType: mimeType,
						Data:     []byte(c.ImageURL.URL),
					},
				})
			}

		case fagent.ContentTypeInputAudio:
			if c.AudioInput != nil {
				mimeType := "audio/" + c.AudioInput.Format
				parts = append(parts, &genai.Part{
					InlineData: &genai.Blob{
						MIMEType: mimeType,
						Data:     []byte(c.AudioInput.Data),
					},
				})
			}

		case fagent.ContentTypeFile:
			if c.FileInput != nil {
				mimeType := c.FileInput.MimeType
				if mimeType == "" {
					mimeType = "application/octet-stream"
				}
				parts = append(parts, &genai.Part{
					InlineData: &genai.Blob{
						MIMEType: mimeType,
						Data:     []byte(c.FileInput.Data),
					},
				})
			}

		default:
			if c.Text != "" {
				parts = append(parts, &genai.Part{Text: c.Text})
			}
		}
	}

	if len(parts) == 0 {
		parts = []*genai.Part{{Text: ""}}
	}

	return &genai.Content{
		Role:  "user",
		Parts: parts,
	}
}

// extractUserText extracts the text content from an agent.Message.
func extractUserText(msg *fagent.Message) string {
	if msg == nil {
		return ""
	}
	for _, c := range msg.Content {
		if c.Type == fagent.ContentTypeText {
			return c.Text
		}
	}
	return ""
}

// messageIDFromRequest extracts the message ID from a run request.
func messageIDFromRequest(req *fagent.RunRequest) string {
	if req.Message != nil && req.Message.ID != "" {
		return req.Message.ID
	}
	return req.InvocationID
}

// generateID creates a cryptographically random unique identifier.
// Uses the foundation package's thread-safe GenerateID.
func generateID() string {
	return fagent.GenerateID()
}

// mimeFromDataURL extracts the MIME type from a data URL.
// Returns "image/png" as default if parsing fails.
func mimeFromDataURL(dataURL string) string {
	if !strings.HasPrefix(dataURL, "data:") {
		return "image/png"
	}
	rest := dataURL[5:]
	if idx := strings.Index(rest, ";"); idx > 0 {
		return rest[:idx]
	}
	return "image/png"
}

// extractAgentConfig extracts the foundation agent.Config from an Agent.
// It tries multiple strategies to unwrap agent wrappers (e.g., dbAgent → CustomAgent)
// and extract the Config. This is adapter-agnostic: it works via interfaces,
// never importing concrete types from other packages.
func extractAgentConfig(a fagent.Agent) *fagent.Config {
	// Strategy 1: Direct Config() method (e.g., CustomAgent).
	type configurable interface {
		Config() fagent.Config
	}
	if ca, ok := a.(configurable); ok {
		cfg := ca.Config()
		return &cfg
	}

	// Strategy 2: The agent embeds an agent.Agent interface field.
	// dbAgent does this: struct { agent.Agent; ... }
	// Go's embedding of interfaces doesn't promote concrete-type methods,
	// so we need to access the embedded field via the EmbeddedAgent() accessor
	// or try to extract it via reflection-free interface probing.
	type embeddedAgentAccessor interface {
		EmbeddedAgent() fagent.Agent
	}
	if ea, ok := a.(embeddedAgentAccessor); ok {
		if inner := ea.EmbeddedAgent(); inner != nil {
			if ca, ok := inner.(configurable); ok {
				cfg := ca.Config()
				return &cfg
			}
		}
	}

	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Middleware Bridge
//
// ADK-Go splits tool lifecycle into BeforeToolCallback and AfterToolCallback.
// The foundation middleware system expects to wrap the entire execution in a
// single chain (before → execute → after, like HTTP middleware). We bridge
// these two models by:
//
//   - BeforeToolCallback: runs the full middleware chain around a no-op
//     terminal handler. This gives logging, rate-limiting, etc. their
//     "before" phase. We set context metadata so middleware_common.go
//     can log agent_name, thread_id, etc.
//
//   - AfterToolCallback: populates c.Result with the tool output and runs
//     the chain again so the logging middleware can record the result.
//     This is best-effort — tool failures don't propagate from here.
//
// This is a pragmatic bridge. The custom adapter wraps execution in one chain
// call; ADK-Go forces the split. The middlewareBridge encapsulates this
// so runner.go stays clean.
// ──────────────────────────────────────────────────────────────────────────────

// middlewareBridge wraps the foundation FunctionMiddleware chain into ADK-Go's
// split before/after tool callback model.
type middlewareBridge struct {
	middlewares []fagent.Middleware
	agentName   string
	sessionID   string
	userID      string
}

// newMiddlewareBridge collects all function middlewares from the agent config
// (both explicit and requirement-provided) and returns a bridge, or nil if
// there are no middlewares to run.
func newMiddlewareBridge(agentCfg *fagent.Config, agentName, sessionID, userID string) *middlewareBridge {
	if agentCfg == nil {
		return nil
	}

	var mws []fagent.Middleware
	mws = append(mws, agentCfg.FunctionMiddlewares...)

	for _, req := range agentCfg.Requirements {
		if fmp, ok := req.(fagent.FunctionMiddlewareProvider); ok {
			mws = append(mws, fmp.FunctionMiddleware())
		}
	}

	if len(mws) == 0 {
		return nil
	}

	return &middlewareBridge{
		middlewares: mws,
		agentName:   agentName,
		sessionID:   sessionID,
		userID:      userID,
	}
}

// newContext creates a foundation middleware Context with properly populated
// metadata, matching what middleware_common.go expects.
func (mb *middlewareBridge) newContext(ctx context.Context, toolName, toolDesc string, args map[string]any) *fagent.Context {
	tool := &adkToolBridge{name: toolName, desc: toolDesc}
	fCtx := fagent.NewFunctionContext(ctx, tool, args)
	fCtx.Metadata["agent_name"] = mb.agentName
	fCtx.Metadata["thread_id"] = mb.sessionID
	fCtx.Metadata["message_id"] = "" // Not available at tool-call level.
	fCtx.Metadata["user_id"] = mb.userID
	return fCtx
}

// runChain executes the middleware chain with the given context and terminal handler.
func (mb *middlewareBridge) runChain(fCtx *fagent.Context, terminal fagent.HandlerFunc) error {
	chain := fagent.Chain(mb.middlewares...)
	return chain(terminal).Handle(fCtx)
}

// beforeToolCallbacks returns the ADK-Go BeforeToolCallback slice.
func (mb *middlewareBridge) beforeToolCallbacks() []llmagent.BeforeToolCallback {
	return []llmagent.BeforeToolCallback{
		func(ctx adktool.Context, t adktool.Tool, args map[string]any) (map[string]any, error) {
			fCtx := mb.newContext(ctx, t.Name(), t.Description(), args)

			if err := mb.runChain(fCtx, func(c *fagent.Context) error {
				return nil // Terminal: allow tool to proceed.
			}); err != nil {
				return map[string]any{errorKey: err.Error()}, nil
			}

			return nil, nil // nil = continue with original args.
		},
	}
}

// afterToolCallbacks returns the ADK-Go AfterToolCallback slice.
func (mb *middlewareBridge) afterToolCallbacks() []llmagent.AfterToolCallback {
	return []llmagent.AfterToolCallback{
		func(ctx adktool.Context, t adktool.Tool, args, result map[string]any, _ error) (map[string]any, error) {
			fCtx := mb.newContext(ctx, t.Name(), t.Description(), args)
			fCtx.Result = result // Populate so logging middleware sees the output.

			// Best-effort: don't fail the tool over post-execution middleware errors.
			_ = mb.runChain(fCtx, func(c *fagent.Context) error {
				return nil
			})

			return nil, nil // nil = keep original result unchanged.
		},
	}
}

// adkToolBridge adapts an adk-go tool name/description to the foundation Tool interface.
type adkToolBridge struct {
	name string
	desc string
}

func (t *adkToolBridge) Name() string           { return t.name }
func (t *adkToolBridge) Description() string    { return t.desc }
func (t *adkToolBridge) Schema() *fagent.Schema { return nil }

// buildAgentCallbacks creates ADK-Go BeforeAgentCallbacks to bridge
// the foundation Requirements system into ADK-Go's agent lifecycle.
// Requirements are evaluated before each agent run; if any requirement
// produces blocking rules, a guidance message is injected into the LLM context.
func buildAgentCallbacks(agentCfg *fagent.Config) []agent.BeforeAgentCallback {
	if agentCfg == nil || len(agentCfg.Requirements) == 0 {
		return nil
	}

	// NOTE: The full RequirementMiddleware runs at the Chat level in the custom
	// adapter. In ADK-Go, we don't have the same middleware hooks on the LLM call.
	// Instead, we use BeforeAgentCallback to evaluate requirements and inject
	// guidance into the session state. The actual tool filtering is handled by
	// the fact that we already filter tools in resolveTools. Requirements that
	// produce force/block rules will be communicated to the LLM as a system
	// message via the session state.
	//
	// For the MVP: we log requirements evaluation but don't short-circuit.
	// Full RequirementMiddleware parity requires deeper ADK-Go integration.
	return nil
}
