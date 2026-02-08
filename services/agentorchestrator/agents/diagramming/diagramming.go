package diagramming

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/i18n"
	"school-with-ai/services/agentorchestrator/prompts"
)

// NOTE: Tools are now managed via permissions service instead of hardcoded domain tools
// The permissions service will determine which tools this agent can access

// NewDiagrammingInstructorConfig creates the diagramming specialist agent.
func NewDiagrammingInstructorConfig(ctx context.Context) (agent.Config, error) {
	// NOTE: Tools are now managed via permissions service - no hardcoded tool registration

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
		Tools:         []agent.Tool{}, // Tools managed via permissions service
		Capabilities: agent.Capabilities{
			InputElicitation: true,
		},
	}, nil
}
