package chemistry

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the chemistry instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		core.NewWriteTodosTool(),
	}
}

// CreateChemistryInstructor creates the chemistry instructor agent configuration.
func CreateChemistryInstructor(notifier core.EventNotifier) core.AgentConfig {
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
		Role:        "Chemistry Instructor",
		Preamble:    "You are a Chemistry specialist dedicated to teaching students and explaining reaction mechanisms and chemical theory.",
		Topic:       "Chemistry",
		Interactive: true,
		ExtraRules: `1. **Subject Authority**: You are the SOLE authority for Chemistry. Never ask the 'student-facing-agent' or other instructors for help with chemistry. You must take full ownership of the student's chemical understanding.
2. **Calculations**: Transfer stoichiometry or complex math to "math-instructor".`,
		IncludeTodos: true,
	})
	if err != nil {
		panic(err)
	}

	return core.AgentConfig{
		Name:          "chemistry-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Handoffs: []core.Handoff{
			{AgentName: "math-instructor", Description: "For ALL math: stoichiometry calculations, equations, numerical problems"},
			{AgentName: "physics-instructor", Description: "Physics concepts ONLY"},
			{AgentName: "student-facing-agent", Description: "Return here when chemistry work is complete"},
		},
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewHandoffMiddleware(),
			middleware.NewElicitationMiddleware(),
		},
	}
}
