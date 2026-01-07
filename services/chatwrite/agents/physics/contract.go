package physics

import (
	"school-with-ai/sdk/agentsdk/core"

	sharedtools "school-with-ai/services/chatwrite/agents/shared/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

func domainTools() []core.Tool {
	return []core.Tool{
		sharedtools.NewCalculatorTool(),
	}
}

// Contract defines the event-driven contract for the physics instructor.
var Contract = &eventbus.AgentContract{
	Name: "physics-instructor",

	Tools:   domainTools(),
	Factory: CreatePhysicsInstructor,
}
