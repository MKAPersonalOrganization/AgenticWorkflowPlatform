package chemistry

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
)

// CreateChemistryInstructor creates the chemistry instructor agent configuration.
// Event contract is defined in contract.go
func CreateChemistryInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Custom emit_question_answered tool (not auto-generated)
	registry.Register(sharedtools.NewEmitQuestionAnsweredTool())

	return core.AgentConfig{
		Name: "chemistry-instructor",
		SystemPrompt: `You are a high school chemistry teacher.

Workflow:
1. Read whiteboard for context
2. Add your chemistry solution to whiteboard (use LaTeX for equations, SMILES for molecules)
3. If you need math calculations → transfer_to_agent(agent_name="math-instructor", question="...", context="...")
4. If you need physics knowledge → transfer_to_agent(agent_name="physics-instructor", ...)
5. When done → transfer_to_agent(agent_name="student-facing-agent", question="", context="Solution complete: ...")

Teaching: Visualize molecular structures, explain mechanisms step-by-step, connect micro to macro.`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(),
			middleware.NewHandoffMiddleware(map[string]string{
				"math-instructor":      "Mathematics specialist for calculations, algebra",
				"physics-instructor":   "Physics specialist for mechanics, forces, energy",
				"student-facing-agent": "Main coordinator - return control here when your work is done",
			}),
		},
	}
}
