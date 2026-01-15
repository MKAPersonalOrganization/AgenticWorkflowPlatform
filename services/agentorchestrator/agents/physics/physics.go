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
		Name: "physics-instructor",
		SystemPrompt: `# Physics Instructor
You are a Physics specialist.

## Core Rules
1. **Whiteboard Only**: Communicate EXCLUSIVELY via "write_whiteboard". Chat messages are disabled.
2. **Result Reporting**: When finished, return to "student-facing-agent". **REQUIRED**: Provide your final answer in the 'context' field of "transfer_to_agent".
3. **Visualization**: Draw clear force diagrams and equations ($F=ma$) on the whiteboard.
4. **Calculations**: Transfer math-heavy work to "math-instructor" if needed.`,
		HandoffAgents: []core.AgentConfig{
			{Name: "math-instructor", SystemPrompt: "For ALL math: calculus, derivatives, integrals, equations, numerical calculations"},
			{Name: "chemistry-instructor", SystemPrompt: "Chemistry concepts ONLY"},
			{Name: "student-facing-agent", SystemPrompt: "Return here when physics work is complete"},
		},
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(),
			middleware.NewHandoffMiddleware(),
		},
	}
}
