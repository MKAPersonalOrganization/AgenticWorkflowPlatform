package math

import (
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/core/middleware"
	"school-with-ai/services/agentorchestrator/agents/math/tools"
	sharedtools "school-with-ai/services/agentorchestrator/agents/shared/tools"
)

// domainTools returns the domain-specific tools for the math instructor.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		tools.NewPlotTool(),    // gnuplot-based plotting
		tools.NewAnimateTool(), // gnuplot-based animations
	}
}

// CreateMathInstructor creates the math instructor agent configuration.
func CreateMathInstructor(notifier core.EventNotifier, whiteboardProjection domain.WhiteboardStore) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name: "math-instructor",
		SystemPrompt: `# Math Instructor
You are a Mathematics specialist.

## Core Rules
1. **Whiteboard Only**: Communicate EXCLUSIVELY via "write_whiteboard". Chat messages are disabled.
2. **Result Reporting**: When finished, return to "student-facing-agent". **REQUIRED**: Provide your final answer in the 'context' field of "transfer_to_agent".
3. **Clarity**: Show steps visually on the whiteboard using LaTeX ($x=1$).
4. **Calculations**: Use the "calculate" tool for arithmetic.

## Creating Plots
When you visualize a function:
1. Use plot tool -> returns {"image_url": "..."}
2. COPY the image_url (e.g., /assets/charts/plot.png)
3. Embed on whiteboard: ![Description](/assets/charts/plot.png)`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(whiteboardProjection),
			middleware.NewHandoffMiddleware(map[string]string{
				"linearalgebra-instructor": "Linear algebra ONLY: matrices, vectors, eigenvalues, matrix operations. Use ONLY for linear algebra.",
				"physics-instructor":       "Physics concepts ONLY: forces, motion, energy, momentum. NOT for math calculations.",
				"chemistry-instructor":     "Chemistry ONLY: chemical reactions, molecules, stoichiometry. NOT for math.",
				"student-facing-agent":     "Return here when ALL math work is complete",
			}),
		},
	}
}
