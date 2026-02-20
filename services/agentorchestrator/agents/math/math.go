package math

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/i18n"
	"school-with-ai/services/agentorchestrator/prompts"
	"school-with-ai/services/artifact"
)

// NOTE: Tools are now managed via permissions service instead of hardcoded domain tools
// The permissions service will determine which tools this agent can access

// NewMathInstructorConfig creates the math instructor configuration.
func NewMathInstructorConfig(ctx context.Context, artifactStore artifact.Service) (agent.Config, error) {
	// NOTE: Tools are now managed via permissions service - no hardcoded tool registration

	prompt, err := prompts.GenerateTeacherSystemPrompt(prompts.TeacherPromptConfig{
		Role:     i18n.T(ctx, "math.role"),
		Preamble: i18n.T(ctx, "math.preamble"),
		Topic:    i18n.T(ctx, "math.topic"),
		Mission:  i18n.T(ctx, "math.mission"),
		Objectives: []string{
			i18n.T(ctx, "math.objective1"),
			i18n.T(ctx, "math.objective2"),
			i18n.T(ctx, "math.objective3"),
			i18n.T(ctx, "math.objective4"),
		},
		LearningPath: &prompts.LearningPath{
			Prerequisites: []string{
				i18n.T(ctx, "math.prerequisite1"),
				i18n.T(ctx, "math.prerequisite2"),
				i18n.T(ctx, "math.prerequisite3"),
				i18n.T(ctx, "math.prerequisite4"),
			},
			NextConcepts: []string{
				i18n.T(ctx, "math.next_concept1"),
				i18n.T(ctx, "math.next_concept2"),
				i18n.T(ctx, "math.next_concept3"),
			},
			RelatedTopics: []string{
				i18n.T(ctx, "math.related_topic1"),
				i18n.T(ctx, "math.related_topic2"),
				i18n.T(ctx, "math.related_topic3"),
			},
			PracticeAreas: []string{
				i18n.T(ctx, "math.practice_area1"),
				i18n.T(ctx, "math.practice_area2"),
				i18n.T(ctx, "math.practice_area3"),
				i18n.T(ctx, "math.practice_area4"),
			},
		},
		Interactive: true,
		ExtraRules:  i18n.T(ctx, "math.extra_rules"),
	})
	if err != nil {
		return agent.Config{}, err
	}

	return agent.Config{
		Name:          "math-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         []agent.Tool{}, // Tools managed via permissions service
		ChatMiddlewares: []agent.Middleware{
			agent.RequirementMiddleware(
				agent.NewConditionalRequirement("think", agent.ForceAtStep(1)),
			),
		},
		Capabilities: agent.Capabilities{
			InputElicitation: true,
		},
	}, nil
}
