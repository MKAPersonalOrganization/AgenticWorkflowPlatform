package eventbus

import (
	"encoding/json"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
	"school-with-ai/services/chatwrite"
	"school-with-ai/services/chatwrite/agents/shared/tools"
)

// ThreadStore provides read access to thread state
type ThreadStore interface {
	GetAgentThread(threadID string) (core.AgentThread, bool)
}

// CreateToolCallHandler creates a tool call handler that emits events for collaboration tools
func CreateToolCallHandler(registry *ContractRegistry, state ThreadStore, whiteboardStore interface{ Get(threadID string) string }) chatwrite.ToolCallHandler {
	return func(threadID string, agentName string, toolCall domain.ToolCall, result string, eventEmitter func(core.Event) error) error {
		toolName := toolCall.Function.Name

		// Create whiteboard state getter using WhiteboardStore
		getWhiteboardState := func(tid string) string {
			return whiteboardStore.Get(tid)
		}

		// Handle transfer_to_agent tool (NEW: handoff system)
		if toolName == "transfer_to_agent" {
			// Parse the tool arguments to get handoff details
			var args struct {
				AgentName string `json:"agent_name"`
				Question  string `json:"question"`
				Context   string `json:"context"`
			}

			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				return err
			}

			// Create agent handoff event
			handoffEvent := &core.AgentHandoffEvent{
				BaseEvent: core.BaseEvent{
					Type:      "AgentHandoff",
					Timestamp: core.Now(),
					ThreadID:  threadID,
				},
				FromAgent:   agentName,
				ToAgent:     args.AgentName,
				Question:    args.Question,
				Context:     args.Context,
				HandoffType: "transfer",
			}

			if err := eventEmitter(handoffEvent); err != nil {
				return err
			}

			return nil
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
		// Stop iteration if any tool triggers another agent
		for _, toolCall := range toolCalls {
			toolName := toolCall.Function.Name
			// Stop for emission tools, handoff tool, or question answered
			if registry.IsEmissionTool(toolName) ||
				toolName == "emit_question_answered" ||
				toolName == "transfer_to_agent" {
				return false
			}
		}
		return true
	}
}
