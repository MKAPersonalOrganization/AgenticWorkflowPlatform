package linearalgebra

import (
	"school-with-ai/services/agents"
	"school-with-ai/services/agents/advancedmath/linearalgebra/tools"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/services/sdk"
)

func domainTools() []sdk.Tool {
	return []sdk.Tool{
		tools.NewVectorOperationsTool(),
		tools.NewMatrixOperationsTool(),
		tools.NewLinearSolverTool(),
		tools.NewEigenvalueTool(),
	}
}

func emits() []agents.EventType {
	return []agents.EventType{
		agents.EventTypeQuestionAnswered,  // "question.answered"
		agents.EventTypeWhiteboardUpdated, // "whiteboard.updated"
	}
}

// Contract defines the event-driven contract for the linear algebra professor.
var Contract = &eventbus.AgentContract{
	Name: "linearalgebra-professor",

	ListensTo: []agents.EventType{
		agents.EventTypeLinearAlgebraQuestion,
	},

	Emits: emits(),
	Tools: domainTools(),

	Factory: CreateLinearAlgebraProfessor,
}
