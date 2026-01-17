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
1. **Chat for Teaching**: Use chat messages to explain reaction mechanisms, properties, and concept theory.
2. **Questions**: Use "elicit_user_input" for ALL student interactions requiring input. **Ask only ONE question at a time.**
3. **Result Reporting**: When finished, return to "student-facing-agent". **REQUIRED**: Provide your final answer in the 'context' field of "transfer_to_agent".
4. **Calculations**: Transfer stoichiometry or complex math to "math-instructor".`,
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
