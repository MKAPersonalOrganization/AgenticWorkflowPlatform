package linearalgebra

import (
	sharedtools "school-with-ai/services/agents/shared/tools"
	"school-with-ai/services/agents/shared/tools/eventbus"
	"school-with-ai/sdk/agent-sdk/core"
)

// CreateLinearAlgebraProfessor creates the linear algebra professor agent.
// Event contract is defined in contract.go
func CreateLinearAlgebraProfessor(notifier sdk.EventNotifier) sdk.AgentConfig {
	registry := sdk.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	// Auto-generated emission tools
	for _, tool := range eventbus.GenerateEmissionTools(emits()) {
		registry.Register(tool)
	}
	// Custom emit_question_answered tool (not auto-generated)
	registry.Register(sharedtools.NewEmitQuestionAnsweredTool())
	// Shared collaboration tools
	registry.Register(sharedtools.NewReadWhiteboardTool())
	registry.Register(sharedtools.NewWriteWhiteboardTool())

	return sdk.AgentConfig{
		Name: "linearalgebra-professor",
		SystemPrompt: `# Linear Algebra Professor

## Core Rules

1. **Read First**: Always call read_whiteboard as your first action
2. **Tool-First**: ALWAYS use tools for computation - never solve manually
3. **Whiteboard Only**: All content via write_whiteboard - never text responses
4. **End with Event**: Always emit question_answered after writing results

## Workflow

1. read_whiteboard() - get problem context
2. Use appropriate tool (linear_solver, matrix_operations, etc.)
3. write_whiteboard(action="append", content="## LINEAR ALGEBRA\n[results]\n")
4. emit_question_answered(subject="linearalgebra", is_complete=true)

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
