package math

import (
	"context"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/i18n"
	mathtools "school-with-ai/services/agentorchestrator/agents/math/tools"
	sharedtools "school-with-ai/services/agentorchestrator/agents/tools"
	"school-with-ai/services/agentorchestrator/prompts"
	"school-with-ai/services/artifact"
	"school-with-ai/services/memory"
	memorytools "school-with-ai/services/memory/tools"
)

// domainTools returns the domain-specific tools for the math instructor.
func domainTools(artifactStore artifact.Service) []agent.Tool {
	return []agent.Tool{
		&sharedtools.AddTool{},
		&sharedtools.SubtractTool{},
		&sharedtools.MultiplyTool{},
		&sharedtools.DivideTool{},
		&sharedtools.PowerTool{},
		&sharedtools.SqrtTool{},
		&mathtools.PlotTool{},
		&mathtools.AnimateTool{},
		&sharedtools.CreateMarkdownTool{ArtifactStore: artifactStore},
		&sharedtools.UpdateMarkdownTool{ArtifactStore: artifactStore},
		&sharedtools.DeleteMarkdownTool{ArtifactStore: artifactStore},
		// &mathtools.EquationSolverTool{},
	}
}

// NewMathInstructorConfig creates the math instructor configuration.
func NewMathInstructorConfig(ctx context.Context, artifactStore artifact.Service) (agent.Config, error) {
	registry := agent.NewToolSet()

	// Domain-specific tools
	for _, tool := range domainTools(artifactStore) {
		registry.Register(tool)
	}

	// Memory tools
	if artifactStore != nil {
		memoryService := memory.NewArtifactMemoryService(artifactStore)
		registry.Register(memorytools.NewMemoryTool(memoryService))
		registry.Register(memorytools.NewForgetMemoryTool(memoryService))
		registry.Register(memorytools.NewListMemoriesTool(memoryService))
		registry.Register(memorytools.NewClearMemoriesTool(memoryService))
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
		return agent.Config{}, err
	}

	return agent.Config{
		Name:          "math-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Capabilities: agent.Capabilities{
			InputElicitation: true,
		},
	}, nil
}
