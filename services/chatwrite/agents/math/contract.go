package math

import (
	"school-with-ai/sdk/agentsdk/core"

	"school-with-ai/services/chatwrite/agents/math/tools"
	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

// domainTools returns the domain-specific tools for the math instructor.
// Defined as a function to avoid initialization cycles.
func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
		tools.NewPlotTool(),    // gnuplot-based plotting - the ONLY plotting tool
		tools.NewAnimateTool(), // gnuplot-based animations
	}
}

// Contract defines the contract for the math instructor.
var Contract = &eventbus.AgentContract{
	Name: "math-instructor",

	Tools:   domainTools(),
	Factory: CreateMathInstructor,
}
