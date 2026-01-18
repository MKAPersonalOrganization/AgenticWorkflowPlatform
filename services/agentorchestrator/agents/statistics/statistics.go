package statistics

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
	"school-with-ai/services/agentorchestrator/agents/statistics/tools"
)

// domainTools returns the domain-specific tools for the statistics instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		tools.NewDescriptiveStatsTool(),
		tools.NewCorrelationTool(),
		tools.NewDistanceMetricsTool(),
		tools.NewSamplingTool(),
		tools.NewDataTransformTool(),
		tools.NewNormalDistributionTool(),
		tools.NewGeometricDistributionTool(),
		tools.NewRegressionTool(),
		core.NewWriteTodosTool(),
	}
}

// CreateStatisticsInstructor creates the statistics instructor agent.
// Event contract is defined in contract.go.
func CreateStatisticsInstructor(notifier core.EventNotifier) core.AgentConfig {
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
		Role:        "Statistics Instructor",
		Preamble:    "You are an expert statistics educator focused on teaching students how to collect, analyze, interpret, and present data.",
		Topic:       "Statistics and Probability",
		Interactive: true,
		ExtraRules: `## Core Statistics Rules
1. **Subject Authority**: You are the SOLE authority for Statistics. Never ask the 'student-facing-agent' or other instructors for help with statistics.
2. **Calculations**: Use specialized domain tools for arithmetic, distributions, and regression.
3. **Teaching Style**: Interpret results in context. Check assumptions (e.g., positive values for geometric mean). Use tables for clarity.

## LaTeX Examples (Use LaTeX for all math)
**Mean:** $\bar{x} = \frac{1}{n}\sum_{i=1}^{n} x_i$
**Std Dev:** $s = \sqrt{\frac{1}{n-1}\sum_{i=1}^{n}(x_i - \bar{x})^2}$
**Correlation:** $r = \frac{\sum(x_i - \bar{x})(y_i - \bar{y})}{\sqrt{\sum(x_i - \bar{x})^2\sum(y_i - \bar{y})^2}}$
**Regression:** $y = mx + b$ with $R^2$`,
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
			middleware.NewHandoffMiddleware(),
			middleware.NewElicitationMiddleware(),
		},
	}
}
