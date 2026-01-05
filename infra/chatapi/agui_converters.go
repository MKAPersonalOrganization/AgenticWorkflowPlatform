package chatapi

import (
	"school-with-ai/domain"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ConvertSessionMessageToAGUI converts a domain.Message to aguievents.Message
func ConvertSessionMessageToAGUI(msg domain.Message) aguievents.Message {
	aguiMsg := aguievents.Message{
		ID:   msg.ID,
		Role: msg.Role,
	}

	// Set content (pointer for optional content)
	if msg.Content != "" {
		aguiMsg.Content = &msg.Content
	}

	// Add agent name for assistant messages
	if msg.Role == "assistant" && msg.AgentName != "" {
		aguiMsg.Name = &msg.AgentName
	}

	// Convert tool calls if present
	if len(msg.ToolCalls) > 0 {
		toolCalls := make([]aguievents.ToolCall, 0, len(msg.ToolCalls))
		for _, tc := range msg.ToolCalls {
			toolCalls = append(toolCalls, aguievents.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: aguievents.Function{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			})
		}
		aguiMsg.ToolCalls = toolCalls
	}

	// Add tool call ID for tool messages
	if msg.Role == "tool" && msg.ToolCallID != "" {
		aguiMsg.ToolCallID = &msg.ToolCallID
	}

	return aguiMsg
}
