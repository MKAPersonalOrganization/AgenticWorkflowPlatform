package statistics

import (
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/agents/statistics/tools"
	"school-with-ai/services/sdk"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
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

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,  // "question.answered"
		agents.EventTypeWhiteboardUpdated, // "whiteboard.updated"
	}
}

// Contract defines the event-driven contract for the statistics instructor.
var Contract = &eventbus.AgentContract{
	Name: "statistics-instructor",

	ListensTo: []agents.EventType{
		agents.EventTypeStatisticsQuestion,
	},

	Emits: emits(),
	Tools: domainTools(),

	Factory: CreateStatisticsInstructor,
}
