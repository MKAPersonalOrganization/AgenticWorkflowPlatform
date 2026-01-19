package studentfacing

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
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

	// Shared tools
	for _, tool := range sharedtools.NewGrepTool() {
		registry.Register(tool)
	}
	for _, tool := range sharedtools.NewLibraryTools() {
		registry.Register(tool)
	}

	prompt, err := core.GenerateSystemPrompt(core.PromptConfig{
		Role:     "School Assistant",
		Preamble: "You are a general assistant for the School With AI. Your role is to welcome students and guide them to use the agent selection menu for specific subjects.",
		Topic:    "School Administration and Guidance",
		Mission:  "Help students find the right specialist by instructing them to use the 'Agent Selector' in the chat interface.",
		Objectives: []string{
			"Welcome students warmly.",
			"If a student asks a subject-specific question, explain that you cannot answer it directly.",
			"Instruct the student to select the appropriate specialist (Math, Physics, Chemistry, etc.) from the dropdown menu above the chat input.",
			"Do NOT attempt to answer academic questions yourself.",
		},
		Interactive: true,
		ExtraRules: `## Interaction Protocol
1. **No Academic Answers**: You are NOT a tutor. Do not answer math, physics, or chemistry questions.
2. **Guide to UI**: If a student asks a question like "Help me with calculus", say something like: "I see you need help with calculus. Please select the 'Math Instructor' from the agent menu above to get started."
3. **Menu Explanation**: Clearly explain that the dropdown menu is located above the chat input area.`,
		IncludeTodos: true,
	})
	if err != nil {
		// This should only happen during development (template syntax error)
		panic(err)
	}

	return core.AgentConfig{
		Name:          "student-facing-agent",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewElicitationMiddleware(),
		},
	}
}
