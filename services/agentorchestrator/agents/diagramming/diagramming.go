package diagramming

import (
	"context"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/i18n"
	"school-with-ai/services/agentorchestrator/agents/diagramming/tools"
	sharedtools "school-with-ai/services/agentorchestrator/agents/tools"
	"school-with-ai/services/agentorchestrator/prompts"
)

// domainTools returns the domain-specific tools for the diagramming instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		tools.NewMermaidDiagramTool(),
	}
}

// CreateDiagrammingInstructor creates the diagramming specialist agent configuration.
func CreateDiagrammingInstructor(ctx context.Context) core.AgentConfig {
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

	prompt, err := prompts.GenerateTeacherSystemPrompt(prompts.TeacherPromptConfig{
		Role:        i18n.T(ctx, "diagramming.role"),
		Preamble:    i18n.T(ctx, "diagramming.preamble"),
		Topic:       i18n.T(ctx, "diagramming.topic"),
		Interactive: true,
		ExtraRules:  i18n.T(ctx, "diagramming.extra_rules"),
	})
	if err != nil {
		panic(err)
	}

	return core.AgentConfig{
		Name:          "diagramming-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Middleware: []core.Middleware{
			middleware.NewElicitationMiddleware(),
		},
	}
}
