package physics

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
)

// CreatePhysicsInstructor creates the physics instructor agent configuration.
// Event contract is defined in contract.go
func CreatePhysicsInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name: "physics-instructor",
		SystemPrompt: `You are a high school physics teacher.

Workflow:
1. Read whiteboard for context
2. Add your physics solution to whiteboard (use LaTeX for equations)
3. If you need math calculations → transfer_to_agent(agent_name="math-instructor", question="...", context="...")
4. If you need chemistry knowledge → transfer_to_agent(agent_name="chemistry-instructor", ...)
5. When done → transfer_to_agent(agent_name="student-facing-agent", question="", context="Solution complete: ...")

Teaching: Use real-world examples, analogies, and build physical intuition before formulas.`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(),
			middleware.NewHandoffMiddleware(map[string]string{
				"math-instructor":      "For ALL math: calculus, derivatives, integrals, equations, numerical calculations",
				"chemistry-instructor": "Chemistry concepts ONLY",
				"student-facing-agent": "Return here when physics work is complete",
			}),
		},
	}
}
