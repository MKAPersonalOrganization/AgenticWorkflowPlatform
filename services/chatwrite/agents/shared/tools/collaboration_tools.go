package tools

import (
	"encoding/json"
	"fmt"

	"school-with-ai/domain"
	"school-with-ai/services/chatwrite/agents"
)

// WhiteboardStateGetter is a function that retrieves the whiteboard state for a thread
type WhiteboardStateGetter func(threadID string) string

// IsCollaborationToolFn is a function type for checking if a tool is a collaboration tool.
// This allows the caller to inject the check logic to avoid import cycles.
type IsCollaborationToolFn func(toolName string) bool

// HandleCollaborationToolFn creates a domain event from a tool call.
type HandleCollaborationToolFn func(threadID, agentName string, toolCall domain.ToolCall, getWhiteboardState WhiteboardStateGetter) (*agents.DomainEvent, error)

// CreateHandoffEventFn creates a handoff event for a tool.
type CreateHandoffEventFn func(threadID, fromAgent, toolName string) *agents.DomainEvent

// HandleSpecialToolCall processes special collaboration tools (question_answered, whiteboard).
// This is called for tools that aren't handled by the eventbus registry.
func HandleSpecialToolCall(threadID string, agentName string, toolCall domain.ToolCall, getWhiteboardState WhiteboardStateGetter) (*agents.DomainEvent, error) {
	toolName := toolCall.Function.Name

	switch toolName {
	case "emit_question_answered":
		var input struct {
			Subject string `json:"subject"`
			Summary string `json:"summary"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse emit_question_answered arguments: %w", err)
		}
		return agents.NewQuestionAnsweredEvent(threadID, input.Subject, input.Summary, agentName, getWhiteboardState(threadID)), nil

	case "write_whiteboard":
		var input struct {
			Content string `json:"content"`
			Action  string `json:"action"`
		}
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &input); err != nil {
			return nil, fmt.Errorf("failed to parse write_whiteboard arguments: %w", err)
		}
		return agents.NewWhiteboardUpdatedEvent(threadID, agentName, input.Content, input.Action), nil

	default:
		return nil, fmt.Errorf("unknown special tool: %s", toolName)
	}
}

// GetSpecialToolHandoffEvent creates a handoff event for special tools.
func GetSpecialToolHandoffEvent(threadID string, fromAgent string, toolName string) *agents.DomainEvent {
	if toolName == "emit_question_answered" {
		return agents.NewAgentHandoffEvent(threadID, fromAgent, "student-facing-agent", "Question answered, returning to coordinator")
	}
	return nil
}
