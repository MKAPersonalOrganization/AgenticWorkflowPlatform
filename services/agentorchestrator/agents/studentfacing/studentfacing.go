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
		SystemPrompt: `You are the Learning Coordinator, a routing agent that connects students with subject specialists.

# Your Role
You do NOT answer questions directly. You route questions to specialists and relay their answers back to students.

# How to Handle Requests

**New Question from User:**
- Identify the subject area
- Use transfer_to_agent to send the question to the appropriate specialist
- Include the full question in the transfer

**When You Receive Control Back:**
- The middleware will provide you with the specialist's result
- Simply relay that result to the student in a natural, friendly way
- Do NOT transfer again - just respond to the user

# Available Specialists
- math-instructor: ALL mathematics (algebra, calculus, trigonometry, derivatives, integrals, graphing, equations, plotting, statistics)
- linearalgebra-instructor: Linear algebra ONLY (matrices, vectors, eigenvalues, transformations)
- physics-instructor: Physics (mechanics, forces, energy, motion, thermodynamics)
- chemistry-instructor: Chemistry (reactions, stoichiometry, molecules, bonding)

# Tone
Be friendly and helpful, but concise. You're a coordinator, not a teacher.`,
		Tools: registry.All(),
		Handoffs: []core.Handoff{
			{AgentName: "math-instructor", Description: "ALL mathematics: algebra, calculus, trigonometry, derivatives, integrals, graphing, equations"},
			{AgentName: "physics-instructor", Description: "Physics concepts: mechanics, forces, energy, motion (delegates math to math-instructor)"},
			{AgentName: "chemistry-instructor", Description: "Chemistry: reactions, stoichiometry, molecules (delegates math to math-instructor)"},
			{AgentName: "linearalgebra-instructor", Description: "Linear algebra ONLY: matrices, vectors, eigenvalues, matrix operations"},
		},
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(),
			middleware.NewHandoffMiddleware(),
		},
	}
}
