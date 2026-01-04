package diagramming

import (
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/diagramming/tools"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/sdk"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
		tools.NewMermaidDiagramTool(),
	}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,  // "question.answered"
		agents.EventTypeWhiteboardUpdated, // "whiteboard.updated"
	}
}

// Contract defines the event-driven contract for the diagramming instructor.
var Contract = &eventbus.AgentContract{
	Name: "diagramming-instructor",

	ListensTo: []agents.EventType{
		agents.EventTypeDiagrammingQuestion,
	},

	Emits: emits(),
	Tools: domainTools(),

	Factory: CreateDiagrammingInstructor,
}
