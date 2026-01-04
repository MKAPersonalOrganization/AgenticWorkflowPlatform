package physics

import (
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/eventbus"
	"school-with-ai/services/agents/physics/tools"
	sharedtools "school-with-ai/services/agents/shared/tools"
	"school-with-ai/services/sdk"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
		sharedtools.NewCalculatorTool(),
		sharedtools.NewUnitConversionTool(),
		tools.NewWeatherTool(),
	}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,  // "question.answered"
		agents.EventTypeMathQuestion,      // "question.math"
		agents.EventTypeChemistryQuestion, // "question.chemistry"
		agents.EventTypeWhiteboardUpdated, // "whiteboard.updated"
	}
}

// Contract defines the event-driven contract for the physics instructor.
var Contract = &eventbus.AgentContract{
	Name: "physics-instructor",

	ListensTo: []agents.EventType{
		agents.EventTypePhysicsQuestion,
	},

	Emits: emits(),
	Tools: domainTools(),

	Factory: CreatePhysicsInstructor,
}
