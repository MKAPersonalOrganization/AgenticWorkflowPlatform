package adkgo

import (
	"context"
	"fmt"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/parallelagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	adkmodel "google.golang.org/adk/model"

	fagent "aicompass/foundation/agent"
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
// File layout (split for bus-factor / reviewability):
//   workflow.go              — shared types, dispatch, sequential/parallel/loop
//   workflow_leaf.go         — buildLeafAgent + resolve/collect/build helpers
//   workflow_conditional.go  — buildConditionalNode + conditionalRunFn
//   workflow_remote.go       — buildRemoteAgentNode + remoteRunFn + A2A events
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

// isWorkflowAgent checks whether a foundation Agent implements the WorkflowAgent interface.
func isWorkflowAgent(a fagent.Agent) (fagent.WorkflowAgent, bool) {
	wa, ok := a.(fagent.WorkflowAgent)
	return wa, ok
}
