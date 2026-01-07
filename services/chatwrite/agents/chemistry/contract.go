package chemistry

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

// Contract defines the event-driven contract for the chemistry instructor.
var Contract = &eventbus.AgentContract{
	Name: "chemistry-instructor",

	Tools:   domainTools(),
	Factory: CreateChemistryInstructor,
}
