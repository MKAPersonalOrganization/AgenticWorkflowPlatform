package studentfacing

import (
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite/agents/shared/tools/eventbus"
)

func domainTools() []core.Tool {
	// No domain-specific tools - this agent only routes
	return []core.Tool{}
}

// Contract defines the contract for the student-facing agent.
var Contract = &eventbus.AgentContract{
	Name: "student-facing-agent",

	Tools:   domainTools(),
	Factory: CreateStudentFacingAgent,
}
