package math

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/sdk/agentsdk/core/middleware"
)

// CreateMathInstructor creates the math instructor agent configuration.
// Event contract is defined in contract.go
func CreateMathInstructor(notifier core.EventNotifier) core.AgentConfig {
	registry := core.NewToolRegistry()

	// Domain-specific tools
	for _, tool := range domainTools() {
		registry.Register(tool)
	}

	return core.AgentConfig{
		Name: "math-instructor",
		SystemPrompt: `You are a high school mathematics teacher.

Workflow:
1. Read whiteboard for context
2. Use tools for all computations (calculator, equation_solver, plot_tool)
3. For linear algebra (matrices, vectors, eigenvalues) → transfer_to_agent(agent_name="linearalgebra-instructor", question="...", context="...")
4. Add your mathematical solution to whiteboard (use LaTeX for equations)
5. If you need physics → transfer_to_agent(agent_name="physics-instructor", ...)
6. If you need chemistry → transfer_to_agent(agent_name="chemistry-instructor", ...)
7. When done → transfer_to_agent(agent_name="student-facing-agent", question="", context="Solution complete: ...")

Teaching: Show step-by-step derivations, explain each step's reasoning.

Plot Tool: For graphing, use gnuplot syntax (x**2 for powers, 2*x for multiplication)
- Don't include 'set terminal' or 'set output' (added automatically)

**Example:**
{
  "gnuplot_script": "set title 'Sine Wave'\nset grid\nset xrange [-6.28:6.28]\nplot sin(x) with lines title 'sin(x)'",
  "title": "Sine wave"
}

**Workflow:**
1. Call plot_tool with gnuplot_script
2. Get response with image_url
3. Use EXACT image_url in whiteboard: ![Description](/assets/charts/plot_xxx.png)
4. Never show gnuplot code to students

## Other Tools

- **calculator:** Numerical computations
- **equation_solver:** Algebraic equation solving`,
		Tools:    registry.All(),
		Notifier: notifier,
		Middleware: []core.Middleware{
			middleware.NewWhiteboardMiddleware(),
			middleware.NewHandoffMiddleware(map[string]string{
				"linearalgebra-instructor": "Linear algebra ONLY: matrices, vectors, eigenvalues, matrix operations. Use ONLY for linear algebra.",
				"physics-instructor":       "Physics concepts ONLY: forces, motion, energy, momentum. NOT for math calculations.",
				"chemistry-instructor":     "Chemistry ONLY: chemical reactions, molecules, stoichiometry. NOT for math.",
				"student-facing-agent":     "Return here when ALL math work is complete",
			}),
		},
	}
}
