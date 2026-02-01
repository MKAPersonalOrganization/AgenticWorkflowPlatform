package engine

import (
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
)

// mapFromCoreToLLMMessages converts a slice of core.Message to llm.ChatMessage.
func mapFromCoreToLLMMessages(msgs []core.Message) []llm.ChatMessage {
	result := make([]llm.ChatMessage, 0, len(msgs))

	var currentToolMsg *llm.ChatMessage

	for _, m := range msgs {
		cm := mapFromCoreToLLMMessage(m)

		if cm.Role == llm.ChatRoleTool {
			// If we are currently building a tool message, add to it
			if currentToolMsg != nil {
				currentToolMsg.Contents = append(currentToolMsg.Contents, cm.Contents...)
				continue
			}
			// Start a new tool message
			currentToolMsg = &cm
			result = append(result, *currentToolMsg)
			// Get reference to the one in the slice so we append to it
			currentToolMsg = &result[len(result)-1]
		} else {
			// Non-tool message, reset current tool message builder
			currentToolMsg = nil
			result = append(result, cm)
		}
	}

	return result
}

// mapFromCoreToLLMMessage converts a core.Message to llm.ChatMessage.
// This bridges the persistence layer (core.Message) with the LLM layer (ChatMessage).
func mapFromCoreToLLMMessage(msg core.Message) llm.ChatMessage {
	cm := llm.ChatMessage{
		Role:       llm.ChatRole(msg.Role),
		AuthorName: msg.AgentName,
		Contents:   make([]llm.AIContent, 0),
	}

	// Add text content if present
	// Note: We check specifically for non-tool role because tool outputs are handled below
	if msg.Content != "" && msg.Role != "tool" {
		cm.Contents = append(cm.Contents, llm.TextContent{Text: msg.Content})
	}

	// Add function calls from tool calls
	for _, tc := range msg.ToolCalls {
		cm.Contents = append(cm.Contents, llm.FunctionCallContent{
			CallID:       tc.ID,
			Name:         tc.Function.Name,
			RawArguments: tc.Function.Arguments,
		})
	}

	// Add function result for tool messages
	if msg.Role == "tool" && msg.ToolCallID != "" {
		cm.Contents = append(cm.Contents, llm.FunctionResultContent{
			CallID: msg.ToolCallID,
			Result: msg.Content,
		})
	}

	return cm
}

func toDomainToolCalls(ctcs []core.ToolCall) []domain.ToolCall {
	if ctcs == nil {
		return nil
	}
	result := make([]domain.ToolCall, len(ctcs))
	for i, ctc := range ctcs {
		result[i] = domain.ToolCall{
			ID:   ctc.ID,
			Type: ctc.Type,
			Function: domain.ToolFunction{
				Name:      ctc.Function.Name,
				Arguments: ctc.Function.Arguments,
			},
		}
	}
	return result
}
