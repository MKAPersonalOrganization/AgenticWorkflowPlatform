package eventbus

import (
	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite"
)

// ThreadStore provides read access to thread state
type ThreadStore interface {
	GetAgentThread(threadID string) (core.AgentThread, bool)
}

// CreateToolCallHandler creates a tool call handler that emits events for collaboration tools
func CreateToolCallHandler(registry *ContractRegistry, state ThreadStore, whiteboardStore interface{ Get(threadID string) string }) chatwrite.ToolCallHandler {
	return func(threadID string, agentName string, toolCall domain.ToolCall, result string, eventEmitter func(core.Event) error) error {
		// No special handling needed - handoffs are detected from request context
		return nil
	}
}

// CreateShouldContinueIterationFn creates a function that determines if iteration should continue
func CreateShouldContinueIterationFn(registry *ContractRegistry) chatwrite.ShouldContinueIterationFn {
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
