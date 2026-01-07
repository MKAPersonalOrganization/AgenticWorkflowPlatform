package linearalgebra

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents/advancedmath/linearalgebra/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

func domainTools() []core.Tool {
	return []core.Tool{
		tools.NewVectorOperationsTool(),
		tools.NewMatrixOperationsTool(),
		tools.NewLinearSolverTool(),
		tools.NewEigenvalueTool(),
	}
}

// Contract defines the contract for the linear algebra professor.
var Contract = &eventbus.AgentContract{
	Name: "linearalgebra-professor",

	Tools:   domainTools(),
	Factory: CreateLinearAlgebraProfessor,
}
