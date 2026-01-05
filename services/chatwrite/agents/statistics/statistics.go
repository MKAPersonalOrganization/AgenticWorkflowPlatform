package statistics

import (
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
	"school-with-ai/sdk/agent-sdk/core"
)

// CreateStatisticsInstructor creates the statistics instructor agent.
// Event contract is defined in contract.go
func CreateStatisticsInstructor(notifier sdk.EventNotifier) sdk.AgentConfig {
	registry := sdk.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Auto-generated emission tools
	for _, tool := range eventbus.GenerateEmissionTools(emits()) {
		registry.Register(tool)
	}
	// Custom emit_question_answered tool (not auto-generated)
	registry.Register(sharedtools.NewEmitQuestionAnsweredTool())
	// Shared collaboration tools
	registry.Register(sharedtools.NewReadWhiteboardTool())
	registry.Register(sharedtools.NewWriteWhiteboardTool())

	return sdk.AgentConfig{
		Name: "statistics-instructor",
		SystemPrompt: `# Statistics Instructor

## Core Rules

1. **Read First**: Always call read_whiteboard as your first action
2. **Whiteboard Only**: All content via write_whiteboard - never text responses
3. **End with Event**: Always emit question_answered after writing results

## Workflow

1. read_whiteboard() - get question and data
2. Execute statistical tools as needed
3. write_whiteboard(action="append", content="## STATISTICS\n[results]\n")
4. emit_question_answered(subject="statistics", is_complete=true)

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
