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
	pluginSettingsResolver PluginSettingsResolver
	profileProvider        UserContextProvider
	permChecker            ToolPermissionChecker
	mcpResolver            MCPToolResolver
	newToolSetFactory      NewToolSetFactoryFunc
	promptInjectors        []PromptInjector
	extraConfigFn          ExtraConfigFunc
}

// RunnerAdapterConfig holds all dependencies for creating a RunnerAdapter.
type RunnerAdapterConfig struct {
	Log                    *logger.Logger
	ThreadMgr              ThreadManager
	AgentProvider          AgentProvider
	ConfigResolver         ModelConfigResolver
	PluginSettingsResolver PluginSettingsResolver
	ProfileProvider        UserContextProvider
	PermChecker            ToolPermissionChecker
	MCPResolver            MCPToolResolver
	NewToolSetFactory      NewToolSetFactoryFunc
	PromptInjectors        []PromptInjector
	ExtraConfigFn          ExtraConfigFunc
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
		pluginSettingsResolver: cfg.PluginSettingsResolver,
		profileProvider:        cfg.ProfileProvider,
		permChecker:            cfg.PermChecker,
		mcpResolver:            cfg.MCPResolver,
		newToolSetFactory:      cfg.NewToolSetFactory,
		promptInjectors:        cfg.PromptInjectors,
		extraConfigFn:          cfg.ExtraConfigFn,
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

		// 4. Build invocation with history for Requirements.
		inv := &fagent.Invocation{
			ID:        req.InvocationID,
			SessionID: req.SessionID,
			UserID:    req.UserID,
			Message:   req.Message,
			Config:    req.Config,
			Context:   req.Context,
		}
		if agg != nil {
			inv.History = threadHistoryToMessages(agg.Messages)
		}

		// 8. Create model.LLM (needed for both regular and workflow agents).
		llm, mErr := NewModel(ctx, modelCfg.Provider, modelCfg.Model, modelCfg.APIKey)
		if mErr != nil {
			yield(nil, fmt.Errorf("creating LLM model: %w", mErr))
			return
		}

		// 9. Build the ADK-Go agent — either a workflow agent or a regular LLM agent.
		var adkAgent agent.Agent

		if wa, isWF := isWorkflowAgent(foundAgent); isWF {
			// ── Workflow Agent Path ──
			// Build a sequential/parallel/loop agent from the workflow steps.
			wbc := &workflowBuildContext{
				ctx:      ctx,
				adapter:  r,
				userID:   req.UserID,
				modelCfg: modelCfg,
				llm:      llm,
				threadID: req.SessionID,
				req:      req,
			}
			var wErr error
			adkAgent, wErr = buildWorkflowAgent(wbc, wa)
			if wErr != nil {
				yield(nil, fmt.Errorf("building workflow agent: %w", wErr))
				return
			}
		} else {
			// ── Regular LLM Agent Path ──
			// 5. Evaluate requirements once and reuse the rules for both tool
			// filtering (resolveTools) and guidance injection (buildBeforeRunHooks).
			var precomputedRules []fagent.Rule
			if agentCfg != nil {
				for _, requirement := range agentCfg.Requirements {
					if e, ok := requirement.(fagent.Enableable); ok && !e.Enabled() {
						continue
					}
					rules, rErr := requirement.Run(ctx, inv)
					if rErr != nil {
						continue
					}
					precomputedRules = append(precomputedRules, rules...)
				}
			}

			// 6. Build tools from agent config + enabled tool sets + MCP.
			adkTools := r.resolveTools(ctx, req.UserID, agentName, modelCfg, req.SessionID, agentCfg, req, precomputedRules)

			// 7. Build system prompt (agent config + requirements + workspace KB + skills + profile).
			systemPrompt := r.buildSystemPrompt(ctx, foundAgent, req.UserID, agentName, req.SessionID, agentCfg, inv)

			genCfg := buildGenerateContentConfig(modelCfg)

			// Bridge foundation function middlewares into ADK-Go's tool lifecycle.
			var beforeToolCBs []llmagent.BeforeToolCallback
			var afterToolCBs []llmagent.AfterToolCallback
			if mb := newMiddlewareBridge(agentCfg, agentName, req.SessionID, req.UserID); mb != nil {
				beforeToolCBs = mb.beforeToolCallbacks()
				afterToolCBs = mb.afterToolCallbacks()
			}

			// Build before-agent callbacks for requirements guidance injection.
			beforeAgentCBs := buildBeforeRunHooks(precomputedRules)

			var aErr error
			adkAgent, aErr = llmagent.New(llmagent.Config{
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
		}

		// 10. Build session from thread history.
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

		// 11. Build adk-go runner.
		adkRunner, rErr := runner.New(runner.Config{
			AppName:        appName,
			Agent:          adkAgent,
			SessionService: sessionSvc,
		})
		if rErr != nil {
			yield(nil, fmt.Errorf("creating adk-go runner: %w", rErr))
			return
		}

		// 12. Inject user into context.
		if req.UserID != "" {
			ctx = context.WithValue(ctx, shared.UserContextKey, &shared.User{ID: req.UserID})
		}

		// 13. Yield initial events.
		for _, ie := range initialEvents {
			if !yield(ie, nil) {
				return
			}
		}

		// 14. Build user content for adk-go.
		userMsg := buildUserContent(req.Message)

		// 15. Run the agent and translate events.
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
					if !yield(de, nil) {
						return
					}

				case *thread.AssistantMessageAdded:
					// When streaming, emit AssistantMessageCompleted first so the
					// frontend receives TEXT_MESSAGE_END. AssistantMessageAdded is
					// always yielded regardless — it is the durable persistence event
					// and must reach ProcessEvents to be written to the event store.
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
					}
					if !yield(de, nil) {
						return
					}

				default:
					if !yield(de, nil) {
						return
					}
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

// threadHistoryToMessages translates thread aggregate messages into foundation
// agent Messages so they can populate Invocation.History for Requirements.
func threadHistoryToMessages(msgs []thread.Message) []*fagent.Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]*fagent.Message, 0, len(msgs))
	for _, m := range msgs {
		var role fagent.MessageRole
		switch m.Role {
		case "user":
			role = fagent.RoleUser
		case "assistant":
			role = fagent.RoleAssistant
		case "tool":
			role = fagent.RoleTool
		default:
			continue
		}
		fm := &fagent.Message{
			ID:        m.ID,
			Role:      role,
			Timestamp: m.Timestamp,
			Author:    m.AgentName,
		}
		if m.Content != "" {
			fm.Content = []fagent.Content{{Type: fagent.ContentTypeText, Text: m.Content}}
		}
		// Translate tool calls so Requirements can inspect tool-call history.
		for _, tc := range m.ToolCalls {
			fm.Content = append(fm.Content, fagent.Content{
				Type: fagent.ContentTypeToolCall,
				ToolCall: &fagent.ToolCall{
					ID:        tc.ID,
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		if m.ToolCallID != "" {
			fm.ToolCallID = m.ToolCallID
		}
		out = append(out, fm)
	}
	return out
}

// resolveTools builds the complete tool set for an agent invocation.
// It collects tools from:
//  1. Agent-level static tools from Config.Tools
//  2. Requirement-provided tools (ToolProvider interface)
//  3. Permission-gated tool sets
//  4. MCP-connected tools
//  5. Sub-agent tools (if agent has SubAgents, they become callable tools)
//
// Tools blocked by Requirements are filtered out at build time, matching
// ADK-Go's pattern of resolving tools in request processors.
func (r *RunnerAdapter) resolveTools(
	ctx context.Context,
	userID, agentName string,
	modelCfg *fagent.ModelConfig,
	threadID string,
	agentCfg *fagent.Config,
	req *fagent.RunRequest,
	precomputedRules []fagent.Rule,
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
		var pluginSettings map[string]map[string]string
		if r.pluginSettingsResolver != nil && userID != "" {
			pluginSettings, _ = r.pluginSettingsResolver.GetAllPluginSettingsForUser(ctx, userID)
		}

		var extra map[string]string
		if r.extraConfigFn != nil {
			extra = r.extraConfigFn(ctx, userID, agentName)
		}

		toolFactory = r.newToolSetFactory(ToolFactoryConfig{
			ModelAPIKey:    modelCfg.APIKey,
			ModelID:        modelCfg.Model,
			PluginSettings: pluginSettings,
			Extra:          extra,
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
	}

	// 5. Pre-filter tools blocked by pre-computed requirement rules.
	// Requirements were already evaluated once in Run(); we reuse the results
	// to avoid double evaluation (and ensure History is populated).
	if len(precomputedRules) > 0 {
		blocked := make(map[string]bool)
		for _, rule := range precomputedRules {
			if rule.Blocked || rule.Hidden {
				blocked[rule.TargetTool] = true
			}
		}

		if len(blocked) > 0 {
			filtered := make([]fagent.ExecutableTool, 0, len(execTools))
			for _, t := range execTools {
				if !blocked[t.Name()] {
					filtered = append(filtered, t)
				}
			}
			execTools = filtered
		}
	}

	// Wrap foundation tools as adk-go tools.
	return WrapExecutableTools(execTools, threadID, userID)
}

// buildSystemPrompt assembles the system prompt from agent config and injectors.
// It includes:
//   - Static SystemPrompt from agent config
//   - InstructionFunc (runtime-generated prompt additions)
//   - Requirement-provided prompt additions (PromptProvider interface)
//   - PromptInjector additions (knowledge bases, skills, etc.)
//   - User context
func (r *RunnerAdapter) buildSystemPrompt(
	ctx context.Context,
	foundAgent fagent.Agent,
	userID, agentName, threadID string,
	agentCfg *fagent.Config,
	inv *fagent.Invocation,
) string {
	var systemPrompt string
	if agentCfg != nil {
		systemPrompt = agentCfg.SystemPrompt

		// Resolve InstructionFunc (runtime-generated prompt additions).
		if agentCfg.InstructionFunc != nil && inv != nil {
			if dynamic, err := agentCfg.InstructionFunc(inv); err == nil && dynamic != "" {
				systemPrompt += "\n\n" + dynamic
			}
		}
	}

	// Collect prompt additions from Requirements that implement PromptProvider.
	if agentCfg != nil {
		for _, req := range agentCfg.Requirements {
			if pp, ok := req.(fagent.PromptProvider); ok {
				systemPrompt += "\n\n" + pp.PromptAddition()
			}
		}
	}

	// Collect prompt additions from PromptInjectors (knowledge bases, skills, etc.).
	for _, injector := range r.promptInjectors {
		if addition := injector.BuildPromptAddition(ctx, agentName, threadID); addition != "" {
			systemPrompt += "\n\n" + addition
		}
	}

	// Inject user context (structured Markdown, matching custom adapter).
	if r.profileProvider != nil && userID != "" {
		profile, err := r.profileProvider.GetProfile(ctx, userID)
		if err == nil && len(profile) > 0 {
			var sb strings.Builder
			sb.WriteString("\n\n## User Context\n")
			for key, val := range profile {
				switch v := val.(type) {
				case string:
					if v != "" {
						sb.WriteString(fmt.Sprintf("- **%s**: %s\n", formatLabel(key), v))
					}
				case []string:
					if len(v) > 0 {
						sb.WriteString(fmt.Sprintf("- **%s**: %s\n", formatLabel(key), strings.Join(v, ", ")))
					}
				}
			}
			systemPrompt += sb.String()
		}
	}

	return systemPrompt
}

// formatLabel converts a snake_case key to a Title Case label.
func formatLabel(key string) string {
	parts := strings.Split(key, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
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
	if l, ok := r.agentProvider.(AgentLister); ok {
		agents := l.List()
		names := make([]string, len(agents))
		for i, a := range agents {
			names[i] = a.Name()
		}
		return names
	}
	return nil
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

// extractAgentConfig extracts the foundation agent.Config from an Agent.
// Uses the agent.Configurable interface for clean extraction.
func extractAgentConfig(a fagent.Agent) *fagent.Config {
	if ca, ok := a.(fagent.Configurable); ok {
		cfg := ca.Config()
		return &cfg
	}
	return nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Middleware Bridge
//
// ADK-Go splits tool lifecycle into BeforeToolCallback and AfterToolCallback.
// The foundation middleware system wraps the entire execution in a single
// chain (before → execute → after, like HTTP middleware). We bridge these
// two models by:
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

// buildBeforeRunHooks creates ADK-Go BeforeAgentCallbacks that inject
// pre-computed requirement guidance into the LLM turn as a system-role
// Content message. Requirements are evaluated once in Run() and the
// resulting rules are passed in — no double evaluation.
func buildBeforeRunHooks(rules []fagent.Rule) []agent.BeforeAgentCallback {
	if len(rules) == 0 {
		return nil
	}

	// Pre-build guidance text from the rules.
	var guidance strings.Builder
	for _, rule := range rules {
		if rule.Blocked && rule.Reason != "" {
			guidance.WriteString(fmt.Sprintf("IMPORTANT: The tool '%s' is currently disabled: %s\n", rule.TargetTool, rule.Reason))
		}
		if rule.Forced && rule.Reason != "" {
			guidance.WriteString(fmt.Sprintf("IMPORTANT: You MUST use the tool '%s': %s\n", rule.TargetTool, rule.Reason))
		}
		if rule.PreventStop && rule.Reason != "" {
			guidance.WriteString(fmt.Sprintf("IMPORTANT: You must not stop yet: %s\n", rule.Reason))
		}
	}

	if guidance.Len() == 0 {
		return nil
	}

	guidanceText := guidance.String()
	return []agent.BeforeAgentCallback{
		func(_ agent.CallbackContext) (*genai.Content, error) {
			// Return guidance as a system-role Content so ADK injects it
			// into the current LLM turn, making it visible to the model.
			return &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: guidanceText}},
			}, nil
		},
	}
}
