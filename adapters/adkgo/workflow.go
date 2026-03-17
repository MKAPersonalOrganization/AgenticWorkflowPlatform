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
func buildNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo, isLastInLoop bool, parentCtx *agentPositionCtx) (agent.Agent, error) {
	switch node.Type {
	case "agent":
		return buildLeafAgent(wbc, node, isLastInLoop, parentCtx)

	case "sequential":
		return buildSequentialNode(wbc, node)

	case "parallel":
		return buildParallelNode(wbc, node)

	case "loop":
		return buildLoopNode(wbc, node)

	case "remote":
		return buildRemoteAgentNode(wbc, node)

	default:
		return nil, fmt.Errorf("node %q: unsupported type %q", node.ID, node.Type)
	}
}

// buildSequentialNode creates an ADK-Go sequentialagent from a composite node.
func buildSequentialNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo) (agent.Agent, error) {
	if len(node.Children) == 0 {
		return nil, fmt.Errorf("node %q: sequential node has no children", node.ID)
	}

	// Reserve the parent name first so children can reference it.
	name := node.Label
	if name == "" {
		name = fmt.Sprintf("seq-%s", node.ID)
	}
	parentName := wbc.uniqueName(name)

	subAgents := make([]agent.Agent, 0, len(node.Children))
	for i := range node.Children {
		childCtx := &agentPositionCtx{
			workflowType: "sequential",
			parentName:   parentName,
			stepIndex:    i,
			stepCount:    len(node.Children),
		}
		child, err := buildNode(wbc, &node.Children[i], false, childCtx)
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
func buildParallelNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo) (agent.Agent, error) {
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
		child, err := buildNode(wbc, &node.Children[i], false, childCtx)
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
// The exit_loop tool is automatically added to the last child agent
// (if it's a leaf) so the LLM can break out of the loop.
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
		// Mark the last child so buildLeafAgent adds the exit_loop tool.
		child, err := buildNode(wbc, &node.Children[i], i == lastIdx, childCtx)
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

	// Build generate content config.
	genCfg := buildGenerateContentConfig(wbc.modelCfg)

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

	// Build native ADK tool callbacks for this workflow sub-agent:
	// the same logging + enforcement pipeline as regular agents.
	var beforeToolCBs []llmagent.BeforeToolCallback
	var afterToolCBs []llmagent.AfterToolCallback

	// 1. Tool logging — native Before/After callbacks.
	if wbc.adapter.log != nil {
		logBefore, logAfter := toolLoggingCallbacks(wbc.adapter.log, agentName, wbc.threadID, wbc.userID)
		beforeToolCBs = append(beforeToolCBs, logBefore)
		afterToolCBs = append(afterToolCBs, logAfter)
	}

	// 2. Block and confirmation enforcement — before-only.
	var enforcerMWs []fagent.Middleware
	if wbc.adapter.blockEnforcer != nil {
		enforcerMWs = append(enforcerMWs, wbc.adapter.blockEnforcer.BlockMiddleware(node.AgentID))
	}
	if wbc.adapter.confirmEnforcer != nil {
		enforcerMWs = append(enforcerMWs, wbc.adapter.confirmEnforcer.ConfirmationMiddleware(node.AgentID))
	}
	beforeToolCBs = append(beforeToolCBs, buildBeforeOnlyCallbacks(agentName, wbc.threadID, wbc.userID, enforcerMWs...)...)

	return buildLLMAgent(llmAgentParams{
		name:                 agentName,
		foundAgent:           foundAgent,
		agentCfg:             agentCfg,
		llm:                  wbc.llm,
		genCfg:               genCfg,
		tools:                adkTools,
		prompt:               systemPrompt,
		beforeAgentCallbacks: buildBeforeRunHooks(precomputedRules),
		beforeToolCallbacks:  beforeToolCBs,
		afterToolCallbacks:   afterToolCBs,
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
