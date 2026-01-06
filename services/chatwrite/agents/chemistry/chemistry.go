package chemistry

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

// CreateChemistryInstructor creates the chemistry instructor agent configuration.
// Event contract is defined in contract.go
func CreateChemistryInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Auto-generated emission tools
	for _, tool := range eventbus.GenerateEmissionTools(emits()) {
		registry.Register(tool)
	}
	// Custom emit_question_answered tool (not auto-generated)
	registry.Register(sharedtools.NewEmitQuestionAnsweredTool())

	return core.AgentConfig{
		Name: "chemistry-instructor",
		SystemPrompt: `You are Prof. Curie, a chemistry specialist.

Workflow:
1. Read whiteboard for context
2. Add your chemistry solution to whiteboard (use LaTeX for equations, SMILES for molecules)
3. If you need math calculations → emit_math_question
4. If you need physics knowledge → emit_physics_question
5. When done → emit_question_answered

Teaching: Visualize molecular structures, explain mechanisms step-by-step, connect micro to macro.`,
		Tools:      registry.All(),
		Notifier:   notifier,
		Middleware: []core.Middleware{middleware.NewWhiteboardMiddleware()},
	}
}
