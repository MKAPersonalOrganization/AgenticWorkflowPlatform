package engine

import (
	"encoding/json"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/internal/llm"
)

// mapFromCoreToLLMMessages converts a slice of core.Message to llm.Message.
func mapFromCoreToLLMMessages(msgs []core.Message) []llm.Message {
	result := make([]llm.Message, 0, len(msgs))

	var currentToolMsg *llm.Message

	for _, m := range msgs {
		cm := mapFromCoreToLLMMessage(m)

		if cm.Role == llm.RoleTool {
			// If we are currently building a tool message, add to it
			if currentToolMsg != nil {
				currentToolMsg.Content = append(currentToolMsg.Content, cm.Content...)
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

// mapFromCoreToLLMMessage converts a core.Message to llm.Message.
// This bridges the persistence layer (core.Message) with the LLM layer (Message).
func mapFromCoreToLLMMessage(msg core.Message) llm.Message {
	cm := llm.Message{
		Role:    llm.Role(msg.Role),
		Name:    msg.AgentName,
		Content: make([]llm.Content, 0),
	}

	// Add text content if present
	// Note: We check specifically for non-tool role because tool outputs are handled below
	if msg.Content != "" && msg.Role != "tool" {
		cm.Content = append(cm.Content, llm.TextContent{Text: msg.Content})
	}

	// Add function calls from tool calls
	for _, tc := range msg.ToolCalls {
		cm.Content = append(cm.Content, llm.FunctionCall{
			ID:        tc.ID,
			Name:      tc.Name,
			Arguments: json.RawMessage(tc.Arguments),
		})
	}

	// Add function result for tool messages
	if msg.Role == "tool" {
		cm.Content = append(cm.Content, llm.FunctionResult{
			CallID:  msg.ToolCallID,
			Name:    msg.ToolName,
			Result:  json.RawMessage(msg.Content),
			IsError: msg.IsError,
		})
	}

	return cm
}
