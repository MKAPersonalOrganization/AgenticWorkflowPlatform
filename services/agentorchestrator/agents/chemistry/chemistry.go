package chemistry

import (
	"context"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/i18n"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the chemistry instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		core.NewWriteTodosTool(),
	}
}

// CreateChemistryInstructor creates the chemistry instructor agent configuration.
func CreateChemistryInstructor(ctx context.Context, notifier core.EventNotifier) core.AgentConfig {
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
		LearningPath: &core.LearningPath{
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
		Interactive:  true,
		ExtraRules:   i18n.T(ctx, "chemistry.extra_rules"),
		IncludeTodos: true,
	})
	if err != nil {
		panic(err)
	}

	return core.AgentConfig{
		Name:          "chemistry-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Notifier:      notifier,
		Middleware: []core.Middleware{
			middleware.NewElicitationMiddleware(),
		},
	}
}
