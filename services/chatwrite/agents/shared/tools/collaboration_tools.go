package tools

import (
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/sdk/agentsdk/core"
)

// WhiteboardStateGetter is a function that retrieves the whiteboard state for a thread
type WhiteboardStateGetter func(threadID string) string

// IsCollaborationToolFn is a function type for checking if a tool is a collaboration tool.
// This allows the caller to inject the check logic to avoid import cycles.
type IsCollaborationToolFn func(toolName string) bool

// HandleCollaborationToolFn creates a domain event from a tool call.
type HandleCollaborationToolFn func(threadID, agentName string, toolCall domain.ToolCall, getWhiteboardState WhiteboardStateGetter) (core.Event, error)

// CreateHandoffEventFn creates a handoff event for a tool.
type CreateHandoffEventFn func(threadID, fromAgent, toolName string) *domain.DomainEvent

// HandleSpecialToolCall processes special collaboration tools.
// This is called for tools that aren't handled by the eventbus registry.
func HandleSpecialToolCall(threadID string, agentName string, toolCall domain.ToolCall, getWhiteboardState WhiteboardStateGetter) (core.Event, error) {
	toolName := toolCall.Function.Name

	switch toolName {
	case "emit_question_answered":
		// No custom event needed - agent handoffs and completions are tracked through
		// standard ToolCallExecutedEvent and agent activity
		return nil, nil

	case "write_whiteboard":
		// No custom event needed - ToolCallExecutedEvent is automatically emitted
		// The whiteboard projection listens to ToolCallExecutedEvent and parses the arguments
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown special tool: %s", toolName)
	}
}

// GetSpecialToolHandoffEvent creates a handoff event for special tools.
// This function is no longer used as handoffs are now tool-based.
func GetSpecialToolHandoffEvent(threadID string, fromAgent string, toolName string) *domain.DomainEvent {
	return nil
}
