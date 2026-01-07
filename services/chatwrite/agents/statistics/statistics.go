package statistics

import (
	"school-with-ai/sdk/agentsdk/core"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/statistics/tools"
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
	}
}

// CreateStatisticsInstructor creates the statistics instructor agent.
// Event contract is defined in contract.go
func CreateStatisticsInstructor(notifier core.EventNotifier, whiteboardProjection interface{ Get(threadID string) string }) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Shared collaboration tools
	registry.Register(sharedtools.NewWriteWhiteboardTool())

	return core.AgentConfig{
		Name: "statistics-instructor",
		SystemPrompt: `# Statistics Instructor

## Core Rules

1. **Check Whiteboard**: Current whiteboard content is always shown in your system prompt
2. **Whiteboard Only**: All content via write_whiteboard - never text responses

## Workflow

1. Review whiteboard content in your system prompt for question and data
2. Execute statistical tools as needed
3. write_whiteboard(action="append", content="## STATISTICS\n[results]\n")

## Available Tools

- **descriptive_statistics**: mean, median, mode, variance, std dev, quartiles, IQR
- **correlation_analysis**: correlation, pearson, covariance, autocorrelation
- **distance_metrics**: euclidean, manhattan, chebyshev, minkowski
- **sampling_and_outliers**: sample, stable_sample, quartile_outliers
- **data_transformation**: sigmoid, softmax, cumulative_sum, entropy
- **normal_distribution**: pdf, cdf, ppf, sf, isf, fit, interval
- **geometric_distribution**: prob_geom, exp_geom, var_geom
- **regression_analysis**: linear, exponential, logarithmic

## Teaching Style

- Interpret results in context
- Check assumptions (e.g., positive values for geometric mean)
- Use tables and LaTeX for clarity
- Discuss statistical significance when relevant

## LaTeX Examples

**Mean:** $\\bar{x} = \\frac{1}{n}\\sum_{i=1}^{n} x_i$
**Std Dev:** $s = \\sqrt{\\frac{1}{n-1}\\sum_{i=1}^{n}(x_i - \\bar{x})^2}$
**Correlation:** $r = \\frac{\\sum(x_i - \\bar{x})(y_i - \\bar{y})}{\\sqrt{\\sum(x_i - \\bar{x})^2\\sum(y_i - \\bar{y})^2}}$
**Regression:** $y = mx + b$ with $R^2$`,
		Tools:    registry.All(),
		Notifier: notifier,
	}
}
