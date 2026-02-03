package physics

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/i18n"
	sharedtools "school-with-ai/services/agentorchestrator/agents/tools"
	"school-with-ai/services/agentorchestrator/prompts"
)

// domainTools returns the domain-specific tools for the physics instructor.
func domainTools() []agent.Tool {
	return []agent.Tool{
		&sharedtools.SqrtTool{},
	}
}

// CreatePhysicsInstructor creates the physics instructor agent configuration.
func CreatePhysicsInstructor(ctx context.Context) agent.Config {
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
		Role:     i18n.T(ctx, "physics.role"),
		Preamble: i18n.T(ctx, "physics.preamble"),
		Topic:    i18n.T(ctx, "physics.topic"),
		Mission:  i18n.T(ctx, "physics.mission"),
		Objectives: []string{
			i18n.T(ctx, "physics.objective1"),
			i18n.T(ctx, "physics.objective2"),
			i18n.T(ctx, "physics.objective3"),
			i18n.T(ctx, "physics.objective4"),
		},
		LearningPath: &prompts.LearningPath{
			Prerequisites: []string{
				i18n.T(ctx, "physics.prerequisite1"),
				i18n.T(ctx, "physics.prerequisite2"),
				i18n.T(ctx, "physics.prerequisite3"),
				i18n.T(ctx, "physics.prerequisite4"),
			},
			NextConcepts: []string{
				i18n.T(ctx, "physics.next_concept1"),
				i18n.T(ctx, "physics.next_concept2"),
				i18n.T(ctx, "physics.next_concept3"),
				i18n.T(ctx, "physics.next_concept4"),
			},
			RelatedTopics: []string{
				i18n.T(ctx, "physics.related_topic1"),
				i18n.T(ctx, "physics.related_topic2"),
				i18n.T(ctx, "physics.related_topic3"),
			},
			PracticeAreas: []string{
				i18n.T(ctx, "physics.practice_area1"),
				i18n.T(ctx, "physics.practice_area2"),
				i18n.T(ctx, "physics.practice_area3"),
				i18n.T(ctx, "physics.practice_area4"),
			},
		},
		Interactive: true,
		ExtraRules:  i18n.T(ctx, "physics.extra_rules"),
	})
	if err != nil {
		panic(err)
	}

	return agent.Config{
		Name:          "physics-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Capabilities: agent.Capabilities{
			InputElicitation: true,
		},
	}
}
