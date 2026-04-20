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

// runPrep is the assembled output of prepareRun — everything the streaming
// loop needs. Exists so Run's closure body stays flat.
type runPrep struct {
	toolCallID     string
	initialEvents  []thread.Event
	agg            *thread.ThreadAggregate
	modelCfg       *fagent.ModelConfig
	agentName      string
	agentPositions map[string]AgentPosition
	sessionSvc     session.Service
	adkRunner      *adkrunner.Runner
	userMsg        *genai.Content
	runOpts        []adkrunner.RunOption
	streamingMode  agent.StreamingMode
	saveInputBlobs bool
	memoryEnabled  bool
}

// Run executes an agent with the given request, yielding thread domain events.
// Implements agentorchestrator.Runner.
func (r *RunnerAdapter) Run(ctx context.Context, req *fagent.RunRequest) iter.Seq2[thread.Event, error] {
	return func(yield func(thread.Event, error) bool) {
		prep, err := r.prepareRun(ctx, req)
		if err != nil {
			yield(nil, err)
			return
		}
		ctx = r.injectRunContext(ctx, req, prep.toolCallID)
		for _, ie := range prep.initialEvents {
			if !yield(ie, nil) {
				return
			}
		}
		r.streamAdkEvents(ctx, yield, req, prep)
		if prep.memoryEnabled {
			r.persistSessionToMemory(ctx, prep.sessionSvc, req)
		}
	}
}

// prepareRun assembles every dependency the streaming loop needs: thread
// state, resolved agent, session + invocation, LLM, ADK agent, ADK runner,
// and the user message. All branching error paths live here so Run's
// closure can stay linear.
func (r *RunnerAdapter) prepareRun(ctx context.Context, req *fagent.RunRequest) (*runPrep, error) {
	modelCfg, err := r.configResolver.GetModelConfigForUser(ctx, req.UserID)
	if err != nil {
		return nil, fmt.Errorf("resolving AI settings for user: %w", err)
	}
	agg, toolCallID, initialEvents, err := r.loadOrSeedThread(ctx, req)
	if err != nil {
		return nil, err
	}
	agentName, foundAgent, agentCfg, err := r.resolveAgentForRun(ctx, req, agg)
	if err != nil {
		return nil, err
	}
	modelCfg, err = r.maybeApplyModelOverride(ctx, req.UserID, modelCfg, agentCfg)
	if err != nil {
		return nil, err
	}

	sessionSvc := session.InMemoryService()
	sess, err := r.buildSessionFromAggregate(ctx, sessionSvc, agg, req.UserID)
	if err != nil {
		return nil, err
	}
	inv := buildInvocation(req, sess, agg)

	llm, err := NewModel(ctx, modelCfg.Provider, modelCfg.Model, modelCfg.APIKey)
	if err != nil {
		return nil, fmt.Errorf("creating LLM model: %w", err)
	}

	memEnabled, memPreload := r.resolveMemoryPolicy(agentCfg)
	memoryEnabled := r.memoryService != nil && memEnabled
	artEnabled, artAutoSaveBlobs := r.resolveArtifactPolicy(agentCfg)
	artifactsEnabled := r.artifactService != nil && artEnabled

	adkAgent, agentPositions, err := r.buildAdkAgent(
		ctx, req, agentName, foundAgent, agentCfg, modelCfg, llm, inv,
		memoryEnabled, memPreload, artifactsEnabled,
	)
	if err != nil {
		return nil, err
	}

	adkRunner, err := r.buildAdkRunner(adkAgent, sessionSvc, memoryEnabled, artifactsEnabled)
	if err != nil {
		return nil, err
	}

	return &runPrep{
		toolCallID:     toolCallID,
		initialEvents:  initialEvents,
		agg:            agg,
		modelCfg:       modelCfg,
		agentName:      agentName,
		agentPositions: agentPositions,
		sessionSvc:     sessionSvc,
		adkRunner:      adkRunner,
		userMsg:        buildRunUserMessage(req, agg, toolCallID),
		runOpts:        buildRunOptions(req),
		streamingMode:  r.resolveStreamingMode(agentCfg),
		saveInputBlobs: artifactsEnabled && artAutoSaveBlobs,
		memoryEnabled:  memoryEnabled,
	}, nil
}

// resolveAgentForRun picks the agent for this run. agentName falls back to
// the last agent that authored a message in the aggregate, then the
// provider looks up the agent and we cache its foundation config.
func (r *RunnerAdapter) resolveAgentForRun(
	ctx context.Context, req *fagent.RunRequest, agg *thread.ThreadAggregate,
) (string, fagent.Agent, *fagent.Config, error) {
	agentName := req.AgentName
	if agentName == "" && agg != nil {
		for i := len(agg.Messages) - 1; i >= 0; i-- {
			if agg.Messages[i].AgentName != "" {
				agentName = agg.Messages[i].AgentName
				break
			}
		}
	}
	foundAgent, err := r.agentProvider.GetForUser(ctx, agentName, req.UserID)
	if err != nil {
		return "", nil, nil, fmt.Errorf("resolving agent %q: %w", agentName, err)
	}
	return agentName, foundAgent, extractAgentConfig(foundAgent), nil
}

// maybeApplyModelOverride returns modelCfg unchanged when the agent has no
// ModelOverride; otherwise it swaps provider/model/apiKey using the per-agent
// override and resolves the new API key.
func (r *RunnerAdapter) maybeApplyModelOverride(
	ctx context.Context, userID string, modelCfg *fagent.ModelConfig, agentCfg *fagent.Config,
) (*fagent.ModelConfig, error) {
	if agentCfg == nil || agentCfg.ModelOverride == nil {
		return modelCfg, nil
	}
	return r.applyModelOverride(ctx, userID, modelCfg, agentCfg.ModelOverride)
}

// buildSessionFromAggregate reconstructs an in-memory ADK session from the
// prior thread so the LLM agent sees past context. When there is no history
// (fresh conversation), the runner's AutoCreateSession will build the
// session on first run.
//
// Session state is restored from ThreadAggregate.State during
// BuildSessionFromThread and later persisted via StateChangeReported events.
func (r *RunnerAdapter) buildSessionFromAggregate(
	ctx context.Context, sessionSvc session.Service, agg *thread.ThreadAggregate, userID string,
) (session.Session, error) {
	if agg == nil || len(agg.Messages) == 0 {
		return nil, nil
	}
	sess, err := BuildSessionFromThread(ctx, sessionSvc, agg, userID)
	if err != nil {
		return nil, fmt.Errorf("building session from thread: %w", err)
	}
	return sess, nil
}

// buildInvocation assembles the foundation Invocation for Requirements
// (history + live session state). State mirrors values accumulated by
// prior agents so per-agent Requirements see what OutputKey callbacks
// wrote.
func buildInvocation(req *fagent.RunRequest, sess session.Session, agg *thread.ThreadAggregate) *fagent.Invocation {
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
	return inv
}

// buildAdkAgent dispatches to the workflow or regular-LLM agent builder.
// Returns the ADK agent and any workflow agent-position map (nil for the
// regular path).
func (r *RunnerAdapter) buildAdkAgent(
	ctx context.Context, req *fagent.RunRequest, agentName string,
	foundAgent fagent.Agent, agentCfg *fagent.Config, modelCfg *fagent.ModelConfig,
	llm adkmodel.LLM, inv *fagent.Invocation,
	memoryEnabled, memPreload, artifactsEnabled bool,
) (agent.Agent, map[string]AgentPosition, error) {
	if wa, isWF := isWorkflowAgent(foundAgent); isWF {
		wbc := &workflowBuildContext{
			ctx: ctx, adapter: r, userID: req.UserID,
			modelCfg: modelCfg, llm: llm, threadID: req.SessionID, req: req,
		}
		adkAgent, positions, err := buildWorkflowAgent(wbc, wa)
		if err != nil {
			return nil, nil, fmt.Errorf("building workflow agent: %w", err)
		}
		return adkAgent, positions, nil
	}
	adkAgent, err := r.buildRegularLLMAgent(
		ctx, req, agentName, foundAgent, agentCfg, modelCfg, llm, inv,
		memoryEnabled, memPreload, artifactsEnabled,
	)
	if err != nil {
		return nil, nil, err
	}
	return adkAgent, nil, nil
}

// buildAdkRunner wires the RetryAndReflect plugin and optional
// memory/artifact services into a runner configured for this invocation.
//
// RetryAndReflect (ADK-Go v1.0.0) intercepts tool errors and injects a
// structured reflection prompt into the LLM context, guiding the model
// to diagnose and retry with corrected arguments. Up to 3 retries per
// invocation; on exhaustion the plugin tells the model to stop using
// the failing tool rather than returning a raw error.
func (r *RunnerAdapter) buildAdkRunner(
	adkAgent agent.Agent, sessionSvc session.Service, memoryEnabled, artifactsEnabled bool,
) (*adkrunner.Runner, error) {
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
	adkRunner, err := adkrunner.New(runnerCfg)
	if err != nil {
		return nil, fmt.Errorf("creating adk-go runner: %w", err)
	}
	return adkRunner, nil
}

// buildRunOptions builds the ADK RunOption slice for this invocation.
// WithStateDelta (ADK-Go v1.0.0) injects per-invocation state at run start
// using the "temp:" prefix so it is never persisted or surfaced as a
// StateChangeReported event (translateStateDelta filters temp: keys).
func buildRunOptions(req *fagent.RunRequest) []adkrunner.RunOption {
	if req.InvocationID == "" {
		return nil
	}
	return []adkrunner.RunOption{
		adkrunner.WithStateDelta(map[string]any{
			"temp:invocation_id": req.InvocationID,
		}),
	}
}

// streamAdkEvents runs the prepared ADK runner and yields translated
// thread events. Streaming-lifecycle state (current author, streaming /
// assistant message IDs, node start time, loop iteration counters) lives
// in runLoopState so the outer iterator stays flat.
//
// SaveInputBlobsAsArtifacts only takes effect when the runner has an
// artifact service wired; we gate on artifactsEnabled for cleanliness.
// See .READ/adk-go/runner/runner.go:270-309 — when true, inline blobs on
// the user message get persisted as artifacts and replaced with text
// placeholders the model reads as "Uploaded file: …".
func (r *RunnerAdapter) streamAdkEvents(
	ctx context.Context, yield func(thread.Event, error) bool,
	req *fagent.RunRequest, prep *runPrep,
) {
	loopState := &runLoopState{
		evtCtx: EventContext{
			ThreadID:  req.SessionID,
			RunID:     messageIDFromRequest(req),
			AgentName: prep.agentName,
			Provider:  prep.modelCfg.Provider,
			Model:     prep.modelCfg.Model,
		},
		agentPositions: prep.agentPositions,
		currentAuthor:  prep.agentName,
		assistantMsgID: generateID(),
		loopIterations: map[string]int{},
		nodeStartedAt:  time.Now(),
	}

	for adkEvt, adkErr := range prep.adkRunner.Run(ctx, req.UserID, req.SessionID, prep.userMsg, agent.RunConfig{
		StreamingMode:             prep.streamingMode,
		SaveInputBlobsAsArtifacts: prep.saveInputBlobs,
	}, prep.runOpts...) {
		if !forwardAdkEvent(yield, loopState, adkEvt, adkErr) {
			return
		}
	}

	// Terminal cleanup: close out the final tracked node and any dangling
	// streaming message. yieldAll's cancel check is a no-op here — nothing
	// runs after this — but we keep it for consistency.
	_ = yieldAll(yield, loopState.onLoopEnd())
}

// forwardAdkEvent handles one (event, err) pair from the ADK runner:
// emits any error-boundary cleanup events then the error itself, or
// translates the event into zero-or-more thread events and yields each.
// Returns false when the consumer cancelled iteration.
func forwardAdkEvent(
	yield func(thread.Event, error) bool, loopState *runLoopState,
	adkEvt *session.Event, adkErr error,
) bool {
	if adkErr != nil {
		if !yieldAll(yield, loopState.onAdkError()) {
			return false
		}
		return yield(nil, adkErr)
	}
	if isAgentTransition(adkEvt, loopState.currentAuthor) {
		if !yieldAll(yield, loopState.onTransition(adkEvt.Author)) {
			return false
		}
	}
	for _, de := range TranslateEvent(adkEvt, loopState.evtCtx) {
		if !yieldAll(yield, lifecycleEvents(loopState, de)) {
			return false
		}
	}
	return true
}

// lifecycleEvents routes a translated event through runLoopState's
// streaming-assistant bookkeeping; non-streaming events pass through
// unchanged.
func lifecycleEvents(loopState *runLoopState, de thread.Event) []thread.Event {
	switch e := de.(type) {
	case *thread.AssistantContentStreamed:
		return loopState.onAssistantContentStreamed(e)
	case *thread.AssistantMessageAdded:
		return loopState.onAssistantMessageAdded(e)
	default:
		return []thread.Event{de}
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
	toolCallID, attachmentRefs := extractThreadContext(req)
	if toolCallID != "" {
		return r.loadForConfirmationResume(ctx, req.SessionID, toolCallID)
	}
	if req.Message != nil {
		return r.seedNewUserMessage(ctx, req, attachmentRefs)
	}
	return r.loadExistingThread(ctx, req.SessionID)
}

// extractThreadContext pulls the tool-call ID and attachment refs out of
// the request's free-form Context map. Missing or wrong-typed entries
// produce zero values.
func extractThreadContext(req *fagent.RunRequest) (string, []thread.Attachment) {
	if req.Context == nil {
		return "", nil
	}
	var toolCallID string
	var attachmentRefs []thread.Attachment
	if refs, ok := req.Context["attachmentRefs"].([]thread.Attachment); ok {
		attachmentRefs = refs
	}
	if id, ok := req.Context["toolCallId"].(string); ok {
		toolCallID = id
	}
	return toolCallID, attachmentRefs
}

// loadForConfirmationResume covers the confirmation/elicitation resume
// branch: the user's response is already captured in the InputProvided
// event (emitted by the orchestrator before Run). Persisting a duplicate
// UserMessageAdded here would pollute the session with a plain-text user
// event that causes ADK's RequestConfirmationRequestProcessor to bail.
func (r *RunnerAdapter) loadForConfirmationResume(
	ctx context.Context, sessionID, toolCallID string,
) (*thread.ThreadAggregate, string, []thread.Event, error) {
	agg, err := r.threadMgr.Load(ctx, sessionID)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to load thread for tool response: %w", err)
	}
	return agg, toolCallID, nil, nil
}

// seedNewUserMessage persists the incoming user message and returns the
// aggregate plus the message-added event and any side-effects that must
// be yielded before the agent runs.
func (r *RunnerAdapter) seedNewUserMessage(
	ctx context.Context, req *fagent.RunRequest, attachmentRefs []thread.Attachment,
) (*thread.ThreadAggregate, string, []thread.Event, error) {
	messageID := req.Message.ID
	if messageID == "" {
		messageID = generateID()
	}
	userContent := extractUserText(req.Message)
	agg, uEvent, sideEffects, err := r.threadMgr.AddUserMessage(
		ctx, req.SessionID, messageID, userContent, req.UserID, attachmentRefs...,
	)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to persist user message: %w", err)
	}
	var initial []thread.Event
	if uEvent != nil {
		initial = append(initial, uEvent)
	}
	initial = append(initial, sideEffects...)
	return agg, "", initial, nil
}

// loadExistingThread covers the no-message, no-tool-call path: optionally
// hydrate an existing thread. ErrThreadNotFound is not an error — this is
// a fresh conversation.
func (r *RunnerAdapter) loadExistingThread(
	ctx context.Context, sessionID string,
) (*thread.ThreadAggregate, string, []thread.Event, error) {
	agg, err := r.threadMgr.Load(ctx, sessionID)
	if err != nil && !errors.Is(err, thread.ErrThreadNotFound) {
		return nil, "", nil, fmt.Errorf("failed to load thread: %w", err)
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
	execTools := collectStaticTools(agentCfg)
	toolFactory := r.buildToolFactory(ctx, userID, agentName, modelCfg)
	if r.toolReqs != nil && toolFactory != nil {
		execTools = append(execTools, r.resolveGenericToolSets(ctx, userID, agentName, toolFactory)...)
		execTools = append(execTools, r.resolveMCPTools(ctx, userID, agentName)...)
		execTools = append(execTools, r.resolveA2ATools(ctx, userID, agentName)...)
	}
	return WrapExecutableTools(execTools, threadID, userID)
}

// collectStaticTools pulls ExecutableTool instances off agentCfg.Tools —
// the agent-declared tools that don't need permission/tool-set resolution.
func collectStaticTools(agentCfg *fagent.Config) []fagent.ExecutableTool {
	if agentCfg == nil {
		return nil
	}
	var out []fagent.ExecutableTool
	for _, t := range agentCfg.Tools {
		if et, ok := t.(fagent.ExecutableTool); ok {
			out = append(out, et)
		}
	}
	return out
}

// buildToolFactory constructs the per-request ToolSetFactory used to
// instantiate tools from tool-set IDs. Nil when no factory has been
// registered on the adapter.
func (r *RunnerAdapter) buildToolFactory(
	ctx context.Context, userID, agentName string, modelCfg *fagent.ModelConfig,
) ToolSetFactory {
	if r.newToolSetFactory == nil {
		return nil
	}
	var pluginSettings map[string]map[string]string
	if r.pluginSettingsResolver != nil && userID != "" {
		pluginSettings, _ = r.pluginSettingsResolver.GetAllPluginSettingsForUser(ctx, userID)
	}
	var extra map[string]string
	if r.extraConfigFn != nil {
		extra = r.extraConfigFn(ctx, userID, agentName)
	}
	return r.newToolSetFactory(ToolFactoryConfig{
		ModelAPIKey:    modelCfg.APIKey,
		ModelID:        modelCfg.Model,
		PluginSettings: pluginSettings,
		Extra:          extra,
	})
}

// resolveGenericToolSets materialises the non-MCP, non-A2A tool sets for
// this agent, filtering out tools blocked by per-tool permissions. Without
// that filter, blocking a single tool inside an enabled tool set would
// have no effect — the tool would still reach the LLM.
func (r *RunnerAdapter) resolveGenericToolSets(
	ctx context.Context, userID, agentName string, toolFactory ToolSetFactory,
) []fagent.ExecutableTool {
	enabledToolSetIDs, err := r.toolReqs.GetEnabledToolSets(ctx, userID, agentName)
	if err != nil {
		return nil
	}
	var out []fagent.ExecutableTool
	for _, toolSetID := range enabledToolSetIDs {
		if strings.HasPrefix(toolSetID, "mcp-") || strings.HasPrefix(toolSetID, "a2a-") {
			continue // handled by resolveMCPTools / resolveA2ATools.
		}
		tools, tErr := toolFactory.CreateExecutableToolsFromSet(ctx, toolSetID)
		if tErr != nil {
			continue
		}
		for _, t := range tools {
			if r.toolReqs.HasPermission(ctx, userID, agentName, toolSetID, t.Name()) {
				out = append(out, t)
			}
		}
	}
	return out
}

// resolveMCPTools asks the MCP resolver for the user's enabled MCP tools,
// with tool-set / per-tool permission callbacks wired in.
func (r *RunnerAdapter) resolveMCPTools(
	ctx context.Context, userID, agentName string,
) []fagent.ExecutableTool {
	if r.mcpResolver == nil || userID == "" {
		return nil
	}
	return r.mcpResolver.ResolveEnabledTools(ctx, userID,
		func(toolSetID string) bool {
			return r.toolReqs.HasToolSetPermission(ctx, userID, agentName, toolSetID)
		},
		func(toolSetID, toolID string) bool {
			return r.toolReqs.HasPermission(ctx, userID, agentName, toolSetID, toolID)
		},
	)
}

// resolveA2ATools asks the A2A resolver for the user's enabled A2A tools,
// with the same permission callback shape as MCP.
func (r *RunnerAdapter) resolveA2ATools(
	ctx context.Context, userID, agentName string,
) []fagent.ExecutableTool {
	if r.a2aResolver == nil || userID == "" {
		return nil
	}
	return r.a2aResolver.ResolveEnabledTools(ctx, userID,
		func(toolSetID string) bool {
			return r.toolReqs.HasToolSetPermission(ctx, userID, agentName, toolSetID)
		},
		func(toolSetID, toolID string) bool {
			return r.toolReqs.HasPermission(ctx, userID, agentName, toolSetID, toolID)
		},
	)
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
	sp := resolveStaticPrompt(agentCfg, inv)
	sp += r.appendInjectorPrompts(ctx, agentName, threadID)
	sp += r.appendUserContext(ctx, userID)
	return sp
}

// resolveStaticPrompt returns the agent's base SystemPrompt with any
// InstructionFunc output appended. InstructionFunc errors and empty
// strings are ignored — the static prompt still wins.
func resolveStaticPrompt(agentCfg *fagent.Config, inv *fagent.Invocation) string {
	if agentCfg == nil {
		return ""
	}
	sp := agentCfg.SystemPrompt
	if agentCfg.InstructionFunc == nil || inv == nil {
		return sp
	}
	dynamic, err := agentCfg.InstructionFunc(inv)
	if err != nil || dynamic == "" {
		return sp
	}
	return sp + "\n\n" + dynamic
}

// appendInjectorPrompts collects prompt additions from PromptInjectors
// (knowledge bases, skills, etc.) and joins them onto the prompt with
// blank-line separators.
func (r *RunnerAdapter) appendInjectorPrompts(ctx context.Context, agentName, threadID string) string {
	var sb strings.Builder
	for _, injector := range r.promptInjectors {
		if addition := injector.BuildPromptAddition(ctx, agentName, threadID); addition != "" {
			sb.WriteString("\n\n")
			sb.WriteString(addition)
		}
	}
	return sb.String()
}

// appendUserContext renders the authenticated user's profile as a
// structured-Markdown "## User Context" block. Empty profile, missing
// provider, or a resolver error all produce an empty string.
func (r *RunnerAdapter) appendUserContext(ctx context.Context, userID string) string {
	if r.profileProvider == nil || userID == "" {
		return ""
	}
	profile, err := r.profileProvider.GetProfile(ctx, userID)
	if err != nil || len(profile) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n## User Context\n")
	for key, val := range profile {
		writeProfileField(&sb, key, val)
	}
	return sb.String()
}

// writeProfileField renders a single profile entry as a Markdown bullet
// when the value is a non-empty string or string slice. Other types and
// empty values are dropped.
func writeProfileField(sb *strings.Builder, key string, val any) {
	switch v := val.(type) {
	case string:
		if v != "" {
			fmt.Fprintf(sb, "- **%s**: %s\n", formatLabel(key), v)
		}
	case []string:
		if len(v) > 0 {
			fmt.Fprintf(sb, "- **%s**: %s\n", formatLabel(key), strings.Join(v, ", "))
		}
	}
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
