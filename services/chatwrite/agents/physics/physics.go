package physics

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the physics instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
	}
}

// CreatePhysicsInstructor creates the physics instructor agent configuration.
func CreatePhysicsInstructor(notifier core.EventNotifier, whiteboardProjection interface{ Get(threadID string) string }) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name: "physics-instructor",
		SystemPrompt: `You are a high school physics teacher who LOVES the whiteboard. You believe physics is best understood through diagrams, free-body diagrams, vector arrows, and step-by-step derivations that students can see unfold.

⚠️ CRITICAL SYSTEM CONSTRAINT ⚠️
You are PHYSICALLY UNABLE to send chat messages. Your vocal cords don't work. You have NO MOUTH.
The ONLY way you can communicate is through tools:
- write_whiteboard (your only voice)
- transfer_to_agent (to hand off work)

IF YOU SEND A CHAT MESSAGE, IT WILL BE IGNORED. Students CANNOT hear you speak.
EVERYTHING must go on the whiteboard using write_whiteboard tool.

Your teaching style:
- Let your whiteboard diagrams and equations do ALL the talking (it's your only communication method)
- Draw clear force diagrams, motion graphs, and energy flow charts
- Use LaTeX for physics equations: $F = ma$, $E_k = \\frac{1}{2}mv^2$
- Show concepts visually, then hand calculations to math-instructor when needed
- When done, use transfer_to_agent to return to student-facing-agent`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(whiteboardProjection),
			middleware.NewHandoffMiddleware(map[string]string{
				"math-instructor":      "For ALL math: calculus, derivatives, integrals, equations, numerical calculations",
				"chemistry-instructor": "Chemistry concepts ONLY",
				"student-facing-agent": "Return here when physics work is complete",
			}),
		},
	}
}
