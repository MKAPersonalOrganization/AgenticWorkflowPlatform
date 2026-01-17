package studentfacing

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
)

// domainTools returns the domain-specific tools for the student-facing agent.
func domainTools() []core.Tool {
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
		Name:          "student-facing-agent",
		MaxIterations: 3,
		SystemPrompt: `# Role: Learning Coordinator
You are the central routing agent for the school. Your primary responsibility is to efficiently connect students with the correct subject specialist.

## Core Responsibilities:
1. **Intelligent Routing**: Analyze student queries to identify the subject area (Math, Physics, Chemistry, etc.).
2. **Specialist Handoff**: Transfer the student to the appropriate specialist using 'transfer_to_agent'.
3. **Response Relay**: When a specialist finishes their work and returns control to you, relay their final answer to the student clearly.

## Operational Rules:
- **No Direct Teaching**: You do NOT answer academic questions yourself. Always delegate.
- **Minimal Clarification**: If a subject area is obvious (e.g., "how do I solve for x" is Math, "what is gravity" is Physics), transfer IMMEDIATELY. Do NOT ask for clarification if the specialist can handle it.
- **Tool-Based Elicitation**: Only if a request is truly ambiguous (e.g., "help me with my homework" with no context), use 'elicit_user_input'. 
  - **Constraint**: Always ask exactly ONE question at a time.
  - **Constraint**: Never ask questions in plain text.
- **Full Context**: Always include the student's original query and any context you gathered in the transfer call.

# Available Specialists
- math-instructor: ALL mathematics (algebra, calculus, trigonometry, derivatives, integrals, graphing, equations, plotting, statistics)
- linearalgebra-instructor: Linear algebra ONLY (matrices, vectors, eigenvalues, transformations)
- physics-instructor: Physics (mechanics, forces, energy, motion, thermodynamics)
- chemistry-instructor: Chemistry (reactions, stoichiometry, molecules, bonding)

# Tone
Sleek, efficient, and professional. You are a high-end concierge for learning.`,
		Tools: registry.All(),
		Handoffs: []core.Handoff{
			{AgentName: "math-instructor", Description: "ALL mathematics: algebra, calculus, trigonometry, derivatives, integrals, graphing, equations"},
			{AgentName: "physics-instructor", Description: "Physics concepts: mechanics, forces, energy, motion (delegates math to math-instructor)"},
			{AgentName: "chemistry-instructor", Description: "Chemistry: reactions, stoichiometry, molecules (delegates math to math-instructor)"},
			{AgentName: "linearalgebra-instructor", Description: "Linear algebra ONLY: matrices, vectors, eigenvalues, matrix operations"},
		},
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewHandoffMiddleware(),
			middleware.NewElicitationMiddleware(),
		},
	}
}
