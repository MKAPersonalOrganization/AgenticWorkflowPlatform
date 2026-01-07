package diagramming

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents/diagramming/tools"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

func domainTools() []core.Tool {
	return []core.Tool{
		tools.NewMermaidDiagramTool(),
	}
}

// Contract defines the contract for the diagramming instructor.
var Contract = &eventbus.AgentContract{
	Name: "diagramming-instructor",

	Tools:   domainTools(),
	Factory: CreateDiagrammingInstructor,
}
