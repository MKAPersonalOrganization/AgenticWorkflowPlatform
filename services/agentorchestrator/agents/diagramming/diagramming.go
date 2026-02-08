package diagramming

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/i18n"
	"school-with-ai/services/agentorchestrator/agents/diagramming/tools"
	"school-with-ai/services/agentorchestrator/prompts"
)

// domainTools returns the domain-specific tools for the diagramming instructor.
func domainTools() []agent.Tool {
	return []agent.Tool{
		tools.NewMermaidDiagramTool(),
	}
}

// NewDiagrammingInstructorConfig creates the diagramming specialist agent.
func NewDiagrammingInstructorConfig(ctx context.Context) (agent.Config, error) {
	registry := agent.NewToolSet()

	// Domain-specific tools
	registry.AddAll(domainTools())

	prompt, err := prompts.GenerateTeacherSystemPrompt(prompts.TeacherPromptConfig{
		Role:        i18n.T(ctx, "diagramming.role"),
		Preamble:    i18n.T(ctx, "diagramming.preamble"),
		Topic:       i18n.T(ctx, "diagramming.topic"),
		Interactive: true,
		ExtraRules:  i18n.T(ctx, "diagramming.extra_rules"),
	})
	if err != nil {
		return agent.Config{}, err
	}

	return agent.Config{
		Name:          "diagramming-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Capabilities: agent.Capabilities{
			InputElicitation: true,
		},
	}, nil
}
