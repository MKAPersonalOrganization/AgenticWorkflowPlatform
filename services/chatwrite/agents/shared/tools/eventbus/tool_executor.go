package eventbus

import (
	"school-with-ai/domain"
	"school-with-ai/services/chatwrite"
)

// CreateShouldContinueIterationFn creates a function that determines if iteration should continue
func CreateShouldContinueIterationFn() chatwrite.ShouldContinueIterationFn {
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
