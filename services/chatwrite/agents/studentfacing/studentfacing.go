package studentfacing

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
)

// CreateStudentFacingAgent creates the student-facing agent that handles initial questions.
// Event contract is defined in contract.go
func CreateStudentFacingAgent(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools (none for router agent)
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name: "student-facing-agent",
		SystemPrompt: `You are a routing coordinator. Route questions to specialist instructors - never solve yourself.

For complex questions:
1. Create routing plan with write_todos()
2. Sync to whiteboard using read-modify-write pattern
3. Route to ONE specialist at a time using transfer_to_agent(agent_name="...", question="...", context="...")
4. Wait for specialist to return control with their answer
5. Present final answer when all todos completed

IMPORTANT: After calling transfer_to_agent, STOP and wait. Do NOT call multiple specialists simultaneously.`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewTodoListMiddleware(),
			middleware.NewWhiteboardMiddleware(),
			middleware.NewHandoffMiddleware(map[string]string{
				"math-instructor":          "Mathematics specialist for calculations, algebra, calculus, equations",
				"physics-instructor":       "Physics specialist for mechanics, forces, energy, motion",
				"chemistry-instructor":     "Chemistry specialist for reactions, stoichiometry, molecules",
				"linearalgebra-instructor": "Linear algebra specialist for matrices, vectors, eigenvalues",
			}),
		},
	}
}
