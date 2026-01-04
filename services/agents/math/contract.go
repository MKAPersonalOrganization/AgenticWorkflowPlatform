package math

import (
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/math/tools"
	sharedtools "school-with-ai/services/agents/shared/tools"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/sdk"
)

// domainTools returns the domain-specific tools for the math instructor.
// Defined as a function to avoid initialization cycles.
func domainTools() []sdk.Tool {
	return []sdk.Tool{
		sharedtools.NewCalculatorTool(),
		tools.NewGraphingTool(),
		tools.NewEquationSolverTool(),
		tools.NewPlotTool(),
		tools.NewAnimateTool(),
	}
}

// emits returns the events this agent can emit.
// Defined as a function to avoid initialization cycles.
func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,      // "question.answered"
		agents.EventTypePhysicsQuestion,       // "question.physics"
		agents.EventTypeChemistryQuestion,     // "question.chemistry"
		agents.EventTypeLinearAlgebraQuestion, // "question.linearalgebra"
		agents.EventTypeDiagrammingQuestion,   // "question.diagramming"
		agents.EventTypeWhiteboardUpdated,     // "whiteboard.updated"
	}
}

// Contract defines the event-driven contract for the math instructor.
// The system uses this to:
// - Route question.math events to this agent
// - Auto-generate emission tools for the events this agent can emit
var Contract = &eventbus.AgentContract{
	Name: "math-instructor",

	// Events this agent reacts to
	ListensTo: []agents.EventType{
		agents.EventTypeMathQuestion, // "question.math"
	},

	// Events this agent can emit (emission tools are auto-generated)
	Emits: emits(),

	// Domain-specific tools (emission tools are auto-generated from Emits)
	Tools: domainTools(),

	Factory: CreateMathInstructor,
}
