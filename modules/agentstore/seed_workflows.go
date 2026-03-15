package agentstore

import (
	"encoding/json"

	"github.com/google/uuid"
)

// workflowCatalog returns one Workflow Agent entry per ADK-Go example.
//
// ADK-Go examples → catalog entries (1:1):
//   - examples/workflowagents/sequential/main.go       → Sequential Agent
//   - examples/workflowagents/sequentialCode/main.go   → Code Generation Pipeline
//   - examples/workflowagents/parallel/main.go         → Parallel Agent
//   - examples/workflowagents/loop/main.go             → Loop Agent
func workflowCatalog() []AgentCatalogEntry {
	return []AgentCatalogEntry{
		sequentialAgentEntry(),
		codeGenerationPipelineEntry(),
		parallelAgentEntry(),
		loopAgentEntry(),
	}
}

// ─── Sequential Agent ────────────────────────────────────────────────────────
// Mirrors: examples/workflowagents/sequential/main.go
//
// Two sub-agents (my_custom_agent_1, my_custom_agent_2) run one after another
// inside a sequentialagent. Each completes fully before the next starts.
// Output from each agent is visible downstream in the conversation history.

func sequentialAgentEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-seq-agent-1",
			Name:        "Agent 1",
			Description: "First agent in the sequence — runs before Agent 2.",
			Icon:        iconBot,
			SystemPrompt: `You are the first agent in a sequential pipeline.

Respond to the user's message with a clear, direct answer. Your output becomes part of the conversation history that Agent 2 will receive.

State your response and end with: "— Agent 1 complete."`,
		},
		{
			ID:          "sub-seq-agent-2",
			Name:        "Agent 2",
			Description: "Second agent in the sequence — runs after Agent 1 completes.",
			Icon:        iconBot,
			SystemPrompt: `You are the second agent in a sequential pipeline.

You can see everything Agent 1 said. Build on it, extend it, or provide a complementary perspective.

State your response and end with: "— Agent 2 complete."`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-seq-agent-1"},
			{"id": "n2", "type": "agent", "agent_id": "sub-seq-agent-2"},
		},
	}

	return catalogWorkflowEntry(
		"Sequential Agent",
		"Two LLM agents that run one after another. Mirrors ADK-Go's sequential example: my_custom_agent_1 → my_custom_agent_2 inside sequentialagent.New().",
		"Run two agents in order inside a sequentialagent.",
		"Examples",
		[]string{"sequential", "two-agents", "ordered", "adk-example"},
		iconPipeline,
		false,
		subs,
		graph,
	)
}

// ─── Code Generation Pipeline ────────────────────────────────────────────────
// Mirrors: examples/workflowagents/sequentialCode/main.go
//
// Three LLM agents in a sequentialagent. Demonstrates the key ADK-Go pattern:
// OutputKey saves an agent's output to session state; downstream agents read it
// via {variable} template syntax in their Instruction.
//
//	CodeWriterAgent    → output_key: "generated_code"
//	CodeReviewerAgent  → reads {generated_code}, output_key: "temp:review_comments"
//	CodeRefactorerAgent → reads {generated_code} + {temp:review_comments}

func codeGenerationPipelineEntry() AgentCatalogEntry {
	fence := "```"

	subs := []CatalogSubAgent{
		{
			ID:          "sub-code-writer",
			Name:        "CodeWriterAgent",
			Description: "Generates Python code from the user's specification. Stores output in session state via OutputKey.",
			Icon:        iconTerminal,
			SystemPrompt: `You are a Python Code Generator.
Based *only* on the user's request, write Python code that fulfills the requirement.
Output *only* the complete Python code block, enclosed in triple backticks.
Do not add any other text before or after the code block.`,
		},
		{
			ID:          "sub-code-reviewer",
			Name:        "CodeReviewerAgent",
			Description: "Reviews the generated code. Reads {generated_code} from session state — set by CodeWriterAgent's OutputKey.",
			Icon:        iconSearchCode,
			SystemPrompt: `You are an expert Python Code Reviewer.
Your task is to provide constructive feedback on the provided code.

**Code to Review:**
` + fence + `python
{generated_code}
` + fence + `

**Review Criteria:**
1. **Correctness:** Does the code work as intended? Are there logic errors?
2. **Readability:** Is the code clear and easy to understand? Follows PEP 8?
3. **Efficiency:** Is the code reasonably efficient? Any obvious performance bottlenecks?
4. **Edge Cases:** Does the code handle potential edge cases or invalid inputs gracefully?
5. **Best Practices:** Does the code follow common Python best practices?

**Output:**
Provide your feedback as a concise, bulleted list. Focus on the most important points for improvement.
If the code is excellent and requires no changes, simply state: "No major issues found."
Output *only* the review comments or the "No major issues" statement.`,
		},
		{
			ID:          "sub-code-refactorer",
			Name:        "CodeRefactorerAgent",
			Description: "Refactors the code using review feedback. Reads both {generated_code} and {temp:review_comments} from session state.",
			Icon:        iconCode,
			SystemPrompt: `You are a Python Code Refactoring AI.
Your goal is to improve the given Python code based on the provided review comments.

**Original Code:**
` + fence + `python
{generated_code}
` + fence + `

**Review Comments:**
{temp:review_comments}

**Task:**
Carefully apply the suggestions from the review comments to refactor the original code.
If the review comments state "No major issues found," return the original code unchanged.
Ensure the final code is complete, functional, and includes necessary imports and docstrings.

**Output:**
Output *only* the final, refactored Python code block, enclosed in triple backticks.
Do not add any other text before or after the code block.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			// OutputKey saves output to session.State["generated_code"].
			// Downstream agents reference it via {generated_code} in their Instruction.
			{"id": "n1", "type": "agent", "agent_id": "sub-code-writer", "output_key": "generated_code"},
			// temp: prefix → discarded from state after the invocation ends.
			{"id": "n2", "type": "agent", "agent_id": "sub-code-reviewer", "output_key": "temp:review_comments"},
			{"id": "n3", "type": "agent", "agent_id": "sub-code-refactorer", "output_key": "refactored_code"},
		},
	}

	return catalogWorkflowEntry(
		"Code Generation Pipeline",
		"Mirrors ADK-Go's sequentialCode example: CodeWriterAgent → CodeReviewerAgent → CodeRefactorerAgent. Demonstrates OutputKey → {variable} state passing between agents in a sequentialagent.",
		"Write, review, and refactor Python code using sequential state passing.",
		"Examples",
		[]string{"sequential", "state-passing", "output-key", "code", "adk-example"},
		iconGitBranch,
		true,
		subs,
		graph,
	)
}

// ─── Parallel Agent ───────────────────────────────────────────────────────────
// Mirrors: examples/workflowagents/parallel/main.go
//
// Two sub-agents run concurrently inside a parallelagent. Each gets its own
// isolated branch of the conversation (branch = "parent.agent_name").
// Neither agent sees the other's output. Results are collected after both finish.

func parallelAgentEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-par-agent-1",
			Name:        "Agent 1",
			Description: "Runs concurrently alongside Agent 2 in its own isolated branch.",
			Icon:        iconBot,
			SystemPrompt: `You are the first agent running in a parallel branch.

You and Agent 2 are running at the same time — you cannot see each other's output.

Respond to the user's message from your own perspective. Be direct and complete.

End your response with: "— Agent 1 (parallel branch 1)"`,
		},
		{
			ID:          "sub-par-agent-2",
			Name:        "Agent 2",
			Description: "Runs concurrently alongside Agent 1 in its own isolated branch.",
			Icon:        iconBot,
			SystemPrompt: `You are the second agent running in a parallel branch.

You and Agent 1 are running at the same time — you cannot see each other's output.

Respond to the user's message from a different angle than Agent 1 might take. Offer a fresh perspective.

End your response with: "— Agent 2 (parallel branch 2)"`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "parallel",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-par-agent-1"},
			{"id": "n2", "type": "agent", "agent_id": "sub-par-agent-2"},
		},
	}

	return catalogWorkflowEntry(
		"Parallel Agent",
		"Two LLM agents that run concurrently in isolated branches. Mirrors ADK-Go's parallel example: my_custom_agent_1 ∥ my_custom_agent_2 inside parallelagent.New().",
		"Run two agents simultaneously in isolated conversation branches.",
		"Examples",
		[]string{"parallel", "concurrent", "two-agents", "branches", "adk-example"},
		iconWorkflow,
		false,
		subs,
		graph,
	)
}

// ─── Loop Agent ───────────────────────────────────────────────────────────────
// Mirrors: examples/workflowagents/loop/main.go
//
// A single sub-agent runs repeatedly inside a loopagent. The loop terminates
// when MaxIterations is reached (3, same as the ADK-Go example) or when the
// agent sets event.Actions.Escalate = true (exit_loop tool).

func loopAgentEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-loop-agent",
			Name:        "My Agent",
			Description: "Runs repeatedly up to MaxIterations=3, or until it calls exit_loop.",
			Icon:        iconRefresh,
			SystemPrompt: `You are an iterative refinement agent. You run in a loop — up to 3 times.

On each iteration:
1. Review what you produced in previous iterations (visible in conversation history).
2. Improve your answer: fix errors, add depth, sharpen clarity.
3. If your answer is already excellent and needs no further improvement, call the exit_loop tool to end the loop early.

Always label your response with the current pass: "Pass 1:", "Pass 2:", "Pass 3:".`,
		},
	}

	graph := map[string]interface{}{
		"id":             "root",
		"type":           "loop",
		"max_iterations": 3,
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-loop-agent"},
		},
	}

	return catalogWorkflowEntry(
		"Loop Agent",
		"A single LLM agent that runs up to MaxIterations=3, improving its output each pass. Mirrors ADK-Go's loop example: my_custom_agent inside loopagent.New(MaxIterations: 3).",
		"Run one agent in a refinement loop, improving output each iteration.",
		"Examples",
		[]string{"loop", "iterative", "max-iterations", "escalate", "adk-example"},
		iconRefresh,
		false,
		subs,
		graph,
	)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func catalogWorkflowEntry(
	name, description, systemPrompt, category string,
	tags []string,
	icon string,
	featured bool,
	subs []CatalogSubAgent,
	graph map[string]interface{},
) AgentCatalogEntry {
	graphJSON, _ := json.Marshal(graph)
	subsJSON, _ := json.Marshal(subs)
	id := deterministicID("catalog-workflow:" + name)
	return AgentCatalogEntry{
		ID:                id,
		Name:              name,
		Description:       description,
		SystemPrompt:      systemPrompt,
		Category:          category,
		Tags:              tags,
		AuthorName:        "ADK-Go",
		Icon:              icon,
		Featured:          featured,
		WorkflowGraphJSON: string(graphJSON),
		SubAgentsJSON:     string(subsJSON),
	}
}

func deterministicID(seed string) string {
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(seed)).String()
}
