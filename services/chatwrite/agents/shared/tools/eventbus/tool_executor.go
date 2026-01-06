package eventbus

import (
	"school-with-ai/domain"
	sdk "school-with-ai/sdk/agent-sdk/core"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/chatwrite/agents/shared/tools"
)

// ThreadStore provides read access to thread state
type ThreadStore interface {
	GetAgentThread(threadID string) (sdk.AgentThread, bool)
}

// CreateToolCallHandler creates a tool call handler that emits events for collaboration tools
func CreateToolCallHandler(registry *ContractRegistry, state ThreadStore, whiteboardStore interface{ Get(threadID string) string }) chatwrite.ToolCallHandler {
	return func(threadID string, agentName string, toolCall domain.ToolCall, result string, eventEmitter func(sdk.Event) error) error {
		toolName := toolCall.Function.Name

		// Create whiteboard state getter using WhiteboardStore
		getWhiteboardState := func(tid string) string {
			return whiteboardStore.Get(tid)
		}

		// Handle emit_*_question tools (dynamically routed via EventBus)
		if registry.IsEmissionTool(toolName) {
			// Emit the domain event
			event, err := registry.CreateEventFromToolCall(threadID, agentName, toolCall, getWhiteboardState)
			if err != nil {
				return err
			}
			if err := eventEmitter(event); err != nil {
				return err
			}

			// Emit agent handoff event
			if handoffEvent := registry.CreateHandoffEvent(threadID, agentName, toolName); handoffEvent != nil {
				if err := eventEmitter(handoffEvent); err != nil {
					return err
				}
			}
			return nil
		}

		// Handle special collaboration tools (emit_question_answered, write_whiteboard)
		if toolName == "emit_question_answered" || toolName == "write_whiteboard" {
			// Emit the domain event
			event, err := tools.HandleSpecialToolCall(threadID, agentName, toolCall, getWhiteboardState)
			if err != nil {
				return err
			}
			if err := eventEmitter(event); err != nil {
				return err
			}

			// Emit agent handoff event for emit_question_answered
			if handoffEvent := tools.GetSpecialToolHandoffEvent(threadID, agentName, toolName); handoffEvent != nil {
				if err := eventEmitter(handoffEvent); err != nil {
					return err
				}
			}
			return nil
		}

		// For all other tools, no special handling needed
		return nil
	}
}

// CreateShouldContinueIterationFn creates a function that determines if iteration should continue
func CreateShouldContinueIterationFn(registry *ContractRegistry) chatwrite.ShouldContinueIterationFn {
	return func(toolCalls []domain.ToolCall) bool {
		// Stop iteration if any tool triggers another agent (emission tools)
		for _, toolCall := range toolCalls {
			if registry.IsEmissionTool(toolCall.Function.Name) || toolCall.Function.Name == "emit_question_answered" {
				return false
			}
		}
		return true
	}
}
