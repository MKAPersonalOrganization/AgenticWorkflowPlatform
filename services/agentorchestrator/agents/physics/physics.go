package physics

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the physics instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		core.NewWriteTodosTool(),
	}
}

// CreatePhysicsInstructor creates the physics instructor agent configuration.
func CreatePhysicsInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name:          "physics-instructor",
		MaxIterations: 5,
		SystemPrompt: `# Physics Instructor
You are a Physics specialist.

## Core Rules
1. **Subject Authority**: You are the SOLE authority for Physics. Never ask the 'student-facing-agent' or other instructors for help with physics.
2. **Chat for Teaching**: Use chat messages for physical intuition and explanations.
3. **Elicitation**: You MUST use the "elicit_user_input" tool for ALL questions. **Always ask exactly ONE question at a time.** Never ask multiple questions. **NO LISTS**: Even for quizzes or exercises, ask one question, wait for the answer, then ask the next.
4. **LaTeX**: Use LaTeX for all mathematical expressions and units (e.g., $F = ma$, $9.8 m/s^2$).
5. **Result Reporting**: When finished, return to "student-facing-agent". **REQUIRED**: Provide your final answer in the 'context' field of "transfer_to_agent".
6. **Calculations**: Transfer math-heavy work to "math-instructor" if needed.

` + core.WriteTodosSystemPrompt,
		Tools: registry.All(),
		Handoffs: []core.Handoff{
			{AgentName: "math-instructor", Description: "For ALL math: calculus, derivatives, integrals, equations, numerical calculations"},
			{AgentName: "chemistry-instructor", Description: "Chemistry concepts ONLY"},
			{AgentName: "student-facing-agent", Description: "Return here when physics work is complete"},
		},
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewHandoffMiddleware(),
			middleware.NewElicitationMiddleware(),
		},
	}
}
