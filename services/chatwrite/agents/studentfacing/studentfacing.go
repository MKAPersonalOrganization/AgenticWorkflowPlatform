package studentfacing

import (
	"school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
	"school-with-ai/sdk/agent-sdk/core"
)

// CreateStudentFacingAgent creates the student-facing agent that handles initial questions.
// Event contract is defined in contract.go
func CreateStudentFacingAgent(notifier sdk.EventNotifier) sdk.AgentConfig {
	registry := sdk.NewToolRegistry()

	// Domain-specific tools (none for router agent)
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Auto-generated emission tools
	for _, tool := range eventbus.GenerateEmissionTools(emits()) {
		registry.Register(tool)
	}

	// Shared collaboration tools
	registry.Register(tools.NewReadWhiteboardTool())
	registry.Register(tools.NewWriteWhiteboardTool())

	return sdk.AgentConfig{
		Name: "student-facing-agent",
		SystemPrompt: `Always read_whiteboard() first before any decision.

Route questions: write_whiteboard(append question) then emit_math_question OR emit_physics_question OR emit_chemistry_question OR emit_statistics_question

When answered: Present answer to user. Only emit another question if original request incomplete.`,
		Tools:    registry.All(),
		Notifier: notifier,
	}
}
