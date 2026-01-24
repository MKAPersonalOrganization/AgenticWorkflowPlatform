package statistics

import (
	"context"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/foundation/i18n"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
	"school-with-ai/services/agentorchestrator/agents/statistics/tools"
)

// domainTools returns the domain-specific tools for the statistics instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		&sharedtools.AddTool{},
		&sharedtools.SubtractTool{},
		&sharedtools.MultiplyTool{},
		&sharedtools.DivideTool{},
		&sharedtools.PowerTool{},
		&sharedtools.SqrtTool{},
		// Descriptive Statistics
		&tools.MeanTool{},
		&tools.MedianTool{},
		&tools.ModeTool{},
		&tools.MinTool{},
		&tools.MaxTool{},
		&tools.SumTool{},
		&tools.VarianceTool{},
		&tools.StandardDeviationTool{},
		&tools.QuartileTool{},
		&tools.PercentileTool{},
		&tools.IQRTool{},
		// Correlation
		&tools.PearsonCorrelationTool{},
		// Distance Metrics
		&tools.EuclideanDistanceTool{},
		&tools.ManhattanDistanceTool{},
		// Sampling & Outliers
		&tools.SampleTool{},
		&tools.QuartileOutliersTool{},
		// Normal Distribution
		&tools.NormalPdfTool{},
		&tools.NormalCdfTool{},
		&tools.NormalPpfTool{},
		&tools.NormalMeanTool{},
		&tools.NormalMedianTool{},
		&tools.NormalStdTool{},
		&tools.NormalVarTool{},
		&tools.NormalIntervalTool{},
		&tools.BinomialNcrTool{},
		// Regression
		&tools.LinearRegressionTool{},
		// Other
		core.NewWriteTodosTool(),
	}
}

// CreateStatisticsInstructor creates the statistics instructor agent.
// Event contract is defined in contract.go.
func CreateStatisticsInstructor(ctx context.Context, notifier core.EventNotifier) core.AgentConfig {
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
		LearningPath: &core.LearningPath{
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
		Interactive:  true,
		ExtraRules:   i18n.T(ctx, "statistics.extra_rules"),
		IncludeTodos: true,
	})
	if err != nil {
		panic(err)
	}

	return core.AgentConfig{
		Name:          "statistics-instructor",
		MaxIterations: 20,
		SystemPrompt:  prompt,
		Tools:         registry.All(),
		Notifier:      notifier,
		Middleware: []core.Middleware{
			middleware.NewElicitationMiddleware(),
		},
	}
}
