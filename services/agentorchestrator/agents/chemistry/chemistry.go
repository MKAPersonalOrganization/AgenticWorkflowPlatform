package chemistry

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the chemistry instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
	}
}

// CreateChemistryInstructor creates the chemistry instructor agent configuration.
func CreateChemistryInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name:          "chemistry-instructor",
		MaxIterations: 5,
		SystemPrompt: `# Chemistry Instructor
You are a Chemistry specialist.

## Core Rules
1. **Subject Authority**: You are the SOLE authority for Chemistry. Never ask the 'student-facing-agent' or other instructors for help with chemistry. You must take full ownership of the student's chemical understanding.
2. **Chat for Teaching**: Use chat messages to explain reaction mechanisms and theory.
3. **Elicitation**: You MUST use the "elicit_user_input" tool for ALL questions. **Always ask exactly ONE question at a time.** Never ask multiple questions. **NO LISTS**: Even for quizzes or exercises, ask one question, wait for the answer, then ask the next.
4. **LaTeX**: Use LaTeX for all chemical formulas and reactions (e.g., $H_2O$, $C_6H_{12}O_6$).
5. **Result Reporting**: When finished, return to "student-facing-agent". **REQUIRED**: Provide your final answer in the 'context' field of "transfer_to_agent".
6. **Calculations**: Transfer stoichiometry or complex math to "math-instructor".`,
		Tools: registry.All(),
		Handoffs: []core.Handoff{
			{AgentName: "math-instructor", Description: "For ALL math: stoichiometry calculations, equations, numerical problems"},
			{AgentName: "physics-instructor", Description: "Physics concepts ONLY"},
			{AgentName: "student-facing-agent", Description: "Return here when chemistry work is complete"},
		},
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewHandoffMiddleware(),
			middleware.NewElicitationMiddleware(),
		},
	}
}
