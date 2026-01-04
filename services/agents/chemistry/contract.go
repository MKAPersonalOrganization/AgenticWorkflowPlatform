package chemistry

import (
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/chemistry/tools"
	"school-with-ai/services/agents/eventbus"
	sharedtools "school-with-ai/services/agents/shared/tools"
	"school-with-ai/services/sdk"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
		sharedtools.NewCalculatorTool(),
		sharedtools.NewUnitConversionTool(),
		tools.NewPeriodicTableTool(),
	}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,  // "question.answered"
		agents.EventTypeMathQuestion,      // "question.math"
		agents.EventTypePhysicsQuestion,   // "question.physics"
		agents.EventTypeWhiteboardUpdated, // "whiteboard.updated"
	}
}

// Contract defines the event-driven contract for the chemistry instructor.
var Contract = &eventbus.AgentContract{
	Name: "chemistry-instructor",

	ListensTo: []agents.EventType{
		agents.EventTypeChemistryQuestion,
	},

	Emits: emits(),
	Tools: domainTools(),

	Factory: CreateChemistryInstructor,
}
