package statistics

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
	"school-with-ai/services/chatwrite/agents/statistics/tools"
)

func domainTools() []core.Tool {
	return []core.Tool{
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

// Contract defines the contract for the statistics instructor.
var Contract = &eventbus.AgentContract{
	Name: "statistics-instructor",

	Tools:   domainTools(),
	Factory: CreateStatisticsInstructor,
}
