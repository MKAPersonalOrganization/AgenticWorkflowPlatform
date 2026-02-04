package chemistry

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/i18n"
	sharedtools "school-with-ai/services/agentorchestrator/agents/tools"
	"school-with-ai/services/agentorchestrator/prompts"
)

// domainTools returns the domain-specific tools for the chemistry instructor.
func domainTools() []agent.Tool {
	return []agent.Tool{
		&sharedtools.AddTool{},
		&sharedtools.SubtractTool{},
		&sharedtools.MultiplyTool{},
		&sharedtools.DivideTool{},
		&sharedtools.PowerTool{},
	}
}

// NewChemistryInstructorConfig creates the chemistry instructor configuration.
func NewChemistryInstructorConfig(ctx context.Context) (agent.Config, error) {
	registry := agent.NewToolSet()

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
		Role:     i18n.T(ctx, "chemistry.role"),
		Preamble: i18n.T(ctx, "chemistry.preamble"),
		Topic:    i18n.T(ctx, "chemistry.topic"),
		Mission:  i18n.T(ctx, "chemistry.mission"),
		Objectives: []string{
			i18n.T(ctx, "chemistry.objective1"),
			i18n.T(ctx, "chemistry.objective2"),
			i18n.T(ctx, "chemistry.objective3"),
			i18n.T(ctx, "chemistry.objective4"),
		},
		LearningPath: &prompts.LearningPath{
			Prerequisites: []string{
				i18n.T(ctx, "chemistry.prerequisite1"),
				i18n.T(ctx, "chemistry.prerequisite2"),
				i18n.T(ctx, "chemistry.prerequisite3"),
				i18n.T(ctx, "chemistry.prerequisite4"),
			},
			NextConcepts: []string{
				i18n.T(ctx, "chemistry.next_concept1"),
				i18n.T(ctx, "chemistry.next_concept2"),
				i18n.T(ctx, "chemistry.next_concept3"),
				i18n.T(ctx, "chemistry.next_concept4"),
			},
			RelatedTopics: []string{
				i18n.T(ctx, "chemistry.related_topic1"),
				i18n.T(ctx, "chemistry.related_topic2"),
				i18n.T(ctx, "chemistry.related_topic3"),
			},
			PracticeAreas: []string{
				i18n.T(ctx, "chemistry.practice_area1"),
				i18n.T(ctx, "chemistry.practice_area2"),
				i18n.T(ctx, "chemistry.practice_area3"),
				i18n.T(ctx, "chemistry.practice_area4"),
			},
		},
		Interactive: true,
		ExtraRules:  i18n.T(ctx, "chemistry.extra_rules"),
	})
	if err != nil {
		return agent.Config{}, err
	}

	return agent.Config{
		Name:          "chemistry-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Capabilities: agent.Capabilities{
			InputElicitation: true,
		},
	}, nil
}
