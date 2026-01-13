package shared

import (
	"school-with-ai/domain"
	"school-with-ai/services/agentorchestrator/types"
)

// CreateShouldContinueIterationFn creates a function that determines if iteration should continue.
func CreateShouldContinueIterationFn() types.ShouldContinueIterationFn {
	return func(toolCalls []domain.ToolCall) bool {
		// Stop iteration if any tool triggers another agent
		for _, toolCall := range toolCalls {
			toolName := toolCall.Function.Name
			// Stop for handoff tool
			if toolName == "transfer_to_agent" {
				return false
			}
		}

		return true
	}
}
