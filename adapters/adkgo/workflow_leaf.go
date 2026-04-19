package adkgo

import (
	"fmt"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	adkmodel "google.golang.org/adk/model"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/exitlooptool"
	"google.golang.org/adk/tool/loadartifactstool"
	"google.golang.org/adk/tool/loadmemorytool"
	"google.golang.org/adk/tool/preloadmemorytool"
	"google.golang.org/genai"

	fagent "aicompass/foundation/agent"
)

// buildLeafAgent resolves a single agent node into an ADK-Go LLM agent.
// It uses the same tool resolution, prompt building, and middleware pipeline
// as the regular single-agent path in Run().
//
// Transfer policies, output keys, and schemas are read from the agent's own
// config (stored on AgentDefinition). Composition nodes only carry structural
// information — agent-level config belongs on the agent definition itself.
//
// The function is a thin orchestrator over named helpers: each stage
// (context, tools, model, callbacks, position) is independently reviewable.
func buildLeafAgent(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo, isLastInLoop bool, parentCtx *agentPositionCtx) (agent.Agent, error) {
	if node.AgentID == "" {
		return nil, fmt.Errorf("node %q: agent node requires agent_id", node.ID)
	}

	leafCtx, err := resolveLeafContext(wbc, node)
	if err != nil {
		return nil, err
	}

	tools := collectLeafTools(wbc, node.AgentID, leafCtx, isLastInLoop)

	model, err := resolveLeafModel(wbc, leafCtx.agentCfg, wbc.req.Config)
	if err != nil {
		return nil, err
	}

	agentName := wbc.uniqueName(leafCtx.foundAgent.Name())
	recordLeafPosition(wbc, agentName, parentCtx)

	cbs := buildLeafCallbacks(wbc, agentName, node.AgentID, leafCtx.precomputedRules)

	prompt := wbc.adapter.buildSystemPrompt(
		wbc.ctx, leafCtx.foundAgent, wbc.userID, node.AgentID, wbc.threadID, leafCtx.agentCfg, leafCtx.invocation,
	)

	return buildLLMAgent(llmAgentParams{
		name:                 agentName,
		foundAgent:           leafCtx.foundAgent,
		agentCfg:             leafCtx.agentCfg,
		llm:                  model.llm,
		genCfg:               model.genCfg,
		tools:                tools,
		prompt:               prompt,
		beforeAgentCallbacks: cbs.beforeAgent,
		beforeModelCallbacks: cbs.beforeModel,
		afterModelCallbacks:  cbs.afterModel,
		beforeToolCallbacks:  cbs.beforeTool,
		afterToolCallbacks:   cbs.afterTool,
		onToolErrorCallbacks: cbs.onToolError,
	})
}

// leafContext bundles the per-leaf lookups that every downstream stage needs:
// the resolved foundation agent, its typed Config (may be nil), a built-up
// Invocation shell for requirements evaluation, and the evaluated rules.
// The rules are precomputed once and reused for both tool filtering and
// BeforeAgent guidance injection.
type leafContext struct {
	foundAgent       fagent.Agent
	agentCfg         *fagent.Config
	invocation       *fagent.Invocation
	precomputedRules []fagent.Rule
}

func resolveLeafContext(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo) (*leafContext, error) {
	foundAgent, err := wbc.adapter.agentProvider.GetForUser(wbc.ctx, node.AgentID, wbc.userID)
	if err != nil {
		return nil, fmt.Errorf("resolving agent %q: %w", node.AgentID, err)
	}
	agentCfg := extractAgentConfig(foundAgent)
	inv := &fagent.Invocation{
		ID:        wbc.req.InvocationID,
		SessionID: wbc.req.SessionID,
		UserID:    wbc.userID,
		Message:   wbc.req.Message,
		Config:    wbc.req.Config,
		Context:   wbc.req.Context,
	}
	return &leafContext{
		foundAgent:       foundAgent,
		agentCfg:         agentCfg,
		invocation:       inv,
		precomputedRules: evaluateRequirements(wbc.ctx, agentCfg, inv),
	}, nil
}

// collectLeafTools merges every tool source a leaf may expose:
//  1. Configured tools (via adapter.resolveTools, filtered by precomputed rules)
//  2. Memory tools — loadmemorytool / preloadmemorytool — gated by MemoryPolicy
//     (falling back to the legacy memory-set toolset permission for pre-M8 agents)
//  3. Artifact tools — loadartifactstool — gated by the old toolset permission (M9 scope)
//  4. AgentTool wrappers for peer agents declared in agentCfg
//  5. exit_loop tool when this leaf is the loop's final child
func collectLeafTools(wbc *workflowBuildContext, agentID string, leafCtx *leafContext, isLastInLoop bool) []adktool.Tool {
	tools := wbc.adapter.resolveTools(
		wbc.ctx, wbc.userID, agentID, wbc.modelCfg,
		wbc.threadID, leafCtx.agentCfg, wbc.req, leafCtx.precomputedRules,
	)

	if wbc.adapter.memoryService != nil {
		memEnabled, memPreload := wbc.adapter.resolveMemoryPolicy(leafCtx.agentCfg)
		if memEnabled {
			tools = append(tools, loadmemorytool.New())
			if memPreload {
				tools = append(tools, preloadmemorytool.New())
			}
		}
	}
	if wbc.adapter.artifactService != nil {
		artEnabled, _ := wbc.adapter.resolveArtifactPolicy(leafCtx.agentCfg)
		if artEnabled {
			tools = append(tools, loadartifactstool.New())
		}
	}

	// Wrap each AgentToolRef via ADK-Go's tool/agenttool so leaf agents
	// inside workflows can also expose peer agents as callable tools.
	tools = append(tools, wbc.adapter.resolveAgentTools(
		wbc.ctx, wbc.userID, wbc.modelCfg, wbc.threadID, leafCtx.agentCfg, wbc.req,
	)...)

	if isLastInLoop {
		if exitTool, etErr := exitlooptool.New(); etErr == nil {
			tools = append(tools, exitTool)
		}
	}

	return tools
}

// leafModel pairs the effective model config + LLM + GenerateContentConfig
// for a single leaf, after any per-leaf ModelOverride has been applied.
type leafModel struct {
	cfg    *fagent.ModelConfig
	llm    adkmodel.LLM
	genCfg *genai.GenerateContentConfig
}

func resolveLeafModel(wbc *workflowBuildContext, agentCfg *fagent.Config, runCfg *fagent.RunConfig) (*leafModel, error) {
	cfg := wbc.modelCfg
	llm := wbc.llm

	if agentCfg != nil && agentCfg.ModelOverride != nil {
		overridden, ovErr := wbc.adapter.applyModelOverride(wbc.ctx, wbc.userID, wbc.modelCfg, agentCfg.ModelOverride)
		if ovErr != nil {
			return nil, ovErr
		}
		cfg = overridden
		overrideLLM, llmErr := NewModel(wbc.ctx, cfg.Provider, cfg.Model, cfg.APIKey)
		if llmErr != nil {
			return nil, fmt.Errorf("creating LLM for model override (provider %q, model %q): %w", cfg.Provider, cfg.Model, llmErr)
		}
		llm = overrideLLM
	}

	// Per-agent overrides apply on top of leaf model cfg; per-request RunConfig
	// overrides apply on top of that.
	var agentModelCfg *fagent.AgentModelConfig
	if agentCfg != nil {
		agentModelCfg = agentCfg.ModelConfig
	}
	genCfg := buildGenerateContentConfig(cfg, agentModelCfg, runCfg)
	// Built-in code execution — provider dispatch. Each leaf carries
	// its own CodeExecutionPolicy, so a workflow can mix code-executing
	// and non-code-executing leaves freely.
	wbc.adapter.applyCodeExecutionTool(agentCfg, cfg, genCfg)

	return &leafModel{cfg: cfg, llm: llm, genCfg: genCfg}, nil
}

// recordLeafPosition stores this leaf's structural context so the runner event
// loop can enrich AgentTransferStarted events with step/branch/iteration info.
func recordLeafPosition(wbc *workflowBuildContext, agentName string, parentCtx *agentPositionCtx) {
	if parentCtx == nil {
		return
	}
	wbc.positions[agentName] = AgentPosition{
		WorkflowType:  parentCtx.workflowType,
		ParentName:    parentCtx.parentName,
		StepIndex:     parentCtx.stepIndex,
		StepCount:     parentCtx.stepCount,
		MaxIterations: parentCtx.maxIterations,
	}
}

// leafCallbacks groups the ADK-native callback slices wired for one leaf.
// The pipeline mirrors the single-agent path in runner.Run so workflow
// sub-agents get the same logging and enforcement treatment:
//  1. Tool + model logging (records timing and token usage)
//  2. Block enforcement (BlockMiddleware; before-only)
//  3. Confirmation enforcement (ADK-native HITL protocol)
//  4. Requirements-driven BeforeAgent hooks (guidance injection)
//  5. Tool error handling (ErrConfirmationRequired / ErrConfirmationRejected
//     sentinels from ADK-Go v1.0.0; logs unexpected native tool errors)
type leafCallbacks struct {
	beforeAgent []agent.BeforeAgentCallback
	beforeModel []llmagent.BeforeModelCallback
	afterModel  []llmagent.AfterModelCallback
	beforeTool  []llmagent.BeforeToolCallback
	afterTool   []llmagent.AfterToolCallback
	onToolError []llmagent.OnToolErrorCallback
}

func buildLeafCallbacks(wbc *workflowBuildContext, agentName, agentID string, precomputedRules []fagent.Rule) leafCallbacks {
	var cbs leafCallbacks

	if wbc.adapter.log != nil {
		logBefore, logAfter := toolLoggingCallbacks(wbc.adapter.log, agentName, wbc.threadID, wbc.userID)
		cbs.beforeTool = append(cbs.beforeTool, logBefore)
		cbs.afterTool = append(cbs.afterTool, logAfter)
		cbs.onToolError = append(cbs.onToolError, toolErrorCallback(wbc.adapter.log, agentName, wbc.threadID))
		modelBefore, modelAfter := modelLoggingCallbacks(wbc.adapter.log, agentName, wbc.threadID)
		cbs.beforeModel = append(cbs.beforeModel, modelBefore)
		cbs.afterModel = append(cbs.afterModel, modelAfter)
	}

	var enforcerMWs []fagent.Middleware
	if wbc.adapter.blockEnforcer != nil {
		enforcerMWs = append(enforcerMWs, wbc.adapter.blockEnforcer.BlockMiddleware(agentID))
	}
	cbs.beforeTool = append(cbs.beforeTool, buildBeforeOnlyCallbacks(agentName, wbc.threadID, wbc.userID, enforcerMWs...)...)

	if wbc.adapter.confirmChecker != nil {
		cbs.beforeTool = append(cbs.beforeTool, buildConfirmationCallback(wbc.adapter.confirmChecker, agentName, wbc.userID, ""))
	}

	cbs.beforeAgent = buildBeforeRunHooks(precomputedRules)

	return cbs
}
