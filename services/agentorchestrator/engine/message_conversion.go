package engine

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
)

// toLLMChatMessages converts a slice of core.Message to llm.ChatMessage.
// Filters out system messages with agent_handoff metadata as they're UI-only.
func toLLMChatMessages(msgs []core.Message) []llm.ChatMessage {
	result := make([]llm.ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		// Skip agent handoff messages - they're for UI display only
		if m.Metadata != nil {
			if msgType, ok := m.Metadata["type"].(string); ok && msgType == "agent_handoff" {
				continue
			}
		}
		result = append(result, toLLMChatMessage(m))
	}

	return result
}

// toLLMChatMessage converts a core.Message to llm.ChatMessage.
// This bridges the persistence layer (core.Message) with the LLM layer (ChatMessage).
func toLLMChatMessage(msg core.Message) llm.ChatMessage {
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
