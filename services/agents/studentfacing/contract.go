package studentfacing

import (
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/sdk"
)

func domainTools() []sdk.Tool {
	// No domain-specific tools - this agent only routes
	return []sdk.Tool{}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeMathQuestion,       // "question.math"
		agents.EventTypePhysicsQuestion,    // "question.physics"
		agents.EventTypeChemistryQuestion,  // "question.chemistry"
		agents.EventTypeStatisticsQuestion, // "question.statistics"
		agents.EventTypeWhiteboardUpdated,  // "whiteboard.updated"
	}
}

// Contract defines the event-driven contract for the student-facing agent.
// This agent is special - it receives user messages and routes them to specialists.
var Contract = &eventbus.AgentContract{
	Name: "student-facing-agent",

	// Also receives direct user messages (handled by system, not EventBus)
	ListensTo: []agents.EventType{
		agents.EventTypeQuestionAnswered,
	},

	Emits: emits(),
	Tools: domainTools(),

	Factory: CreateStudentFacingAgent,
}
