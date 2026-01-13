package studentfacing

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
)

// domainTools returns the domain-specific tools for the student-facing agent.
func domainTools() []core.Tool {
	// No domain-specific tools - this agent only routes
	return []core.Tool{}
}

// CreateStudentFacingAgent creates the student-facing agent configuration.
func CreateStudentFacingAgent(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools (none for router agent)
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name: "student-facing-agent",
		SystemPrompt: `You are the Learning Coordinator. Your role is to route student questions to specialists and report results.

PROTOCOL:
1. **Routing**: For any new question, identify the subject and use "transfer_to_agent" to send it to a specialist.
2. **Result Recovery**: When a specialist returns control:
   - READ the 'context' field of their "transfer_to_agent" call (this is the final answer).
   - Report this answer to the student directly.
   - Use a helpful tone: "The instructor has finished. [Result]. Do you have any other questions?"
3. **Loop Prevention**: Do NOT re-transfer a question if it has already been answered by a specialist in the history.

SPECIALISTS:
- math-instructor: Arithmetic, Algebra, Calculus.
- linearalgebra-instructor: Matrices, Vectors.
- physics-instructor: Forces, Energy, Motion.
- chemistry-instructor: Reactions, Molecules.`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(),
			middleware.NewHandoffMiddleware(map[string]string{
				"math-instructor":          "ALL mathematics: algebra, calculus, trigonometry, derivatives, integrals, graphing, equations",
				"physics-instructor":       "Physics concepts: mechanics, forces, energy, motion (delegates math to math-instructor)",
				"chemistry-instructor":     "Chemistry: reactions, stoichiometry, molecules (delegates math to math-instructor)",
				"linearalgebra-instructor": "Linear algebra ONLY: matrices, vectors, eigenvalues, matrix operations",
			}),
		},
	}
}
