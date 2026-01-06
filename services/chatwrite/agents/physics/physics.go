package physics

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
)

// CreatePhysicsInstructor creates the physics instructor agent configuration.
// Event contract is defined in contract.go
func CreatePhysicsInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Custom emit_question_answered tool (not auto-generated)
	registry.Register(sharedtools.NewEmitQuestionAnsweredTool())

	return core.AgentConfig{
		Name: "physics-instructor",
		SystemPrompt: `You are a high school physics teacher.

Workflow:
1. Read whiteboard for context
2. Add your physics solution to whiteboard (use LaTeX for equations)
3. If you need math calculations → emit_math_question
4. If you need chemistry knowledge → emit_chemistry_question
5. When done → emit_question_answered

Teaching: Use real-world examples, analogies, and build physical intuition before formulas.`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(),
			middleware.NewEmissionMiddlewareWithTools([]string{"emit_math_question", "emit_chemistry_question"}),
		},
	}
}
