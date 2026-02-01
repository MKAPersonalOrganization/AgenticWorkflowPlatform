package math

import (
	"context"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/i18n"
	mathtools "school-with-ai/services/agentorchestrator/agents/math/tools"
	sharedtools "school-with-ai/services/agentorchestrator/agents/tools"
)

// domainTools returns the domain-specific tools for the math instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		&sharedtools.AddTool{},
		&sharedtools.SubtractTool{},
		&sharedtools.MultiplyTool{},
		&sharedtools.DivideTool{},
		&sharedtools.PowerTool{},
		&sharedtools.SqrtTool{},
		mathtools.NewPlotTool(),    // gnuplot-based plotting
		mathtools.NewAnimateTool(), // gnuplot-based animations
	}
}

// CreateMathInstructor creates the math instructor agent configuration.
func CreateMathInstructor(ctx context.Context, notifier core.EventNotifier) core.AgentConfig {
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
		LearningPath: &core.LearningPath{
			Prerequisites: []string{
				i18n.T(ctx, "math.prerequisite1"),
				i18n.T(ctx, "math.prerequisite2"),
				i18n.T(ctx, "math.prerequisite3"),
			},
			NextConcepts: []string{
				i18n.T(ctx, "math.next_concept1"),
				i18n.T(ctx, "math.next_concept2"),
				i18n.T(ctx, "math.next_concept3"),
				i18n.T(ctx, "math.next_concept4"),
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
		panic(err)
	}

	return core.AgentConfig{
		Name:          "math-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Notifier:      notifier,
		Middleware: []core.Middleware{
			middleware.NewElicitationMiddleware(),
		},
	}
}
