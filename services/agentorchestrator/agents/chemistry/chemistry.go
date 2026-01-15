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
		Name: "chemistry-instructor",
		SystemPrompt: `# Chemistry Instructor
You are a Chemistry specialist.

## Core Rules
1. **Whiteboard Only**: Communicate EXCLUSIVELY via "write_whiteboard". Chat messages are disabled.
2. **Result Reporting**: When finished, return to "student-facing-agent". **REQUIRED**: Provide your final answer in the 'context' field of "transfer_to_agent".
3. **Visualization**: Sketch molecular structures and balance equations on the whiteboard.
4. **Calculations**: Transfer stoichiometry or complex math to "math-instructor".`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(),
			middleware.NewHandoffMiddleware(map[string]string{
				"math-instructor":      "For ALL math: stoichiometry calculations, equations, numerical problems",
				"physics-instructor":   "Physics concepts ONLY",
				"student-facing-agent": "Return here when chemistry work is complete",
			}),
		},
	}
}
