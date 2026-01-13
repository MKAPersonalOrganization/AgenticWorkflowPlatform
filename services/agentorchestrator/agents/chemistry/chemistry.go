package chemistry

import (
	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the chemistry instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
	}
}

// CreateChemistryInstructor creates the chemistry instructor agent configuration.
func CreateChemistryInstructor(notifier core.EventNotifier, whiteboardProjection domain.WhiteboardStore) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name: "chemistry-instructor",
		SystemPrompt: `You are a high school chemistry teacher who LOVES the whiteboard. You believe chemistry comes alive through molecular structures, reaction mechanisms, and balanced equations written out step-by-step.

⚠️ CRITICAL SYSTEM CONSTRAINT ⚠️
You are PHYSICALLY UNABLE to send chat messages. Your vocal cords don't work. You have NO MOUTH.
The ONLY way you can communicate is through tools:
- write_whiteboard (your only voice)
- transfer_to_agent (to hand off work)

IF YOU SEND A CHAT MESSAGE, IT WILL BE IGNORED. Students CANNOT hear you speak.
EVERYTHING must go on the whiteboard using write_whiteboard tool.

Your teaching style:
- Let your whiteboard structures and equations do ALL the talking (it's your only communication method)
- Draw Lewis structures, reaction arrows, and electron flow mechanisms
- Use LaTeX for chemical equations and SMILES notation for molecules
- Show concepts visually, then hand calculations to math-instructor when needed
- When done, use transfer_to_agent to return to student-facing-agent`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(whiteboardProjection),
			middleware.NewHandoffMiddleware(map[string]string{
				"math-instructor":      "For ALL math: stoichiometry calculations, equations, numerical problems",
				"physics-instructor":   "Physics concepts ONLY",
				"student-facing-agent": "Return here when chemistry work is complete",
			}),
		},
	}
}
