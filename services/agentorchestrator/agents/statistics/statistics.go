package statistics

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/i18n"
	"school-with-ai/services/agentorchestrator/prompts"
)

// NOTE: Tools are now managed via permissions service instead of hardcoded domain tools
// The permissions service will determine which tools this agent can access

// NewStatisticsInstructorConfig creates the statistics instructor configuration.
func NewStatisticsInstructorConfig(ctx context.Context) (agent.Config, error) {
	// NOTE: Tools are now managed via permissions service - no hardcoded tool registration

	prompt, err := prompts.GenerateTeacherSystemPrompt(prompts.TeacherPromptConfig{
		Role:     i18n.T(ctx, "statistics.role"),
		Preamble: i18n.T(ctx, "statistics.preamble"),
		Topic:    i18n.T(ctx, "statistics.topic"),
		Mission:  i18n.T(ctx, "statistics.mission"),
		Objectives: []string{
			i18n.T(ctx, "statistics.objective1"),
			i18n.T(ctx, "statistics.objective2"),
			i18n.T(ctx, "statistics.objective3"),
			i18n.T(ctx, "statistics.objective4"),
		},
		LearningPath: &prompts.LearningPath{
			Prerequisites: []string{
				i18n.T(ctx, "statistics.prerequisite1"),
				i18n.T(ctx, "statistics.prerequisite2"),
				i18n.T(ctx, "statistics.prerequisite3"),
				i18n.T(ctx, "statistics.prerequisite4"),
			},
			NextConcepts: []string{
				i18n.T(ctx, "statistics.next_concept1"),
				i18n.T(ctx, "statistics.next_concept2"),
				i18n.T(ctx, "statistics.next_concept3"),
				i18n.T(ctx, "statistics.next_concept4"),
			},
			RelatedTopics: []string{
				i18n.T(ctx, "statistics.related_topic1"),
				i18n.T(ctx, "statistics.related_topic2"),
				i18n.T(ctx, "statistics.related_topic3"),
				i18n.T(ctx, "statistics.related_topic4"),
			},
			PracticeAreas: []string{
				i18n.T(ctx, "statistics.practice_area1"),
				i18n.T(ctx, "statistics.practice_area2"),
				i18n.T(ctx, "statistics.practice_area3"),
				i18n.T(ctx, "statistics.practice_area4"),
			},
		},
		Interactive: true,
		ExtraRules:  i18n.T(ctx, "statistics.extra_rules"),
	})
	if err != nil {
		return agent.Config{}, err
	}

	return agent.Config{
		Name:          "statistics-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         []agent.Tool{}, // Tools managed via permissions service

		// Declarative requirements: the framework auto-wires tools, prompts, and middleware.
		Requirements: []agent.Requirement{
			agent.NewRequireUserInput(),
		},
	}, nil
}
