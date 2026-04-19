package adkgo

import (
	"fmt"
	"iter"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"

	fagent "aicompass/foundation/agent"
)

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
	if evaluatorNode.Type != "agent" || evaluatorNode.AgentID == "" {
		return nil, fmt.Errorf("node %q: evaluator (first child) must be an agent node with agent_id", node.ID)
	}

	// Read output_key from the evaluator's AgentDefinition — the agent owns its
	// routing config. Templates declare it in sub_agents[].output_key and the fork
	// process transfers it to AgentDefinition.OutputKey.
	foundAgent, err := wbc.adapter.agentProvider.GetForUser(wbc.ctx, evaluatorNode.AgentID, wbc.userID)
	if err != nil {
		return nil, fmt.Errorf("node %q: resolving evaluator agent: %w", node.ID, err)
	}
	evalCfg := extractAgentConfig(foundAgent)
	if evalCfg.OutputKey == "" {
		return nil, fmt.Errorf("node %q: evaluator agent %q must have output_key set in its definition", node.ID, foundAgent.Name())
	}
	outputKey := evalCfg.OutputKey

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
	branches := make([]conditionalBranch, 0, len(node.Children)-1)
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
		branches = append(branches, conditionalBranch{
			condition: condition,
			agent:     branchAgent,
		})
	}

	// Build all sub-agents for ADK-Go's SubAgents list (evaluator + all branches).
	allSubAgents := []agent.Agent{evaluatorAgent}
	for _, b := range branches {
		allSubAgents = append(allSubAgents, b.agent)
	}

	return agent.New(agent.Config{
		Name:        parentName,
		Description: fmt.Sprintf("Conditional router: evaluates then branches on condition match (%d branches)", len(branches)),
		SubAgents:   allSubAgents,
		Run:         conditionalRunFn(evaluatorAgent, branches, outputKey, parentName),
	})
}

// conditionalBranch pairs a condition string with the agent it selects.
type conditionalBranch struct {
	condition string
	agent     agent.Agent
}

// conditionalRunFn returns the Run closure that implements runtime routing:
// drain the evaluator, read its output from session state, select the first
// branch whose condition matches (case-insensitive exact, "*" = fallback),
// then run that branch. Lifted out of buildConditionalNode so the routing
// logic is unit-testable without constructing a full workflow tree.
func conditionalRunFn(evaluatorAgent agent.Agent, branches []conditionalBranch, outputKey, parentName string) func(agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
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
}
