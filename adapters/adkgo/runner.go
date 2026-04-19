package adkgo

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"strings"
	"time"

	agentio "aicompass/foundation/agent/io"
	"aicompass/foundation/auth"
	"aicompass/foundation/logger"
	"aicompass/modules/chat/thread"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/memory"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/plugin/retryandreflect"
	adkrunner "google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/loadartifactstool"
	"google.golang.org/adk/tool/loadmemorytool"
	"google.golang.org/adk/tool/preloadmemorytool"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"

	fagent "aicompass/foundation/agent"
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
	apiKeyResolver         APIKeyResolver
	pluginSettingsResolver PluginSettingsResolver
	profileProvider        UserContextProvider
	toolReqs               ToolRequirementsService
	blockEnforcer          BlockEnforcer
	confirmChecker         ConfirmationChecker
	mcpResolver            MCPToolResolver
	a2aResolver            A2AToolResolver
	a2aSubAgentProvider    A2ASubAgentProvider
	newToolSetFactory      NewToolSetFactoryFunc
	promptInjectors        []PromptInjector
	memoryService          memory.Service
	artifactService        artifact.Service
	extraConfigFn          ExtraConfigFunc
}

// RunnerAdapterConfig holds all dependencies for creating a RunnerAdapter.
type RunnerAdapterConfig struct {
	Log                    *logger.Logger
	ThreadMgr              ThreadManager
	AgentProvider          AgentProvider
	ConfigResolver         ModelConfigResolver
	APIKeyResolver         APIKeyResolver
	PluginSettingsResolver PluginSettingsResolver
	ProfileProvider        UserContextProvider
	ToolReqs               ToolRequirementsService
	BlockEnforcer          BlockEnforcer
	ConfirmChecker         ConfirmationChecker
	MCPResolver            MCPToolResolver
	A2AResolver            A2AToolResolver
	A2ASubAgentProvider    A2ASubAgentProvider
	NewToolSetFactory      NewToolSetFactoryFunc
	PromptInjectors        []PromptInjector
	MemoryService          memory.Service   // required; inject memdb.New(db) for persistent storage
	ArtifactService        artifact.Service // optional; inject for versioned file management
	ExtraConfigFn          ExtraConfigFunc
}

// NewRunnerAdapter creates a new adk-go runner adapter.
func NewRunnerAdapter(cfg RunnerAdapterConfig) (*RunnerAdapter, error) {
	if cfg.ConfigResolver == nil {
		return nil, fmt.Errorf("model config resolver is required")
	}
	if cfg.MemoryService == nil {
		return nil, fmt.Errorf("memory service is required")
	}
	return &RunnerAdapter{
		log:                    cfg.Log,
		threadMgr:              cfg.ThreadMgr,
		agentProvider:          cfg.AgentProvider,
		configResolver:         cfg.ConfigResolver,
		apiKeyResolver:         cfg.APIKeyResolver,
		pluginSettingsResolver: cfg.PluginSettingsResolver,
		profileProvider:        cfg.ProfileProvider,
		toolReqs:               cfg.ToolReqs,
		blockEnforcer:          cfg.BlockEnforcer,
		confirmChecker:         cfg.ConfirmChecker,
		mcpResolver:            cfg.MCPResolver,
		a2aResolver:            cfg.A2AResolver,
		a2aSubAgentProvider:    cfg.A2ASubAgentProvider,
		newToolSetFactory:      cfg.NewToolSetFactory,
		promptInjectors:        cfg.PromptInjectors,
		memoryService:          cfg.MemoryService,
		artifactService:        cfg.ArtifactService,
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
		agg, toolCallID, initialEvents, err := r.loadOrSeedThread(ctx, req)
		if err != nil {
			yield(nil, err)
			return
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

		// 3b. Apply per-agent model override — swaps provider/model/apiKey when set.
		if agentCfg != nil && agentCfg.ModelOverride != nil {
			overridden, ovErr := r.applyModelOverride(ctx, req.UserID, modelCfg, agentCfg.ModelOverride)
			if ovErr != nil {
				yield(nil, ovErr)
				return
			}
			modelCfg = overridden
		}

		// 4. Build session from thread history (if any).
		//
		// When there is existing conversation history, we replay it into an
		// adk-go session so the LLM agent sees prior context. For fresh
		// conversations (no history), we skip this — the runner's
		// AutoCreateSession will create the session on the first Run call.
		//
		// Session state from history must be available before Invocation is
		// constructed so inv.State reflects values accumulated by prior agents
		// (e.g. via Config.OutputKey).
		//
		// Session state is persisted via StateChangeReported events to the event
		// store. State is restored from ThreadAggregate.State during BuildSessionFromThread.
		sessionSvc := session.InMemoryService()
		var sess session.Session
		if agg != nil && len(agg.Messages) > 0 {
			sess, err = BuildSessionFromThread(ctx, sessionSvc, agg, req.UserID)
			if err != nil {
				yield(nil, fmt.Errorf("building session from thread: %w", err))
				return
			}
		}

		// 5. Build invocation with history and live session state for Requirements.
		var state fagent.ReadonlyState
		if sess != nil {
			state = &adkSessionState{state: sess.State()}
		}
		inv := &fagent.Invocation{
			ID:        req.InvocationID,
			SessionID: req.SessionID,
			UserID:    req.UserID,
			Message:   req.Message,
			Config:    req.Config,
			Context:   req.Context,
			State:     state,
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
		var agentPositions map[string]AgentPosition
		memEnabled, memPreload := r.resolveMemoryPolicy(agentCfg)
		memoryEnabled := r.memoryService != nil && memEnabled
		artEnabled, artAutoSaveBlobs := r.resolveArtifactPolicy(agentCfg)
		artifactsEnabled := r.artifactService != nil && artEnabled

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
			adkAgent, agentPositions, wErr = buildWorkflowAgent(wbc, wa)
			if wErr != nil {
				yield(nil, fmt.Errorf("building workflow agent: %w", wErr))
				return
			}
		} else {
			// ── Regular LLM Agent Path ──
			var aErr error
			adkAgent, aErr = r.buildRegularLLMAgent(ctx, req, agentName, foundAgent, agentCfg, modelCfg, llm, inv,
				memoryEnabled, memPreload, artifactsEnabled)
			if aErr != nil {
				yield(nil, aErr)
				return
			}
		}

		// 11. Build adk-go runner with RetryAndReflect plugin.
		// RetryAndReflect (ADK-Go v1.0.0) intercepts tool errors and injects a
		// structured reflection prompt into the LLM context, guiding the model
		// to diagnose and retry with corrected arguments. Up to 3 retries are
		// allowed per invocation; on exhaustion the plugin tells the model to
		// stop using the failing tool rather than returning a raw error.
		runnerCfg := adkrunner.Config{
			AppName:           appName,
			Agent:             adkAgent,
			SessionService:    sessionSvc,
			AutoCreateSession: true,
			PluginConfig: adkrunner.PluginConfig{
				Plugins: []*plugin.Plugin{
					retryandreflect.MustNew(
						retryandreflect.WithMaxRetries(3),
						retryandreflect.WithTrackingScope(retryandreflect.Invocation),
						retryandreflect.WithErrorIfRetryExceeded(false),
					),
				},
			},
		}
		if memoryEnabled {
			runnerCfg.MemoryService = r.memoryService
		}
		if artifactsEnabled {
			runnerCfg.ArtifactService = r.artifactService
		}
		adkRunner, rErr := adkrunner.New(runnerCfg)
		if rErr != nil {
			yield(nil, fmt.Errorf("creating adk-go runner: %w", rErr))
			return
		}

		// 12. Inject user + IO handler into context.
		ctx = r.injectRunContext(ctx, req, toolCallID)

		// 13. Yield initial events.
		for _, ie := range initialEvents {
			if !yield(ie, nil) {
				return
			}
		}

		// 14. Build user content for adk-go.
		userMsg := buildRunUserMessage(req, agg, toolCallID)

		// 15. Run the agent and translate events.
		//
		// Per-event translation + streaming-lifecycle bookkeeping lives in
		// runner_loop.go. State that accumulates across iterations (current
		// author, streaming/assistant message IDs, node start time, loop
		// iteration counters) is held in runLoopState so the outer iterator
		// stays flat.
		loopState := &runLoopState{
			evtCtx: EventContext{
				ThreadID:  req.SessionID,
				RunID:     messageIDFromRequest(req),
				AgentName: agentName,
				Provider:  modelCfg.Provider,
				Model:     modelCfg.Model,
			},
			agentPositions: agentPositions,
			currentAuthor:  agentName,
			assistantMsgID: generateID(),
			loopIterations: map[string]int{},
			nodeStartedAt:  time.Now(),
		}

		// Build run options. WithStateDelta (ADK-Go v1.0.0) injects per-invocation
		// state at run start using the "temp:" prefix so it is never persisted or
		// surfaced as a StateChangeReported event (translateStateDelta filters temp: keys).
		var runOpts []adkrunner.RunOption
		if req.InvocationID != "" {
			runOpts = append(runOpts, adkrunner.WithStateDelta(map[string]any{
				"temp:invocation_id": req.InvocationID,
			}))
		}

		// SaveInputBlobsAsArtifacts only takes effect when the runner has an
		// artifact service wired; gate on artifactsEnabled for cleanliness.
		// See .READ/adk-go/runner/runner.go:270-309 — when true, inline blobs
		// on the user message get persisted as artifacts and replaced with
		// text placeholders the model reads as "Uploaded file: …".
		saveInputBlobs := artifactsEnabled && artAutoSaveBlobs
		streamingMode := r.resolveStreamingMode(agentCfg)
		for adkEvt, adkErr := range adkRunner.Run(ctx, req.UserID, req.SessionID, userMsg, agent.RunConfig{
			StreamingMode:             streamingMode,
			SaveInputBlobsAsArtifacts: saveInputBlobs,
		}, runOpts...) {
			if adkErr != nil {
				if !yieldAll(yield, loopState.onAdkError()) {
					return
				}
				if !yield(nil, adkErr) {
					return
				}
				continue
			}

			if isAgentTransition(adkEvt, loopState.currentAuthor) {
				if !yieldAll(yield, loopState.onTransition(adkEvt.Author)) {
					return
				}
			}

			for _, de := range TranslateEvent(adkEvt, loopState.evtCtx) {
				var events []thread.Event
				switch e := de.(type) {
				case *thread.AssistantContentStreamed:
					events = loopState.onAssistantContentStreamed(e)
				case *thread.AssistantMessageAdded:
					events = loopState.onAssistantMessageAdded(e)
				default:
					events = []thread.Event{de}
				}
				if !yieldAll(yield, events) {
					return
				}
			}
		}

		// Terminal cleanup: close out the final tracked node and any dangling
		// streaming message. yieldAll's cancel check is a no-op here — nothing
		// runs after this — but we keep it for consistency.
		_ = yieldAll(yield, loopState.onLoopEnd())

		// 16. Persist session to memory for future cross-session recall.
		if memoryEnabled {
			r.persistSessionToMemory(ctx, sessionSvc, req)
		}
	}
}

// injectRunContext covers step 12: wires the authenticated user into
// context so downstream tools that reach for auth.GetUserFromContext
// (DB-writing tools etc.) see the caller, and attaches the IO handler
// for tools that use io.Read() / io.Confirm(). Default is PauseHandler
// so io.Read() can emit TOOL_NEEDS_INPUT events; when resuming with
// custom user input, the CachedReadHandler short-circuits the pause.
func (r *RunnerAdapter) injectRunContext(ctx context.Context, req *fagent.RunRequest, toolCallID string) context.Context {
	if req.UserID != "" {
		ctx = context.WithValue(ctx, auth.UserContextKey, &auth.User{ID: req.UserID})
	}
	ctx = agentio.WithHandler(ctx, agentio.NewPauseHandler())
	if toolCallID != "" {
		if userInput, _ := req.Context["userInput"].(string); userInput != "" {
			ctx = agentio.WithHandler(ctx, agentio.NewCachedReadHandler(userInput))
		}
	}
	return ctx
}

// buildRunUserMessage covers step 14: pick between the confirmation-response
// shape (when resuming a HITL tool call) and the plain user-message shape.
// On resume, the frontend sends the original tool-call ID but ADK expects
// the confirmation-wrapper ID — look it up from thread history, falling
// back to the original ID if the wrapper can't be found (shouldn't happen
// in a healthy flow, but fails safely).
func buildRunUserMessage(req *fagent.RunRequest, agg *thread.ThreadAggregate, toolCallID string) *genai.Content {
	if toolCallID == "" {
		return buildUserContent(req.Message)
	}
	wrapperID := GetConfirmationWrapperID(agg.Messages, toolCallID)
	if wrapperID == "" {
		wrapperID = toolCallID
	}
	return buildConfirmationResponse(wrapperID, toolCallID)
}

// buildRegularLLMAgent covers the non-workflow branch of Run: resolve
// tools + memory/artifact/agent-tool injections, assemble the system
// prompt, wire logging + enforcement + confirmation callbacks, build any
// coordinator sub-agents, and finally build the top-level LLM agent.
// Error is already wrapped for the yield caller.
func (r *RunnerAdapter) buildRegularLLMAgent(
	ctx context.Context,
	req *fagent.RunRequest,
	agentName string,
	foundAgent fagent.Agent,
	agentCfg *fagent.Config,
	modelCfg *fagent.ModelConfig,
	llm adkmodel.LLM,
	inv *fagent.Invocation,
	memoryEnabled, memPreload, artifactsEnabled bool,
) (agent.Agent, error) {
	// Build tools from agent config + enabled tool sets + MCP.
	adkTools := r.resolveTools(ctx, req.UserID, agentName, modelCfg, req.SessionID, agentCfg, req)

	// Inject ADK-native memory tools when the agent's MemoryPolicy says
	// so. load_memory is injected whenever memory is enabled; preload_memory
	// is only injected when Preload is requested — ADK's two-tool model.
	if memoryEnabled {
		adkTools = append(adkTools, loadmemorytool.New())
		if memPreload {
			adkTools = append(adkTools, preloadmemorytool.New())
		}
	}
	if artifactsEnabled {
		adkTools = append(adkTools, loadartifactstool.New())
	}

	// Wrap each AgentToolRef so the parent can call another user-owned
	// agent as a plain function.
	adkTools = append(adkTools, r.resolveAgentTools(ctx, req.UserID, modelCfg, req.SessionID, agentCfg, req)...)

	systemPrompt := r.buildSystemPrompt(ctx, foundAgent, req.UserID, agentName, req.SessionID, agentCfg, inv)

	var agentModelCfg *fagent.AgentModelConfig
	if agentCfg != nil {
		agentModelCfg = agentCfg.ModelConfig
	}
	genCfg := buildGenerateContentConfig(modelCfg, agentModelCfg, req.Config)
	// Built-in code execution — provider-dispatched. See applyCodeExecutionTool
	// for the Gemini/OpenAI/Anthropic dispatch table.
	r.applyCodeExecutionTool(agentCfg, modelCfg, genCfg)

	beforeModelCBs, afterModelCBs, beforeToolCBs, afterToolCBs, onToolErrorCBs := r.buildAgentCallbacks(agentName, req)

	adkSubAgents, err := r.buildCoordinatorSubAgents(agentCfg, llm, genCfg)
	if err != nil {
		return nil, err
	}

	adkAgent, aErr := buildLLMAgent(llmAgentParams{
		name:                 agentName,
		foundAgent:           foundAgent,
		agentCfg:             agentCfg,
		llm:                  llm,
		genCfg:               genCfg,
		tools:                adkTools,
		prompt:               systemPrompt,
		beforeModelCallbacks: beforeModelCBs,
		afterModelCallbacks:  afterModelCBs,
		beforeToolCallbacks:  beforeToolCBs,
		afterToolCallbacks:   afterToolCBs,
		onToolErrorCallbacks: onToolErrorCBs,
		subAgents:            adkSubAgents,
	})
	if aErr != nil {
		return nil, fmt.Errorf("creating adk-go agent: %w", aErr)
	}
	return adkAgent, nil
}

// buildAgentCallbacks wires logging, block enforcement, and confirmation
// handlers into ADK's native Before/After callback slots.
func (r *RunnerAdapter) buildAgentCallbacks(agentName string, req *fagent.RunRequest) (
	beforeModel []llmagent.BeforeModelCallback,
	afterModel []llmagent.AfterModelCallback,
	beforeTool []llmagent.BeforeToolCallback,
	afterTool []llmagent.AfterToolCallback,
	onToolError []llmagent.OnToolErrorCallback,
) {
	if r.log != nil {
		logBefore, logAfter := toolLoggingCallbacks(r.log, agentName, req.SessionID, req.UserID)
		beforeTool = append(beforeTool, logBefore)
		afterTool = append(afterTool, logAfter)
		// OnToolErrorCallback handles ErrConfirmationRequired/ErrConfirmationRejected
		// (ADK-Go v1.0.0 sentinels) and logs unexpected native tool errors.
		onToolError = append(onToolError, toolErrorCallback(r.log, agentName, req.SessionID))
		modelBefore, modelAfter := modelLoggingCallbacks(r.log, agentName, req.SessionID)
		beforeModel = append(beforeModel, modelBefore)
		afterModel = append(afterModel, modelAfter)
	}

	// Block enforcement — before-only foundation middleware converted to
	// a native BeforeToolCallback.
	var enforcerMWs []fagent.Middleware
	if r.blockEnforcer != nil {
		enforcerMWs = append(enforcerMWs, r.blockEnforcer.BlockMiddleware(agentName))
	}
	beforeTool = append(beforeTool, buildBeforeOnlyCallbacks(agentName, req.SessionID, req.UserID, enforcerMWs...)...)

	// Confirmation enforcement — ADK-native HITL protocol. Uses
	// ctx.RequestConfirmation() instead of legacy io.Confirm().
	if r.confirmChecker != nil {
		beforeTool = append(beforeTool, buildConfirmationCallback(r.confirmChecker, agentName, req.UserID, ""))
	}
	return beforeModel, afterModel, beforeTool, afterTool, onToolError
}

// buildCoordinatorSubAgents builds ADK-Go sub-agents for the coordinator
// pattern (LLM agent whose SubAgents are populated from agentCfg.SubAgents).
// Non-Configurable sub-agents are skipped — they can still be invoked via
// the agent-tool path.
func (r *RunnerAdapter) buildCoordinatorSubAgents(
	agentCfg *fagent.Config,
	llm adkmodel.LLM,
	genCfg *genai.GenerateContentConfig,
) ([]agent.Agent, error) {
	if agentCfg == nil || len(agentCfg.SubAgents) == 0 {
		return nil, nil
	}
	var subs []agent.Agent
	for _, subA := range agentCfg.SubAgents {
		cfgurable, ok := subA.(fagent.Configurable)
		if !ok {
			continue
		}
		subCfg := cfgurable.Config()
		subAdk, sErr := buildLLMAgent(llmAgentParams{
			name:       subCfg.Name,
			foundAgent: subA,
			agentCfg:   extractAgentConfig(subA),
			llm:        llm,
			genCfg:     genCfg,
			tools:      nil,
			prompt:     subCfg.SystemPrompt,
		})
		if sErr != nil {
			return nil, fmt.Errorf("building coordinator sub-agent %q: %w", subCfg.Name, sErr)
		}
		subs = append(subs, subAdk)
	}
	return subs, nil
}

// loadOrSeedThread covers step 1 of Run: inspect req.Context for a
// pending tool-call ID and attachment refs, then either (a) reload the
// existing thread aggregate (confirmation/elicitation resume path), (b)
// append the user's message and return its side-effects (normal path),
// or (c) load an optional existing thread (no message, no tool-call).
// Returns the aggregate, the extracted toolCallID (empty when not a
// resume), the events that should be yielded before the agent runs, and
// any persistence error.
func (r *RunnerAdapter) loadOrSeedThread(ctx context.Context, req *fagent.RunRequest) (*thread.ThreadAggregate, string, []thread.Event, error) {
	var toolCallID string
	var attachmentRefs []thread.Attachment
	if req.Context != nil {
		if refs, ok := req.Context["attachmentRefs"].([]thread.Attachment); ok {
			attachmentRefs = refs
		}
		if id, ok := req.Context["toolCallId"].(string); ok {
			toolCallID = id
		}
	}

	if toolCallID != "" {
		// Confirmation/elicitation resume: the user's response is already
		// captured in the InputProvided event (emitted by the orchestrator
		// before calling Run). Skip persisting a duplicate UserMessageAdded
		// to avoid polluting the session with a plain-text user event that
		// would cause ADK's RequestConfirmationRequestProcessor to bail.
		agg, lErr := r.threadMgr.Load(ctx, req.SessionID)
		if lErr != nil {
			return nil, "", nil, fmt.Errorf("failed to load thread for tool response: %w", lErr)
		}
		return agg, toolCallID, nil, nil
	}

	if req.Message != nil {
		messageID := req.Message.ID
		if messageID == "" {
			messageID = generateID()
		}
		userContent := extractUserText(req.Message)
		agg, uEvent, sideEffects, pErr := r.threadMgr.AddUserMessage(ctx, req.SessionID, messageID, userContent, req.UserID, attachmentRefs...)
		if pErr != nil {
			return nil, "", nil, fmt.Errorf("failed to persist user message: %w", pErr)
		}
		var initial []thread.Event
		if uEvent != nil {
			initial = append(initial, uEvent)
		}
		initial = append(initial, sideEffects...)
		return agg, "", initial, nil
	}

	agg, lErr := r.threadMgr.Load(ctx, req.SessionID)
	if lErr != nil && !errors.Is(lErr, thread.ErrThreadNotFound) {
		return nil, "", nil, fmt.Errorf("failed to load thread: %w", lErr)
	}
	return agg, "", nil, nil
}

// persistSessionToMemory indexes the just-finished session for keyword
// search so the preload_memory tool can retrieve relevant context in
// future runs. Best-effort: any failure is logged, never returned.
func (r *RunnerAdapter) persistSessionToMemory(ctx context.Context, sessionSvc session.Service, req *fagent.RunRequest) {
	getResp, getErr := sessionSvc.Get(ctx, &session.GetRequest{
		AppName:   appName,
		UserID:    req.UserID,
		SessionID: req.SessionID,
	})
	if getErr != nil || getResp == nil {
		return
	}
	if addErr := r.memoryService.AddSessionToMemory(ctx, getResp.Session); addErr != nil && r.log != nil {
		r.log.Error(ctx, "failed to persist session to memory",
			"error", addErr, "userID", req.UserID, "sessionID", req.SessionID)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

// resolveMemoryPolicy returns (enabled, preload) for this agent from its
// per-agent Config.MemoryPolicy. Nil policy means memory is off — the
// legacy `memory-set` toolset-permission fallback has been removed; users
// configure memory exclusively through the designer's Memory card.
func (r *RunnerAdapter) resolveMemoryPolicy(agentCfg *fagent.Config) (enabled bool, preload bool) {
	if agentCfg == nil || agentCfg.MemoryPolicy == nil {
		return false, false
	}
	return agentCfg.MemoryPolicy.Enabled, agentCfg.MemoryPolicy.Preload
}

// applyCodeExecutionTool appends the provider-specific built-in code
// execution tool onto genCfg.Tools when the agent's policy is enabled
// and the active provider has native code-interpreter support.
//
// Why genCfg.Tools and not the adk tool.Tool list: Gemini's code executor
// is enabled by adding a bare `&genai.Tool{CodeExecution: ...}` entry to
// the request's Tools slice — it's a capability flag, not a function the
// model calls. The ADK flow clones the agent's GenerateContentConfig
// (including Tools) into every request, so pre-populating it here is the
// simplest provider-aware wiring. Function tools from the agent's Tool
// list get appended alongside in ADK's tools processor.
//
// Dispatch table:
//
//   - google (Gemini): ships. Adds &genai.Tool{CodeExecution: &genai.ToolCodeExecution{}}.
//     Google runs the code inside its own sandbox (no execution on our
//     infra) and returns ExecutableCode + CodeExecutionResult parts,
//     which events.go normalises to CodeExecutionRan domain events.
//   - openai:           not wired. When we're ready, add OpenAI's
//     Responses-API code-interpreter tool via an
//     equivalent entry in the OpenAI adapter layer.
//     Policy shape doesn't change.
//   - anthropic:        not wired. Same pattern — Claude's
//     code_execution tool will plug in here.
//
// Deliberately tolerant: nil policy, nil modelCfg, nil genCfg, or an
// unrecognised provider all leave genCfg untouched. The agent still
// runs with its other tools intact.
func (r *RunnerAdapter) applyCodeExecutionTool(agentCfg *fagent.Config, modelCfg *fagent.ModelConfig, genCfg *genai.GenerateContentConfig) {
	if agentCfg == nil || agentCfg.CodeExecutionPolicy == nil || !agentCfg.CodeExecutionPolicy.Enabled {
		return
	}
	if modelCfg == nil || genCfg == nil {
		return
	}
	switch strings.ToLower(modelCfg.Provider) {
	case "google", "gemini", "vertex", "vertexai":
		genCfg.Tools = append(genCfg.Tools, &genai.Tool{
			CodeExecution: &genai.ToolCodeExecution{},
		})
	case "openai":
		// The OpenAI Responses-API adapter looks for this same
		// genai.Tool.CodeExecution marker on the request and maps it
		// to &responses.ToolCodeInterpreterParam{Container: "auto"}
		// (see openai_responses.go:toolsToResponsesParam). One
		// provider-agnostic signal in, two provider-native wirings
		// out.
		genCfg.Tools = append(genCfg.Tools, &genai.Tool{
			CodeExecution: &genai.ToolCodeExecution{},
		})
	default:
		// Anthropic's OpenAI-compatible endpoint is on Chat Completions
		// and doesn't support code interpreter. When we add a native
		// Anthropic adapter (with Claude's code_execution tool), add a
		// branch here — the foundation AgentCodeExecutionPolicy shape
		// doesn't need to change.
	}
}

// resolveArtifactPolicy returns (enabled, autoSaveUserBlobs) for this agent
// from its per-agent Config.ArtifactPolicy. Nil policy means artifacts are
// off — the legacy `artifacts-set` toolset-permission fallback has been
// removed; users configure artifacts exclusively through the designer.
func (r *RunnerAdapter) resolveArtifactPolicy(agentCfg *fagent.Config) (enabled bool, autoSaveUserBlobs bool) {
	if agentCfg == nil || agentCfg.ArtifactPolicy == nil {
		return false, false
	}
	return agentCfg.ArtifactPolicy.Enabled, agentCfg.ArtifactPolicy.AutoSaveUserBlobs
}

// resolveStreamingMode returns the ADK RunConfig.StreamingMode value for
// this agent. Nil policy means stream — matches the adapter's historical
// default. An explicit policy with Enabled=false flips the run to
// StreamingModeNone so ADK delivers the full response in a single frame.
//
// See .READ/adk-go/agent/run_config.go for the mode enum.
func (r *RunnerAdapter) resolveStreamingMode(agentCfg *fagent.Config) agent.StreamingMode {
	if agentCfg != nil && agentCfg.StreamingPolicy != nil && !agentCfg.StreamingPolicy.Enabled {
		return agent.StreamingModeNone
	}
	return agent.StreamingModeSSE
}

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
//  2. Permission-gated tool sets
//  3. MCP-connected tools
//  4. A2A-connected tools
func (r *RunnerAdapter) resolveTools(
	ctx context.Context,
	userID, agentName string,
	modelCfg *fagent.ModelConfig,
	threadID string,
	agentCfg *fagent.Config,
	req *fagent.RunRequest,
) []adktool.Tool {
	var execTools []fagent.ExecutableTool

	// 1. Collect agent-level static tools from Config.
	if agentCfg != nil {
		for _, t := range agentCfg.Tools {
			if et, ok := t.(fagent.ExecutableTool); ok {
				execTools = append(execTools, et)
			}
		}
	}

	// 2. Build per-request tool factory.
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

	// 3. Get enabled tool sets via the tool requirements service.
	if r.toolReqs != nil && toolFactory != nil {
		enabledToolSetIDs, err := r.toolReqs.GetEnabledToolSets(ctx, userID, agentName)
		if err != nil {
			enabledToolSetIDs = []string{}
		}

		for _, toolSetID := range enabledToolSetIDs {
			if strings.HasPrefix(toolSetID, "mcp-") {
				continue // MCP handled below.
			}
			if strings.HasPrefix(toolSetID, "a2a-") {
				continue // A2A handled below.
			}
			tools, tErr := toolFactory.CreateExecutableToolsFromSet(ctx, toolSetID)
			if tErr != nil {
				continue
			}
			// Filter out individual tools blocked by per-tool permissions.
			// Without this, blocking a single tool (e.g. "add") inside an
			// enabled tool set (e.g. "calculator-set") has no effect—the tool
			// still gets sent to the LLM and executed.
			for _, t := range tools {
				if r.toolReqs.HasPermission(ctx, userID, agentName, toolSetID, t.Name()) {
					execTools = append(execTools, t)
				}
			}
		}

		// Resolve MCP tools.
		if r.mcpResolver != nil && userID != "" {
			mcpTools := r.mcpResolver.ResolveEnabledTools(ctx, userID,
				func(toolSetID string) bool {
					return r.toolReqs.HasToolSetPermission(ctx, userID, agentName, toolSetID)
				},
				func(toolSetID, toolID string) bool {
					return r.toolReqs.HasPermission(ctx, userID, agentName, toolSetID, toolID)
				},
			)
			execTools = append(execTools, mcpTools...)
		}

		// Resolve A2A tools.
		if r.a2aResolver != nil && userID != "" {
			a2aTools := r.a2aResolver.ResolveEnabledTools(ctx, userID,
				func(toolSetID string) bool {
					return r.toolReqs.HasToolSetPermission(ctx, userID, agentName, toolSetID)
				},
				func(toolSetID, toolID string) bool {
					return r.toolReqs.HasPermission(ctx, userID, agentName, toolSetID, toolID)
				},
			)
			execTools = append(execTools, a2aTools...)
		}
	}

	// Wrap foundation tools as adk-go tools.
	return WrapExecutableTools(execTools, threadID, userID)
}

// buildSystemPrompt assembles the system prompt from agent config and injectors.
// It includes:
//   - Static SystemPrompt from agent config
//   - InstructionFunc (runtime-generated prompt additions)
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

// buildConfirmationResponse creates a genai.Content with a FunctionResponse
// for the adk_request_confirmation protocol. This is sent to adkRunner.Run()
// instead of plain text when resuming from a tool confirmation pause.
//
// The wrapperID is the adk_request_confirmation FunctionCall ID (used by ADK internally).
// The originalToolCallID is the original tool call ID (used for logging/debugging).
func buildConfirmationResponse(wrapperID, originalToolCallID string) *genai.Content {
	return &genai.Content{
		Role: "user",
		Parts: []*genai.Part{{
			FunctionResponse: &genai.FunctionResponse{
				Name: toolconfirmation.FunctionCallName, // = "adk_request_confirmation"
				ID:   wrapperID,                         // ADK uses this to match pending confirmation
				Response: map[string]any{
					"confirmed": true,
					// Include original tool call ID for traceability
					"original_tool_call_id": originalToolCallID,
				},
			},
		}},
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

// extractAgentConfig extracts the foundation agent.Config from an Agent.
// Uses the agent.Configurable interface for clean extraction.
func extractAgentConfig(a fagent.Agent) *fagent.Config {
	if ca, ok := a.(fagent.Configurable); ok {
		cfg := ca.Config()
		return &cfg
	}
	return nil
}

// applyModelOverride returns a new ModelConfig with provider/model/apiKey replaced
// by the per-agent override. Generation parameters (Temperature, MaxTokens, etc.)
// are carried over from the user's global config so Gap-1 overrides still apply.
func (r *RunnerAdapter) applyModelOverride(ctx context.Context, userID string, base *fagent.ModelConfig, ov *fagent.AgentModelOverride) (*fagent.ModelConfig, error) {
	if r.apiKeyResolver == nil {
		return nil, fmt.Errorf("agent %q has a model override for provider %q but no API key resolver is configured", ov.ModelName, ov.Provider)
	}
	apiKey, err := r.apiKeyResolver.GetAPIKeyForProvider(ctx, userID, ov.Provider)
	if err != nil {
		return nil, fmt.Errorf("resolving API key for model override (provider %q): %w", ov.Provider, err)
	}
	return &fagent.ModelConfig{
		Provider:      ov.Provider,
		Model:         ov.ModelName,
		APIKey:        apiKey,
		Temperature:   base.Temperature,
		MaxTokens:     base.MaxTokens,
		MaxIterations: base.MaxIterations,
	}, nil
}
