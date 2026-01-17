package studentfacing

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
)

// domainTools returns the domain-specific tools for the student-facing agent.
func domainTools() []core.Tool {
	return []core.Tool{
		core.NewWriteTodosTool(),
	}
}

// CreateStudentFacingAgent creates the student-facing agent configuration.
func CreateStudentFacingAgent(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools (none for router agent)
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	prompt, err := core.GenerateSystemPrompt(core.PromptConfig{
		Role:        "Learning Coordinator",
		Preamble:    "You are the central routing agent for the school. Your primary responsibility is to efficiently connect students with the correct subject specialist.",
		Topic:       "School Administration and Learning Coordination",
		Interactive: true,
		Specialists: []core.Specialty{
			{Name: "math-instructor", Description: "ALL mathematics (algebra, calculus, trigonometry, derivatives, integrals, graphing, equations, plotting, statistics)"},
			{Name: "linearalgebra-instructor", Description: "Linear algebra ONLY (matrices, vectors, eigenvalues, transformations)"},
			{Name: "physics-instructor", Description: "Physics (mechanics, forces, energy, motion, thermodynamics)"},
			{Name: "chemistry-instructor", Description: "Chemistry (reactions, stoichiometry, molecules, bonding)"},
		},
		StudentProfile: &core.StudentProfile{
			Name:          "Jordan",
			LearningStyle: "Visual and Hands-on",
			Struggles:     []string{"Abstract Algebra", "Chemical Bonding Theory"},
			Strengths:     []string{"Mechanics", "Geometry", "Experimental setups"},
		},
		ExtraRules: `- **No Direct Teaching**: You do NOT answer academic questions yourself. Always delegate.
- **Minimal Clarification**: If a subject area is obvious, transfer IMMEDIATELY. Do NOT ask for clarification if the specialist can handle it.
- **Full Context**: Always include the student's original query and any context you gathered in the transfer call.`,
		IncludeTodos: true,
	})
	if err != nil {
		// This should only happen during development (template syntax error)
		panic(err)
	}

	return core.AgentConfig{
		Name:          "student-facing-agent",
		MaxIterations: 3,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
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
