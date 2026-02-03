package math

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/i18n"
	mathtools "school-with-ai/services/agentorchestrator/agents/math/tools"
	sharedtools "school-with-ai/services/agentorchestrator/agents/tools"
	"school-with-ai/services/agentorchestrator/prompts"
)

// domainTools returns the domain-specific tools for the math instructor.
func domainTools() []agent.Tool {
	return []agent.Tool{
		&sharedtools.AddTool{},
		&sharedtools.SubtractTool{},
		&sharedtools.MultiplyTool{},
		&sharedtools.DivideTool{},
		&sharedtools.PowerTool{},
		&sharedtools.SqrtTool{},
		&mathtools.PlotTool{},
		&mathtools.AnimateTool{},
		// &mathtools.EquationSolverTool{},
	}
}

// CreateMathInstructor creates the math instructor agent configuration.
func CreateMathInstructor(ctx context.Context) agent.Config {
	registry := agent.NewToolSet()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

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
		panic(err)
	}

	return agent.Config{
		Name:          "math-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Capabilities: agent.Capabilities{
			InputElicitation: true,
		},
	}
}
