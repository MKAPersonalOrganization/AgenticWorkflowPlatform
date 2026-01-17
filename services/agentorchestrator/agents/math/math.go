package math

import (
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
func CreateMathInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name:          "math-instructor",
		MaxIterations: 5,
		SystemPrompt: `# Math Instructor
You are a Mathematics specialist.

## Core Rules
1. **Chat for Teaching**: Use chat messages for explanations, walk-throughs, and conversational teaching.
2. **Questions**: If you need to ask the student something, you MUST use the "elicit_user_input" tool. **Always ask exactly ONE question at a time.** Never ask multiple questions in a single response.
3. **Result Reporting**: When finished, return to "student-facing-agent". **REQUIRED**: Provide your final answer in the 'context' field of "transfer_to_agent".
4. **Calculations**: Use the "calculate" tool for arithmetic.

## Creating Plots
When you visualize a function:
1. Use plot tool -> returns {"image_url": "..."}
2. Include the image in your response: ![Description](/assets/charts/plot.png)`,
		Tools: registry.All(),
		Handoffs: []core.Handoff{
			{AgentName: "linearalgebra-instructor", Description: "Linear algebra ONLY: matrices, vectors, eigenvalues, matrix operations. Use ONLY for linear algebra."},
			{AgentName: "physics-instructor", Description: "Physics concepts ONLY: forces, motion, energy, momentum. NOT for math calculations."},
			{AgentName: "chemistry-instructor", Description: "Chemistry ONLY: chemical reactions, molecules, stoichiometry. NOT for math."},
			{AgentName: "student-facing-agent", Description: "Return here when ALL math work is complete"},
		},
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewHandoffMiddleware(),
			middleware.NewElicitationMiddleware(),
		},
	}
}
