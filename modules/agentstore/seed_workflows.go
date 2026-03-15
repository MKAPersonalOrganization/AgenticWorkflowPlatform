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
	adk := []AgentCatalogEntry{
		sequentialAgentEntry(),
		codeGenerationPipelineEntry(),
		parallelAgentEntry(),
		loopAgentEntry(),
		courseGenerationPipelineEntry(),
	}
	return append(adk, crewAICatalog()...)
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

// ─── Course Generation Pipeline ──────────────────────────────────────────────
// Mirrors: examples/course-generator (CrewAI reference implementation)
//
// Four specialised LLM agents in a sequentialagent, mapping the CrewAI
// Crew + Flow hybrid pattern onto ADK-Go's composition model.
//
// CrewAI concept             →  ADK-Go equivalent
// ─────────────────────────────────────────────────────────────
// Crew (sequential process)  →  sequentialagent.New()
// Task context dependencies  →  OutputKey + {variable} templates
// Agent role / backstory     →  Instruction (system prompt)
// Flow state (Pydantic)      →  session.State via OutputKey
// Flow quality router        →  loop node with exit_loop escalation
//
// Pipeline:
//   CurriculumArchitect  → output_key: "curriculum_plan"
//   ContentWriter        → reads {curriculum_plan}, output_key: "lesson_content"
//   QuizMaster           → reads {curriculum_plan} + {lesson_content},
//                           output_key: "temp:quiz_material"
//   CodeReviewer         → reads {lesson_content} + {temp:quiz_material},
//                           output_key: "review_report"

func courseGenerationPipelineEntry() AgentCatalogEntry {
	fence := "```"

	subs := []CatalogSubAgent{
		// ── 1. Curriculum Architect ──────────────────────────────────────
		// CrewAI: curriculum_expert_agent (Instructional Designer)
		// Produces structured learning plan that downstream agents consume.
		{
			ID:          "sub-curriculum-architect",
			Name:        "CurriculumArchitect",
			Description: "Analyses a topic and produces a structured curriculum plan with learning objectives, lesson outline, prerequisites, and suggested code examples.",
			Icon:        iconGraduationCap,
			SystemPrompt: `You are a Curriculum Architect — an expert Instructional Designer.

Given a **topic** and **difficulty level** (beginner / intermediate / advanced) provided by the user, produce a structured curriculum plan in Markdown.

**Your plan MUST include:**
1. **Learning Objectives** — 3-5 clear, measurable outcomes (use Bloom's taxonomy verbs).
2. **Prerequisites** — what the learner should already know.
3. **Lesson Outline** — numbered sections with titles and one-sentence descriptions.
4. **Suggested Code Examples** — for each section, list 1-2 code examples with a short description of what they demonstrate (simple → intermediate → advanced progression).
5. **Estimated Duration** — total time and per-section breakdown.

Output *only* the curriculum plan. Do not write the actual lesson content — that is the next agent's job.`,
		},
		// ── 2. Content Writer ─────────────────────────────────────────────
		// CrewAI: content_writer_agent (Technical Writer)
		// Reads {curriculum_plan} via OutputKey and writes the full lesson.
		{
			ID:          "sub-content-writer",
			Name:        "ContentWriter",
			Description: "Writes a complete Markdown lesson with 3+ code examples based on the curriculum plan. Reads {curriculum_plan} from session state.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a Content Writer — an expert Technical Writer.

You receive a structured curriculum plan produced by the Curriculum Architect.

**Curriculum Plan:**
{curriculum_plan}

**Your task:**
Write a **complete, publication-ready lesson** in Markdown that follows the plan above.

**Requirements:**
- Follow the outline section-by-section. Do not skip or reorder.
- Include **at least 3 working code examples** progressing from simple → intermediate → advanced.
- Wrap each code example in fenced code blocks with the correct language tag.
- Add practical tips, common pitfalls, and best-practice callouts where appropriate.
- Use clear, accessible language appropriate for the stated difficulty level.
- End with a concise **Summary** recapping the key takeaways.

Output *only* the lesson in Markdown. Do not repeat the curriculum plan.`,
		},
		// ── 3. Quiz Master ────────────────────────────────────────────────
		// CrewAI: quiz_creator_agent (Assessment Specialist)
		// Reads {curriculum_plan} + {lesson_content}; produces assessment.
		{
			ID:          "sub-quiz-master",
			Name:        "QuizMaster",
			Description: "Creates 5 quiz questions and a hands-on exercise from the curriculum plan and lesson content. Reads {curriculum_plan} and {lesson_content}.",
			Icon:        iconClipboardCheck,
			SystemPrompt: `You are a Quiz Master — an expert Assessment Specialist.

You receive two inputs produced by earlier agents:

**Curriculum Plan:**
{curriculum_plan}

**Lesson Content:**
{lesson_content}

**Your task:**
Create an **assessment section** in Markdown that tests the learner's understanding.

**Requirements:**
1. **5 Quiz Questions** — mix of:
   - Multiple choice (at least 2)
   - True / False (at least 1)
   - "What's wrong with this code?" (at least 1 — include a code snippet with a subtle bug)
   - Conceptual / short-answer (at least 1)
2. **1 Hands-On Exercise** (15-30 minutes):
   - Clear objective and acceptance criteria.
   - Starter code scaffold in a fenced code block.
   - A complete reference solution in a separate fenced code block (labelled "Solution").
3. **Answer Key** — correct answers for all 5 quiz questions at the end.

Output *only* the assessment material in Markdown. Do not repeat the lesson content.`,
		},
		// ── 4. Code Reviewer ───────────────────────────────────────────────
		// CrewAI: quality_assurance_agent (Senior Engineer)
		// Reads {lesson_content} + {temp:quiz_material}; validates all code.
		{
			ID:          "sub-code-reviewer-edu",
			Name:        "CodeReviewer",
			Description: "Reviews every code snippet in the lesson and quiz for correctness, best practices, and consistency. Reads {lesson_content} and {temp:quiz_material}.",
			Icon:        iconSearchCode,
			SystemPrompt: `You are a Code Reviewer — a Senior Software Engineer.

You receive two inputs from earlier agents:

**Lesson Content (with code examples):**
{lesson_content}

**Quiz & Exercise Material (with code snippets):**
{temp:quiz_material}

**Your task:**
Review **every code snippet** across both inputs.

**For each snippet, evaluate:**
1. **Correctness** — Does it compile / run without errors? Logic bugs?
2. **Best Practices** — Idiomatic style, proper error handling, naming conventions?
3. **Consistency** — Do examples use compatible library versions and import styles?
4. **Educational Value** — Does the code clearly demonstrate the concept it claims to?

**Output format (Markdown):**

` + fence + `
## Code Review Report

### Overall Assessment
<Approved / Needs Changes>

### Snippet Reviews
For each snippet that needs attention:
- **Location:** <section or question number>
- **Issue:** <description>
- **Suggested Fix:** <corrected code or guidance>

### Final Verdict
<"Production-ready" or "Needs revision" with summary>
` + fence + `

If all code is correct, state: "All code reviewed — production-ready. No issues found."
Output *only* the review report.`,
		},
	}

	// Sequential graph with OutputKey state passing — mirrors the CrewAI
	// task context chain:
	//   curriculum_task → content_task → quiz_task → review_task
	//
	// ADK-Go equivalent: each node's OutputKey saves its output into
	// session.State; downstream nodes read it via {variable} in Instruction.
	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			// CurriculumArchitect → saves plan for ContentWriter + QuizMaster.
			{"id": "n1", "type": "agent", "agent_id": "sub-curriculum-architect", "output_key": "curriculum_plan"},
			// ContentWriter → saves lesson for QuizMaster + CodeReviewer.
			{"id": "n2", "type": "agent", "agent_id": "sub-content-writer", "output_key": "lesson_content"},
			// QuizMaster → temp: prefix means state is discarded after pipeline.
			{"id": "n3", "type": "agent", "agent_id": "sub-quiz-master", "output_key": "temp:quiz_material"},
			// CodeReviewer → final output delivered to the user.
			{"id": "n4", "type": "agent", "agent_id": "sub-code-reviewer-edu", "output_key": "review_report"},
		},
	}

	return catalogWorkflowEntry(
		"Course Generation Pipeline",
		"Four-agent sequential pipeline that generates a complete course lesson. "+
			"Mirrors the CrewAI course-generator example: CurriculumArchitect → ContentWriter → QuizMaster → CodeReviewer. "+
			"Demonstrates ADK-Go's OutputKey → {variable} state passing across 4 agents in a sequentialagent.",
		"Generate a structured course lesson with curriculum plan, written content, assessments, and code review.",
		"Examples",
		[]string{"sequential", "state-passing", "output-key", "education", "course", "crew-ai", "adk-example"},
		iconBookOpen,
		true,
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
