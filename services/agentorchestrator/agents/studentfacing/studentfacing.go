package studentfacing

import (
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
)

// domainTools returns the domain-specific tools for the student-facing agent.
func domainTools() []core.Tool {
	// No domain-specific tools - this agent only routes
	return []core.Tool{}
}

// CreateStudentFacingAgent creates the student-facing agent configuration.
func CreateStudentFacingAgent(notifier core.EventNotifier, whiteboardProjection domain.WhiteboardStore) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools (none for router agent)
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name: "student-facing-agent",
		SystemPrompt: `You are a coordinator that routes student questions to specialist instructors.

CRITICAL RULE: You NEVER answer questions yourself. You are NOT a teacher. Your ONLY job is to immediately transfer questions to the appropriate specialist.

When a student asks a question:
1. Identify the subject (math, physics, chemistry, linear algebra)
2. IMMEDIATELY use transfer_to_agent to send it to the specialist
3. Do NOT attempt to answer, explain, or provide information yourself

You are a router, not a responder.`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewTodoListMiddleware(),
			middleware.NewWhiteboardMiddleware(whiteboardProjection),
			middleware.NewHandoffMiddleware(map[string]string{
				"math-instructor":          "ALL mathematics: algebra, calculus, trigonometry, derivatives, integrals, graphing, equations",
				"physics-instructor":       "Physics concepts: mechanics, forces, energy, motion (delegates math to math-instructor)",
				"chemistry-instructor":     "Chemistry: reactions, stoichiometry, molecules (delegates math to math-instructor)",
				"linearalgebra-instructor": "Linear algebra ONLY: matrices, vectors, eigenvalues, matrix operations",
			}),
		},
	}
}
