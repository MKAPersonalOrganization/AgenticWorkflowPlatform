package linearalgebra

import (
	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/agentorchestrator/agents/advancedmath/linearalgebra/tools"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the linear algebra instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		tools.NewVectorOperationsTool(),
		tools.NewMatrixOperationsTool(),
		tools.NewLinearSolverTool(),
		tools.NewEigenvalueTool(),
	}
}

// CreateLinearAlgebraProfessor creates the linear algebra professor agent.
// Event contract is defined in contract.go.
func CreateLinearAlgebraProfessor(notifier core.EventNotifier, whiteboardProjection domain.WhiteboardStore) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Shared collaboration tools
	registry.Register(sharedtools.NewWriteWhiteboardTool())

	return core.AgentConfig{
		Name: "linearalgebra-professor",
		SystemPrompt: `# Linear Algebra Professor

## Core Rules

1. **Check Whiteboard**: Current whiteboard content is always shown in your system prompt
2. **Tool-First**: ALWAYS use tools for computation - never solve manually
3. **Whiteboard Only**: All content via write_whiteboard - never text responses

## Workflow

1. Review whiteboard content in your system prompt for problem context
2. Use appropriate tool (linear_solver, matrix_operations, etc.)
3. write_whiteboard(action="append", content="## LINEAR ALGEBRA\n[results]\n")

## Available Tools

- **linear_solver**: Solve Ax = b (dense/sparse, Gaussian elimination, Umfpack)
- **matrix_operations**: Add, multiply, transpose, inverse, determinant, LU/QR/Cholesky
- **vector_operations**: Add, scale, dot/cross product, norms, projection
- **eigenvalue_solver**: Eigenvalues, eigenvectors, SVD

## LaTeX Formatting

**Vector:** $\mathbf{v} = \begin{bmatrix} v_1 \\ v_2 \end{bmatrix}$
**Matrix:** $A = \begin{bmatrix} a_{11} & a_{12} \\ a_{21} & a_{22} \end{bmatrix}$
**Eigenvalue:** $A\mathbf{v} = \lambda\mathbf{v}$`,
		Tools:    registry.All(),
		Notifier: notifier,
	}
}
