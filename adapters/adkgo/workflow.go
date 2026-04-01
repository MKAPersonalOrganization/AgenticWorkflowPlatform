package adkgo

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/parallelagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool/exitlooptool"
	"google.golang.org/adk/tool/loadartifactstool"
	"google.golang.org/adk/tool/loadmemorytool"
	"google.golang.org/adk/tool/preloadmemorytool"
	"google.golang.org/genai"

	fagent "school-with-ai/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// Recursive Workflow Agent Builder
//
// When the RunnerAdapter encounters a WorkflowAgent, it recursively builds
// the appropriate ADK-Go agent tree from the WorkflowNodeInfo graph.
//
// The tree supports arbitrary composition:
//
//   sequential
//   ├─ agent(researcher)          → llmagent
//   ├─ parallel                   → parallelagent
//   │  ├─ agent(writer)           → llmagent
//   │  └─ agent(fact-checker)     → llmagent
//   └─ loop                       → loopagent
//      ├─ agent(reviewer)         → llmagent
//      └─ agent(editor)           → llmagent (+ exitlooptool)
//
// This maps 1:1 to ADK-Go's composition model where SubAgents []Agent
// accepts any agent type, enabling infinite nesting.
//
// See arch/adr/019-multi-agent-workflow-composition.md.
// ──────────────────────────────────────────────────────────────────────────────

// AgentPosition describes a leaf agent's location within a workflow tree.
// This is built during workflow construction and used at runtime to enrich
// AgentTransferStarted events with structural context (step/branch/iteration).
type AgentPosition struct {
	WorkflowType  string // "sequential", "parallel", "loop"
	ParentName    string // ADK-Go name of the parent composite agent
	StepIndex     int    // 0-based index among siblings
	StepCount     int    // Total number of siblings
	MaxIterations int    // Only for loop contexts
}

// agentPositionCtx carries parent composite node info during recursive build.
type agentPositionCtx struct {
	workflowType  string
	parentName    string
	stepIndex     int
	stepCount     int
	maxIterations int
}

// workflowBuildContext carries the dependencies needed to build a workflow agent.
type workflowBuildContext struct {
	ctx      context.Context
	adapter  *RunnerAdapter
	userID   string
	modelCfg *fagent.ModelConfig
	llm      adkmodel.LLM
	threadID string
	req      *fagent.RunRequest

	// usedNames tracks agent names already assigned in this tree.
	// ADK-Go requires every agent name in the tree to be unique.
	usedNames map[string]int

	// positions maps leaf agent names → their position in the workflow tree.
	// Populated during build, consumed by the runner event loop.
	positions map[string]AgentPosition
}

// uniqueName returns a name guaranteed to be unique within this build.
// First occurrence keeps the original name; subsequent ones get " (2)", " (3)", etc.
func (wbc *workflowBuildContext) uniqueName(name string) string {
	if wbc.usedNames == nil {
		wbc.usedNames = make(map[string]int)
	}
	wbc.usedNames[name]++
	if wbc.usedNames[name] == 1 {
		return name
	}
	return fmt.Sprintf("%s (%d)", name, wbc.usedNames[name])
}

// buildWorkflowAgent converts a foundation WorkflowAgent into an ADK-Go
// agent tree by recursively walking the WorkflowNodeInfo graph.
// It also returns a position map for leaf agents so the runner can enrich
// agent transfer events with workflow structure context.
func buildWorkflowAgent(wbc *workflowBuildContext, wa fagent.WorkflowAgent) (agent.Agent, map[string]AgentPosition, error) {
	wbc.positions = make(map[string]AgentPosition)
	graph := wa.WorkflowGraph()
	agnt, err := buildNode(wbc, &graph, false, nil)
	return agnt, wbc.positions, err
}

// buildNode recursively converts a WorkflowNodeInfo into the corresponding
// ADK-Go agent. Leaf nodes become LLM agents; composite nodes become
// sequential/parallel/loop agents wrapping their children.
// parentCtx carries the parent composite node's info for position tracking.
//
// isLastInLoop is propagated into composite nodes so that the exit_loop tool
// reaches the deepest last leaf even when intermediate composite nodes sit
// between the loop and the leaf agent.
func buildNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo, isLastInLoop bool, parentCtx *agentPositionCtx) (agent.Agent, error) {
	switch node.Type {
	case "agent":
		return buildLeafAgent(wbc, node, isLastInLoop, parentCtx)

	case "sequential":
		// Propagate isLastInLoop: only the last child in the sequence is last.
		return buildSequentialNode(wbc, node, isLastInLoop)

	case "parallel":
		// Propagate isLastInLoop to all children: any branch may signal exit.
		return buildParallelNode(wbc, node, isLastInLoop)

	case "loop":
		// Nested loops manage their own exit_loop tool; do not inherit outer flag.
		return buildLoopNode(wbc, node)

	case "remote":
		// Remote agents run externally and cannot invoke the exit_loop tool.
		return buildRemoteAgentNode(wbc, node)

	case "conditional":
		// Conditional routing: evaluator → condition match → single branch.
		return buildConditionalNode(wbc, node)

	default:
		return nil, fmt.Errorf("node %q: unsupported type %q", node.ID, node.Type)
	}
}

// buildSequentialNode creates an ADK-Go sequentialagent from a composite node.
// isLastInLoop is forwarded to the last child so the exit_loop tool reaches
// the final agent even when a sequential node sits inside a loop.
func buildSequentialNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo, isLastInLoop bool) (agent.Agent, error) {
	if len(node.Children) == 0 {
		return nil, fmt.Errorf("node %q: sequential node has no children", node.ID)
	}

	// Reserve the parent name first so children can reference it.
	name := node.Label
	if name == "" {
		name = fmt.Sprintf("seq-%s", node.ID)
	}
	parentName := wbc.uniqueName(name)

	lastIdx := len(node.Children) - 1
	subAgents := make([]agent.Agent, 0, len(node.Children))
	for i := range node.Children {
		childCtx := &agentPositionCtx{
			workflowType: "sequential",
			parentName:   parentName,
			stepIndex:    i,
			stepCount:    len(node.Children),
		}
		// Only the last child of this sequential is "last in loop".
		childIsLast := isLastInLoop && i == lastIdx
		child, err := buildNode(wbc, &node.Children[i], childIsLast, childCtx)
		if err != nil {
			return nil, fmt.Errorf("node %q child %d: %w", node.ID, i, err)
		}
		subAgents = append(subAgents, child)
	}

	return sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:      parentName,
			SubAgents: subAgents,
		},
	})
}

// buildParallelNode creates an ADK-Go parallelagent from a composite node.
// isLastInLoop is forwarded to ALL children: since parallel branches run
// concurrently, any branch may reach the completion condition and signal exit.
func buildParallelNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo, isLastInLoop bool) (agent.Agent, error) {
	if len(node.Children) == 0 {
		return nil, fmt.Errorf("node %q: parallel node has no children", node.ID)
	}

	// Reserve the parent name first so children can reference it.
	name := node.Label
	if name == "" {
		name = fmt.Sprintf("par-%s", node.ID)
	}
	parentName := wbc.uniqueName(name)

	subAgents := make([]agent.Agent, 0, len(node.Children))
	for i := range node.Children {
		childCtx := &agentPositionCtx{
			workflowType: "parallel",
			parentName:   parentName,
			stepIndex:    i,
			stepCount:    len(node.Children),
		}
		child, err := buildNode(wbc, &node.Children[i], isLastInLoop, childCtx)
		if err != nil {
			return nil, fmt.Errorf("node %q child %d: %w", node.ID, i, err)
		}
		subAgents = append(subAgents, child)
	}

	return parallelagent.New(parallelagent.Config{
		AgentConfig: agent.Config{
			Name:      parentName,
			SubAgents: subAgents,
		},
	})
}

// buildLoopNode creates an ADK-Go loopagent from a composite node.
// When AllowEarlyExit is true the exit_loop tool is added to the last child
// agent so the LLM can break out before MaxIterations. When false (default)
// the loop always runs exactly MaxIterations times.
func buildLoopNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo) (agent.Agent, error) {
	if len(node.Children) == 0 {
		return nil, fmt.Errorf("node %q: loop node has no children", node.ID)
	}

	maxIter := node.MaxIterations
	if maxIter < 1 {
		maxIter = 3
	}

	// Reserve the parent name first so children can reference it.
	name := node.Label
	if name == "" {
		name = fmt.Sprintf("loop-%s", node.ID)
	}
	parentName := wbc.uniqueName(name)

	subAgents := make([]agent.Agent, 0, len(node.Children))
	lastIdx := len(node.Children) - 1
	for i := range node.Children {
		childCtx := &agentPositionCtx{
			workflowType:  "loop",
			parentName:    parentName,
			stepIndex:     i,
			stepCount:     len(node.Children),
			maxIterations: maxIter,
		}
		// Mark the last child so buildLeafAgent adds the exit_loop tool,
		// but only when early exit is explicitly enabled for this loop node.
		child, err := buildNode(wbc, &node.Children[i], node.AllowEarlyExit && i == lastIdx, childCtx)
		if err != nil {
			return nil, fmt.Errorf("node %q child %d: %w", node.ID, i, err)
		}
		subAgents = append(subAgents, child)
	}

	return loopagent.New(loopagent.Config{
		MaxIterations: uint(maxIter),
		AgentConfig: agent.Config{
			Name:      parentName,
			SubAgents: subAgents,
		},
	})
}

// buildConditionalNode creates a custom ADK-Go agent that implements conditional routing.
//
// Architecture (using agent.New with a custom Run function):
//
//	conditional
//	├─ evaluator (Children[0]) — runs first, output stored via OutputKey in session state
//	├─ branch-a (Children[1], Condition="technical") — executes if output matches
//	├─ branch-b (Children[2], Condition="billing")   — executes if output matches
//	└─ branch-c (Children[3], Condition="*")         — fallback (wildcard)
//
// The evaluator agent MUST have an OutputKey set so its output is persisted to
// session state. The conditional node reads this value and matches it against
// each branch's Condition field using case-insensitive exact string matching.
// A branch with Condition="*" always matches (fallback/default).
//
// This follows ADK-Go's pattern of building custom agents via agent.New()
// with a custom Run function, as demonstrated in the loop agent's escalation
// check and the Python ADK's WorkflowAgent conditional routing.
func buildConditionalNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo) (agent.Agent, error) {
	if len(node.Children) < 2 {
		return nil, fmt.Errorf("node %q: conditional node requires at least 2 children (evaluator + 1 branch)", node.ID)
	}

	evaluatorNode := &node.Children[0]
	if evaluatorNode.OutputKey == "" {
		return nil, fmt.Errorf("node %q: evaluator child (first child) must have output_key set", node.ID)
	}

	outputKey := evaluatorNode.OutputKey

	name := node.Label
	if name == "" {
		name = fmt.Sprintf("cond-%s", node.ID)
	}
	parentName := wbc.uniqueName(name)

	// Build the evaluator agent.
	evaluatorAgent, err := buildNode(wbc, evaluatorNode, false, &agentPositionCtx{
		workflowType: "conditional",
		parentName:   parentName,
		stepIndex:    0,
		stepCount:    len(node.Children),
	})
	if err != nil {
		return nil, fmt.Errorf("node %q evaluator: %w", node.ID, err)
	}

	// Build branch agents and their conditions.
	type branch struct {
		condition string
		agent     agent.Agent
	}
	branches := make([]branch, 0, len(node.Children)-1)
	for i := 1; i < len(node.Children); i++ {
		childNode := &node.Children[i]
		branchAgent, bErr := buildNode(wbc, childNode, false, &agentPositionCtx{
			workflowType: "conditional",
			parentName:   parentName,
			stepIndex:    i,
			stepCount:    len(node.Children),
		})
		if bErr != nil {
			return nil, fmt.Errorf("node %q branch %d: %w", node.ID, i, bErr)
		}
		condition := childNode.Condition
		if condition == "" {
			condition = "*"
		}
		branches = append(branches, branch{
			condition: condition,
			agent:     branchAgent,
		})
	}

	// Build all sub-agents for ADK-Go's SubAgents list (evaluator + all branches).
	allSubAgents := []agent.Agent{evaluatorAgent}
	for _, b := range branches {
		allSubAgents = append(allSubAgents, b.agent)
	}

	// Custom Run function implementing conditional routing.
	runFn := func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			// Phase 1: Run the evaluator agent and drain all its events.
			for event, err := range evaluatorAgent.Run(ctx) {
				if !yield(event, err) {
					return
				}
			}

			// Phase 2: Read the evaluator's output from session state.
			raw, stateErr := ctx.Session().State().Get(outputKey)
			if stateErr != nil {
				// OutputKey not set — evaluator didn't produce output.
				// Yield an error event so the caller knows routing failed.
				yield(nil, fmt.Errorf("conditional node %q: evaluator did not set output key %q: %w", parentName, outputKey, stateErr))
				return
			}

			// Convert output to string for matching.
			var evaluatorOutput string
			switch v := raw.(type) {
			case string:
				evaluatorOutput = strings.ToLower(strings.TrimSpace(v))
			default:
				evaluatorOutput = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", v)))
			}

			// Phase 3: Find the matching branch.
			var matchedAgent agent.Agent
			var fallbackAgent agent.Agent

			for _, b := range branches {
				cond := strings.ToLower(strings.TrimSpace(b.condition))
				if cond == "*" {
					fallbackAgent = b.agent
				} else if cond == evaluatorOutput {
					matchedAgent = b.agent
					break
				}
			}

			// Use fallback if no exact match.
			target := matchedAgent
			if target == nil {
				target = fallbackAgent
			}
			if target == nil {
				yield(nil, fmt.Errorf("conditional node %q: evaluator output %q matched no branch condition and no fallback (*) defined", parentName, evaluatorOutput))
				return
			}

			// Phase 4: Run the matched branch agent.
			for event, err := range target.Run(ctx) {
				if !yield(event, err) {
					return
				}
			}
		}
	}

	return agent.New(agent.Config{
		Name:        parentName,
		Description: fmt.Sprintf("Conditional router: evaluates then branches on condition match (%d branches)", len(branches)),
		SubAgents:   allSubAgents,
		Run:         runFn,
	})
}

// buildLeafAgent resolves a single agent node into an ADK-Go LLM agent.
// It uses the same tool resolution, prompt building, and middleware pipeline
// as the regular single-agent path in Run().
//
// Transfer policies are resolved with this precedence:
//  1. Workflow node override (DisallowTransferToParent/Peers in WorkflowNodeInfo)
//  2. AgentDefinition default (stored in DB)
//  3. ADK-Go default (allow all transfers)
func buildLeafAgent(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo, isLastInLoop bool, parentCtx *agentPositionCtx) (agent.Agent, error) {
	if node.AgentID == "" {
		return nil, fmt.Errorf("node %q: agent node requires agent_id", node.ID)
	}

	// Resolve the agent definition.
	foundAgent, err := wbc.adapter.agentProvider.GetForUser(wbc.ctx, node.AgentID, wbc.userID)
	if err != nil {
		return nil, fmt.Errorf("resolving agent %q: %w", node.AgentID, err)
	}

	agentCfg := extractAgentConfig(foundAgent)

	// Build invocation for requirements.
	inv := &fagent.Invocation{
		ID:        wbc.req.InvocationID,
		SessionID: wbc.req.SessionID,
		UserID:    wbc.userID,
		Message:   wbc.req.Message,
		Config:    wbc.req.Config,
		Context:   wbc.req.Context,
	}

	// Evaluate requirements once; reuse for tool filtering and guidance injection.
	precomputedRules := evaluateRequirements(wbc.ctx, agentCfg, inv)

	// Resolve tools.
	adkTools := wbc.adapter.resolveTools(
		wbc.ctx, wbc.userID, node.AgentID, wbc.modelCfg,
		wbc.threadID, agentCfg, wbc.req, precomputedRules,
	)

	// Inject ADK-native tools gated by plugin permissions.
	if wbc.adapter.memoryService != nil && wbc.adapter.hasMemoryEnabled(wbc.ctx, wbc.userID, node.AgentID) {
		adkTools = append(adkTools, preloadmemorytool.New(), loadmemorytool.New())
	}
	if wbc.adapter.artifactService != nil && wbc.adapter.hasArtifactsEnabled(wbc.ctx, wbc.userID, node.AgentID) {
		adkTools = append(adkTools, loadartifactstool.New())
	}

	// For the last child in a loop, add the exit_loop tool.
	if isLastInLoop {
		exitTool, etErr := exitlooptool.New()
		if etErr == nil {
			adkTools = append(adkTools, exitTool)
		}
	}

	// Build system prompt.
	systemPrompt := wbc.adapter.buildSystemPrompt(
		wbc.ctx, foundAgent, wbc.userID, node.AgentID, wbc.threadID, agentCfg, inv,
	)

	// Apply per-leaf model override: if this leaf has a different provider/model,
	// resolve its API key, swap modelCfg, and create a dedicated LLM instance.
	leafModelCfg := wbc.modelCfg
	leafLLM := wbc.llm
	if agentCfg != nil && agentCfg.ModelOverride != nil {
		overridden, ovErr := wbc.adapter.applyModelOverride(wbc.ctx, wbc.userID, wbc.modelCfg, agentCfg.ModelOverride)
		if ovErr != nil {
			return nil, ovErr
		}
		leafModelCfg = overridden
		overrideLLM, llmErr := NewModel(wbc.ctx, leafModelCfg.Provider, leafModelCfg.Model, leafModelCfg.APIKey)
		if llmErr != nil {
			return nil, fmt.Errorf("creating LLM for model override (provider %q, model %q): %w", leafModelCfg.Provider, leafModelCfg.Model, llmErr)
		}
		leafLLM = overrideLLM
	}

	// Build generate content config (per-agent overrides → per-request RunConfig overrides).
	var agentModelCfg *fagent.AgentModelConfig
	if agentCfg != nil {
		agentModelCfg = agentCfg.ModelConfig
	}
	genCfg := buildGenerateContentConfig(leafModelCfg, agentModelCfg, wbc.req.Config)

	agentName := wbc.uniqueName(foundAgent.Name())

	// Record this leaf's position in the workflow tree so the runner can
	// enrich AgentTransferStarted events with structural context.
	if parentCtx != nil {
		wbc.positions[agentName] = AgentPosition{
			WorkflowType:  parentCtx.workflowType,
			ParentName:    parentCtx.parentName,
			StepIndex:     parentCtx.stepIndex,
			StepCount:     parentCtx.stepCount,
			MaxIterations: parentCtx.maxIterations,
		}
	}

	// Build native ADK tool and model callbacks for this workflow sub-agent:
	// the same logging + enforcement pipeline as regular agents.
	var beforeModelCBs []llmagent.BeforeModelCallback
	var afterModelCBs []llmagent.AfterModelCallback
	var beforeToolCBs []llmagent.BeforeToolCallback
	var afterToolCBs []llmagent.AfterToolCallback

	// 1. Tool + model logging — native Before/After callbacks.
	var onToolErrorCBs []llmagent.OnToolErrorCallback
	if wbc.adapter.log != nil {
		logBefore, logAfter := toolLoggingCallbacks(wbc.adapter.log, agentName, wbc.threadID, wbc.userID)
		beforeToolCBs = append(beforeToolCBs, logBefore)
		afterToolCBs = append(afterToolCBs, logAfter)
		// OnToolErrorCallback handles ErrConfirmationRequired/ErrConfirmationRejected
		// (ADK-Go v1.0.0 sentinels) and logs unexpected native tool errors.
		onToolErrorCBs = append(onToolErrorCBs, toolErrorCallback(wbc.adapter.log, agentName, wbc.threadID))
		// Model logging — records timing and token usage at the model level.
		modelBefore, modelAfter := modelLoggingCallbacks(wbc.adapter.log, agentName, wbc.threadID)
		beforeModelCBs = append(beforeModelCBs, modelBefore)
		afterModelCBs = append(afterModelCBs, modelAfter)
	}

	// 2. Block enforcement — before-only.
	var enforcerMWs []fagent.Middleware
	if wbc.adapter.blockEnforcer != nil {
		enforcerMWs = append(enforcerMWs, wbc.adapter.blockEnforcer.BlockMiddleware(node.AgentID))
	}
	beforeToolCBs = append(beforeToolCBs, buildBeforeOnlyCallbacks(agentName, wbc.threadID, wbc.userID, enforcerMWs...)...)

	// 2b. Confirmation enforcement — ADK-native HITL protocol.
	if wbc.adapter.confirmChecker != nil {
		beforeToolCBs = append(beforeToolCBs, buildConfirmationCallback(wbc.adapter.confirmChecker, agentName, wbc.userID, ""))
	}

	return buildLLMAgent(llmAgentParams{
		name:                 agentName,
		foundAgent:           foundAgent,
		agentCfg:             agentCfg,
		llm:                  leafLLM,
		genCfg:               genCfg,
		tools:                adkTools,
		prompt:               systemPrompt,
		beforeAgentCallbacks: buildBeforeRunHooks(precomputedRules),
		beforeModelCallbacks: beforeModelCBs,
		afterModelCallbacks:  afterModelCBs,
		beforeToolCallbacks:  beforeToolCBs,
		afterToolCallbacks:   afterToolCBs,
		onToolErrorCallbacks: onToolErrorCBs,
		nodeOverride:         node,
	})
}

// buildRemoteAgentNode creates a custom ADK-Go agent that delegates to an
// external A2A endpoint via the A2ASubAgentProvider port.
//
// Because ADK-Go's remoteagent package uses a different version of the a2a-go
// module (v0.3.3 vs our v2), we cannot use remoteagent.NewA2A() directly.
// Instead we build a plain agent.New() whose Run function calls our existing
// a2a.Service.SendMessage() infrastructure, keeping all auth and transport
// logic in one place.
func buildRemoteAgentNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo) (agent.Agent, error) {
	if node.A2AConnectorID == "" {
		return nil, fmt.Errorf("node %q: remote node requires a2a_connector_id", node.ID)
	}
	if wbc.adapter.a2aSubAgentProvider == nil {
		return nil, fmt.Errorf("node %q: A2ASubAgentProvider not configured", node.ID)
	}

	// Resolve connector metadata once at build time for name/description.
	info, err := wbc.adapter.a2aSubAgentProvider.GetConnectorInfo(wbc.ctx, node.A2AConnectorID, wbc.userID)
	if err != nil {
		return nil, fmt.Errorf("node %q: resolving A2A connector %q: %w", node.ID, node.A2AConnectorID, err)
	}

	name := node.Label
	if name == "" {
		name = info.Name
	}
	agentName := wbc.uniqueName(name)

	connectorID := node.A2AConnectorID
	userID := wbc.userID
	outputKey := node.OutputKey
	provider := wbc.adapter.a2aSubAgentProvider

	runFn := func(ictx agent.InvocationContext) iter.Seq2[*session.Event, error] {
		return func(yield func(*session.Event, error) bool) {
			// Extract the user's message text from the invocation context.
			var message string
			if uc := ictx.UserContent(); uc != nil {
				for _, part := range uc.Parts {
					if part != nil && part.Text != "" {
						message = part.Text
						break
					}
				}
			}

			// Stream chunks from the remote agent.
			//
			// Text deltas are buffered one step behind: we emit the previous delta
			// as a PARTIAL event (AssistantContentStreamed → live display) only once
			// we know there is a following delta. The very last delta is emitted as
			// the NON-PARTIAL event so ADK-Go persists the full accumulated text to
			// the session — no separate final event needed, which avoids re-displaying
			// the content a second time.
			//
			// Artifact chunks are emitted immediately as separate non-content events
			// carrying the artifact data in StateDelta, which TranslateEvent converts
			// to thread.A2AArtifactReceived for separate frontend display.
			var accumulated strings.Builder
			var pendingEvt *session.Event

			for chunk, streamErr := range provider.SendStreamingMessage(ictx, connectorID, userID, message) {
				if streamErr != nil {
					yield(nil, fmt.Errorf("remote agent %q: %w", agentName, streamErr))
					return
				}

				if chunk.IsFile {
					// Encode file metadata into StateDelta so TranslateEvent can emit
					// a FileAttachmentReceived event for separate frontend display.
					fileData, _ := json.Marshal(map[string]any{
						"fileURL":  chunk.FileURL,
						"fileName": chunk.FileName,
						"mimeType": chunk.MimeType,
					})
					fileEvt := session.NewEvent(ictx.InvocationID())
					fileEvt.Author = agentName
					fileEvt.Actions.StateDelta = map[string]any{
						"__a2a_file__": string(fileData),
					}
					if !yield(fileEvt, nil) {
						return
					}
					continue
				}

				if chunk.IsArtifact {
					// Skip artifacts whose content duplicates the accumulated streaming
					// text. Python ADK always packages the complete response as a final
					// artifact even for plain conversational responses — showing it would
					// repeat the message the user already sees in the stream.
					// Only surface artifacts with genuinely distinct content (e.g. a
					// generated playlist document, file, or structured output).
					if chunk.Content == "" || chunk.Content == accumulated.String() {
						continue
					}

					// Encode artifact metadata into StateDelta so TranslateEvent
					// can emit thread.A2AArtifactReceived without changing the
					// session.Event structure.
					// Note: pendingEvt is intentionally NOT flushed here — the normal
					// end-of-loop path emits the final NON-PARTIAL text event so the
					// message is persisted correctly regardless of artifact ordering.
					artifactData, _ := json.Marshal(map[string]any{
						"artifactId": chunk.ArtifactID,
						"title":      chunk.Title,
						"content":    chunk.Content,
					})
					artifactEvt := session.NewEvent(ictx.InvocationID())
					artifactEvt.Author = agentName
					artifactEvt.Actions.StateDelta = map[string]any{
						"__a2a_artifact__": string(artifactData),
					}
					if !yield(artifactEvt, nil) {
						return
					}
					continue
				}

				if chunk.Delta == "" {
					continue
				}

				// Flush the previous delta as PARTIAL now that we know it is
				// not the last chunk.
				if pendingEvt != nil {
					if !yield(pendingEvt, nil) {
						return
					}
				}

				accumulated.WriteString(chunk.Delta)

				// Buffer this delta — it will become PARTIAL if another delta
				// follows, or NON-PARTIAL if the stream ends here.
				pendingEvt = session.NewEvent(ictx.InvocationID())
				pendingEvt.Author = agentName
				pendingEvt.Content = &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{{Text: chunk.Delta}},
				}
				pendingEvt.Partial = true
			}

			// Emit the last buffered event as NON-PARTIAL with the full
			// accumulated text so the session service persists the message.
			if pendingEvt != nil {
				pendingEvt.Partial = false
				pendingEvt.Content = &genai.Content{
					Role:  "model",
					Parts: []*genai.Part{{Text: accumulated.String()}},
				}
				if outputKey != "" {
					pendingEvt.Actions.StateDelta = map[string]any{outputKey: accumulated.String()}
				}
				yield(pendingEvt, nil)
			}
		}
	}

	return agent.New(agent.Config{
		Name:        agentName,
		Description: info.Description,
		Run:         runFn,
	})
}

// isWorkflowAgent checks whether a foundation Agent implements the WorkflowAgent interface.
func isWorkflowAgent(a fagent.Agent) (fagent.WorkflowAgent, bool) {
	wa, ok := a.(fagent.WorkflowAgent)
	return wa, ok
}
