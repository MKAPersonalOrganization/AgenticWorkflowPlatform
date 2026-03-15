package adkgo

import (
	"context"
	"fmt"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/parallelagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	adkmodel "google.golang.org/adk/model"
	"google.golang.org/adk/tool/exitlooptool"

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
func buildWorkflowAgent(wbc *workflowBuildContext, wa fagent.WorkflowAgent) (agent.Agent, error) {
	graph := wa.WorkflowGraph()
	return buildNode(wbc, &graph, false)
}

// buildNode recursively converts a WorkflowNodeInfo into the corresponding
// ADK-Go agent. Leaf nodes become LLM agents; composite nodes become
// sequential/parallel/loop agents wrapping their children.
func buildNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo, isLastInLoop bool) (agent.Agent, error) {
	switch node.Type {
	case "agent":
		return buildLeafAgent(wbc, node, isLastInLoop)

	case "sequential":
		return buildSequentialNode(wbc, node)

	case "parallel":
		return buildParallelNode(wbc, node)

	case "loop":
		return buildLoopNode(wbc, node)

	default:
		return nil, fmt.Errorf("node %q: unsupported type %q", node.ID, node.Type)
	}
}

// buildSequentialNode creates an ADK-Go sequentialagent from a composite node.
func buildSequentialNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo) (agent.Agent, error) {
	if len(node.Children) == 0 {
		return nil, fmt.Errorf("node %q: sequential node has no children", node.ID)
	}

	subAgents := make([]agent.Agent, 0, len(node.Children))
	for i := range node.Children {
		child, err := buildNode(wbc, &node.Children[i], false)
		if err != nil {
			return nil, fmt.Errorf("node %q child %d: %w", node.ID, i, err)
		}
		subAgents = append(subAgents, child)
	}

	name := node.Label
	if name == "" {
		name = fmt.Sprintf("seq-%s", node.ID)
	}

	return sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:      wbc.uniqueName(name),
			SubAgents: subAgents,
		},
	})
}

// buildParallelNode creates an ADK-Go parallelagent from a composite node.
func buildParallelNode(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo) (agent.Agent, error) {
	if len(node.Children) == 0 {
		return nil, fmt.Errorf("node %q: parallel node has no children", node.ID)
	}

	subAgents := make([]agent.Agent, 0, len(node.Children))
	for i := range node.Children {
		child, err := buildNode(wbc, &node.Children[i], false)
		if err != nil {
			return nil, fmt.Errorf("node %q child %d: %w", node.ID, i, err)
		}
		subAgents = append(subAgents, child)
	}

	name := node.Label
	if name == "" {
		name = fmt.Sprintf("par-%s", node.ID)
	}

	return parallelagent.New(parallelagent.Config{
		AgentConfig: agent.Config{
			Name:      wbc.uniqueName(name),
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

	subAgents := make([]agent.Agent, 0, len(node.Children))
	lastIdx := len(node.Children) - 1
	for i := range node.Children {
		// Mark the last child so buildLeafAgent adds the exit_loop tool.
		child, err := buildNode(wbc, &node.Children[i], i == lastIdx)
		if err != nil {
			return nil, fmt.Errorf("node %q child %d: %w", node.ID, i, err)
		}
		subAgents = append(subAgents, child)
	}

	maxIter := node.MaxIterations
	if maxIter < 1 {
		maxIter = 3
	}

	name := node.Label
	if name == "" {
		name = fmt.Sprintf("loop-%s", node.ID)
	}

	return loopagent.New(loopagent.Config{
		MaxIterations: uint(maxIter),
		AgentConfig: agent.Config{
			Name:      wbc.uniqueName(name),
			SubAgents: subAgents,
		},
	})
}

// buildLeafAgent resolves a single agent node into an ADK-Go LLM agent.
// It uses the same tool resolution and prompt building pipeline as regular agents.
func buildLeafAgent(wbc *workflowBuildContext, node *fagent.WorkflowNodeInfo, isLastInLoop bool) (agent.Agent, error) {
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

	// Evaluate requirements for this agent.
	var precomputedRules []fagent.Rule
	if agentCfg != nil {
		for _, requirement := range agentCfg.Requirements {
			if e, ok := requirement.(fagent.Enableable); ok && !e.Enabled() {
				continue
			}
			rules, rErr := requirement.Run(wbc.ctx, inv)
			if rErr != nil {
				continue
			}
			precomputedRules = append(precomputedRules, rules...)
		}
	}

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

	cfg := llmagent.Config{
		Name:                  wbc.uniqueName(foundAgent.Name()),
		Description:           foundAgent.Description(),
		Model:                 wbc.llm,
		Instruction:           systemPrompt,
		Tools:                 adkTools,
		GenerateContentConfig: genCfg,
	}

	if node.OutputKey != "" {
		cfg.OutputKey = node.OutputKey
	}

	return llmagent.New(cfg)
}

// isWorkflowAgent checks whether a foundation Agent implements the WorkflowAgent interface.
func isWorkflowAgent(a fagent.Agent) (fagent.WorkflowAgent, bool) {
	wa, ok := a.(fagent.WorkflowAgent)
	return wa, ok
}
