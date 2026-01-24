package diagramming

import (
	"context"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/i18n"
	"school-with-ai/services/agentorchestrator/agents/diagramming/tools"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the diagramming instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		tools.NewMermaidDiagramTool(),
		core.NewWriteTodosTool(),
	}
}

// CreateDiagrammingInstructor creates the diagramming specialist agent configuration.
// Event contract is defined in contract.go.
func CreateDiagrammingInstructor(ctx context.Context, notifier core.EventNotifier) core.AgentConfig {
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
		Role:         i18n.T(ctx, "diagramming.role"),
		Preamble:     i18n.T(ctx, "diagramming.preamble"),
		Topic:        i18n.T(ctx, "diagramming.topic"),
		Interactive:  true,
		ExtraRules:   i18n.T(ctx, "diagramming.extra_rules"),
		IncludeTodos: true,
	})
	if err != nil {
		panic(err)
	}

	return core.AgentConfig{
		Name:          "diagramming-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Notifier:      notifier,
		Middleware: []core.Middleware{
			middleware.NewElicitationMiddleware(),
		},
	}
}
