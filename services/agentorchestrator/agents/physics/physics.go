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
		core.NewWriteTodosTool(),
	}
}

// CreatePhysicsInstructor creates the physics instructor agent configuration.
func CreatePhysicsInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Shared tools
	for _, tool := range sharedtools.NewGrepTool() {
		registry.Register(tool)
	}
	for _, tool := range sharedtools.NewLibraryTools() {
		registry.Register(tool)
	}

	prompt, err := core.GenerateSystemPrompt(core.PromptConfig{
		Role:        "Physics Instructor",
		Preamble:    "You are a Physics specialist dedicated to teaching students and solving complex physics problems.",
		Topic:       "Physics",
		Interactive: true,
		ExtraRules: `1. **Subject Authority**: You are the SOLE authority for Physics. Never ask the 'student-facing-agent' or other instructors for help with physics.
2. **Calculations**: Transfer math-heavy work to "math-instructor" if needed.
3. **Physical Intuition**: Focus your chat messages on physical intuition and conceptual explanations.`,
		IncludeTodos: true,
	})
	if err != nil {
		panic(err)
	}

	return core.AgentConfig{
		Name:          "physics-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Handoffs: []core.Handoff{
			{AgentName: "math-instructor", Description: "For ALL math: calculus, derivatives, integrals, equations, numerical calculations"},
			{AgentName: "chemistry-instructor", Description: "Chemistry concepts ONLY"},
			{AgentName: "student-facing-agent", Description: "Return here when physics work is complete"},
		},
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewHandoffMiddleware(),
			middleware.NewElicitationMiddleware(),
		},
	}
}
