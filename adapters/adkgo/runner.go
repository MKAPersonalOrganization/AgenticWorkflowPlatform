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
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/plugin"
	"google.golang.org/adk/plugin/retryandreflect"
	adkrunner "google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/loadartifactstool"
	"google.golang.org/adk/tool/loadmemorytool"
	"google.golang.org/adk/tool/preloadmemorytool"

	fagent "school-with-ai/foundation/agent"
	"school-with-ai/foundation/logger"
	"school-with-ai/modules/chat/thread"
	"school-with-ai/plugins/agentmemory"
	"school-with-ai/plugins/artifacts"
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
		memoryEnabled := r.memoryService != nil && r.hasMemoryEnabled(ctx, req.UserID, agentName)
		artifactsEnabled := r.artifactService != nil && r.hasArtifactsEnabled(ctx, req.UserID, agentName)

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
			// 5. Evaluate requirements once and reuse rules for tool filtering
			// and LLM guidance injection.
			precomputedRules := evaluateRequirements(ctx, agentCfg, inv)

			// 6. Build tools from agent config + enabled tool sets + MCP.
			adkTools := r.resolveTools(ctx, req.UserID, agentName, modelCfg, req.SessionID, agentCfg, req, precomputedRules)

			// 6b. Inject ADK-native tools gated by plugin permissions.
			if memoryEnabled {
				adkTools = append(adkTools, preloadmemorytool.New(), loadmemorytool.New())
			}
			if artifactsEnabled {
				adkTools = append(adkTools, loadartifactstool.New())
			}

			// 7. Build system prompt (agent config + requirements + workspace KB + skills + profile).
			systemPrompt := r.buildSystemPrompt(ctx, foundAgent, req.UserID, agentName, req.SessionID, agentCfg, inv)

			var agentModelCfg *fagent.AgentModelConfig
			if agentCfg != nil {
				agentModelCfg = agentCfg.ModelConfig
			}
			genCfg := buildGenerateContentConfig(modelCfg, agentModelCfg, req.Config)

			// Build native ADK tool and model callbacks: logging + enforcement.
			var beforeModelCBs []llmagent.BeforeModelCallback
			var afterModelCBs []llmagent.AfterModelCallback
			var beforeToolCBs []llmagent.BeforeToolCallback
			var afterToolCBs []llmagent.AfterToolCallback

			// 1. Tool + model logging — native Before/After callbacks.
			var onToolErrorCBs []llmagent.OnToolErrorCallback
			if r.log != nil {
				logBefore, logAfter := toolLoggingCallbacks(r.log, agentName, req.SessionID, req.UserID)
				beforeToolCBs = append(beforeToolCBs, logBefore)
				afterToolCBs = append(afterToolCBs, logAfter)
				// OnToolErrorCallback handles ErrConfirmationRequired/ErrConfirmationRejected
				// (ADK-Go v1.0.0 sentinels) and logs unexpected native tool errors.
				onToolErrorCBs = append(onToolErrorCBs, toolErrorCallback(r.log, agentName, req.SessionID))
				// Model logging — records timing and token usage at the model level.
				modelBefore, modelAfter := modelLoggingCallbacks(r.log, agentName, req.SessionID)
				beforeModelCBs = append(beforeModelCBs, modelBefore)
				afterModelCBs = append(afterModelCBs, modelAfter)
			}

			// 2. Block enforcement — before-only foundation
			//    middleware converted to a native BeforeToolCallback.
			var enforcerMWs []fagent.Middleware
			if r.blockEnforcer != nil {
				enforcerMWs = append(enforcerMWs, r.blockEnforcer.BlockMiddleware(agentName))
			}
			beforeToolCBs = append(beforeToolCBs, buildBeforeOnlyCallbacks(agentName, req.SessionID, req.UserID, enforcerMWs...)...)

			// 2b. Confirmation enforcement — ADK-native HITL protocol.
			//    Uses ctx.RequestConfirmation() instead of legacy io.Confirm().
			if r.confirmChecker != nil {
				beforeToolCBs = append(beforeToolCBs, buildConfirmationCallback(r.confirmChecker, agentName, req.UserID, ""))
			}

			var aErr error
			adkAgent, aErr = buildLLMAgent(llmAgentParams{
				name:                 agentName,
				foundAgent:           foundAgent,
				agentCfg:             agentCfg,
				llm:                  llm,
				genCfg:               genCfg,
				tools:                adkTools,
				prompt:               systemPrompt,
				beforeAgentCallbacks: buildBeforeRunHooks(precomputedRules),
				beforeModelCallbacks: beforeModelCBs,
				afterModelCallbacks:  afterModelCBs,
				beforeToolCallbacks:  beforeToolCBs,
				afterToolCallbacks:   afterToolCBs,
				onToolErrorCallbacks: onToolErrorCBs,
			})
			if aErr != nil {
				yield(nil, fmt.Errorf("creating adk-go agent: %w", aErr))
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

		// Track the current active agent so we can detect transitions.
		// ADK-Go sets session.Event.Author to the name of the agent that
		// produced the event. In workflow execution, this changes as control
		// passes between sub-agents (e.g., from "researcher" to "fact-checker"
		// in a sequential pipeline). We emit AgentTransferStarted when the
		// author changes and update evtCtx.AgentName so all subsequent domain
		// events carry the correct agent attribution.
		currentAuthor := agentName

		// Track loop iterations so we can report which iteration the UI
		// should display. Key = parent loop agent name, value = current iteration.
		loopIterations := map[string]int{}

		// Track when the current workflow node started so we can report duration
		// in WorkflowNodeCompleted events. Reset on every agent transition.
		nodeStartedAt := time.Now()

		// Build run options. WithStateDelta (ADK-Go v1.0.0) injects per-invocation
		// state at run start using the "temp:" prefix so it is never persisted or
		// surfaced as a StateChangeReported event (translateStateDelta filters temp: keys).
		var runOpts []adkrunner.RunOption
		if req.InvocationID != "" {
			runOpts = append(runOpts, adkrunner.WithStateDelta(map[string]any{
				"temp:invocation_id": req.InvocationID,
			}))
		}

		for adkEvt, adkErr := range adkRunner.Run(ctx, req.UserID, req.SessionID, userMsg, agent.RunConfig{
			StreamingMode: agent.StreamingModeSSE,
		}, runOpts...) {
			if adkErr != nil {
				// Emit node failure for the currently-active workflow sub-agent.
				if _, isWorkflowNode := agentPositions[currentAuthor]; isWorkflowNode {
					if !yield(&thread.WorkflowNodeCompleted{
						ThreadID:   evtCtx.ThreadID,
						AgentName:  currentAuthor,
						DurationMs: time.Since(nodeStartedAt).Milliseconds(),
						Outcome:    "error",
						Timestamp:  time.Now(),
					}, nil) {
						return
					}
					nodeStartedAt = time.Now()
				}
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

			// ── Agent transition detection ──
			// ADK-Go sets Author to the name of the sub-agent that produced
			// the event. When it differs from the last known author, we know
			// control has transferred to a different agent in the workflow.
			// We skip "user" authors (those are user-originated function
			// responses, not agent transitions).
			if adkEvt.Author != "" && adkEvt.Author != "user" && adkEvt.Author != currentAuthor {
				// Close any in-progress streaming message before the transfer
				// so the UI properly ends the previous agent's message.
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

				// Always generate a fresh message ID for the new agent so
				// each agent's response is a separate message bubble in
				// the UI — even if the previous agent's stream already
				// completed naturally before the transfer event arrived.
				assistantMsgID = generateID()

				fromAgent := currentAuthor
				currentAuthor = adkEvt.Author
				evtCtx.AgentName = currentAuthor

				// Build the transfer event, enriched with workflow position
				// context when available.
				transferEvt := &thread.AgentTransferStarted{
					ThreadID:  evtCtx.ThreadID,
					FromAgent: fromAgent,
					ToAgent:   currentAuthor,
					Timestamp: time.Now(),
				}
				if pos, ok := agentPositions[currentAuthor]; ok {
					transferEvt.WorkflowType = pos.WorkflowType
					transferEvt.ParentAgent = pos.ParentName
					transferEvt.StepIndex = pos.StepIndex
					transferEvt.StepCount = pos.StepCount
					transferEvt.MaxIterations = pos.MaxIterations

					// Track loop iterations: when we see stepIndex==0 again
					// within the same loop parent, a new iteration started.
					if pos.WorkflowType == "loop" {
						if _, seen := loopIterations[pos.ParentName]; !seen {
							loopIterations[pos.ParentName] = 1
						} else if pos.StepIndex == 0 {
							loopIterations[pos.ParentName]++
						}
						transferEvt.Iteration = loopIterations[pos.ParentName]
					}
				}

				// Emit WorkflowNodeCompleted for the agent that just handed off.
				// Only for tracked sub-agents (agentPositions excludes the root orchestrator).
				if _, isWorkflowNode := agentPositions[fromAgent]; isWorkflowNode {
					if !yield(&thread.WorkflowNodeCompleted{
						ThreadID:   evtCtx.ThreadID,
						AgentName:  fromAgent,
						DurationMs: time.Since(nodeStartedAt).Milliseconds(),
						Outcome:    "success",
						Timestamp:  time.Now(),
					}, nil) {
						return
					}
				}
				nodeStartedAt = time.Now()

				if !yield(transferEvt, nil) {
					return
				}
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
					// Ensure a stable message ID — A2A-sourced events may arrive
					// with an empty session.Event.ID, which would fail persistence.
					if evt.MessageID == "" {
						evt.MessageID = assistantMsgID
					}
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
					} else if evt.Content != "" {
						// Non-streaming response (e.g. A2A remote agent): no
						// AssistantContentStreamed deltas were emitted, so the
						// frontend has nothing to show. Synthesize the full
						// TEXT_MESSAGE lifecycle as transient events.
						if !yield(&thread.AssistantMessageStarted{
							ThreadID:  evt.ThreadID,
							MessageID: evt.MessageID,
							Timestamp: evt.Timestamp,
						}, nil) {
							return
						}
						if !yield(&thread.AssistantContentStreamed{
							ThreadID:  evt.ThreadID,
							MessageID: evt.MessageID,
							Delta:     evt.Content,
							Timestamp: evt.Timestamp,
						}, nil) {
							return
						}
						if !yield(&thread.AssistantMessageCompleted{
							ThreadID:  evt.ThreadID,
							MessageID: evt.MessageID,
							Content:   evt.Content,
							AgentName: evt.AgentName,
							Timestamp: evt.Timestamp,
						}, nil) {
							return
						}
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

		// Emit WorkflowNodeCompleted for the last agent if it's a workflow sub-agent.
		if _, isWorkflowNode := agentPositions[currentAuthor]; isWorkflowNode {
			yield(&thread.WorkflowNodeCompleted{
				ThreadID:   evtCtx.ThreadID,
				AgentName:  currentAuthor,
				DurationMs: time.Since(nodeStartedAt).Milliseconds(),
				Outcome:    "success",
				Timestamp:  time.Now(),
			}, nil)
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

		// 16. Persist session to memory for future cross-session recall.
		// The session now contains all events from this run. Adding it to the
		// memory service indexes its content for keyword-based search, so the
		// preload_memory tool can retrieve relevant context in future runs.
		if memoryEnabled {
			getResp, getErr := sessionSvc.Get(ctx, &session.GetRequest{
				AppName:   appName,
				UserID:    req.UserID,
				SessionID: req.SessionID,
			})
			if getErr == nil && getResp != nil {
				if addErr := r.memoryService.AddSessionToMemory(ctx, getResp.Session); addErr != nil {
					if r.log != nil {
						r.log.Error(ctx, "failed to persist session to memory", "error", addErr, "userID", req.UserID, "sessionID", req.SessionID)
					}
				}
			}
		}
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────────────

// hasMemoryEnabled checks whether the agent memory plugin is enabled for the
// given user+agent via the tool requirements permission system.
func (r *RunnerAdapter) hasMemoryEnabled(ctx context.Context, userID, agentName string) bool {
	if r.toolReqs == nil {
		return false
	}
	return r.toolReqs.HasToolSetPermission(ctx, userID, agentName, agentmemory.ToolSetID)
}

// hasArtifactsEnabled checks whether the artifacts plugin is enabled for the
// given user+agent via the tool requirements permission system.
func (r *RunnerAdapter) hasArtifactsEnabled(ctx context.Context, userID, agentName string) bool {
	if r.toolReqs == nil {
		return false
	}
	return r.toolReqs.HasToolSetPermission(ctx, userID, agentName, artifacts.ToolSetID)
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

	// 4. Get enabled tool sets via the tool requirements service.
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
