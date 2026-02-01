package llm

import (
	"strings"
)

// ChatMessage represents a single message in a conversation.
// A message has a role and contains one or more content items.
type ChatMessage struct {
	// Role indicates who sent this message.
	Role ChatRole `json:"role"`

	// AuthorName is an optional identifier for the message author.
	// For agents, this is typically the agent name.
	AuthorName string `json:"authorName,omitempty"`

	// Contents holds the content items in this message.
	// A message can have multiple content items (e.g., text + function calls).
	Contents []AIContent `json:"contents"`
}

// Text returns the concatenated text content of the message.
// Only TextContent items are included; other content types are ignored.
func (m ChatMessage) Text() string {
	var sb strings.Builder
	for _, c := range m.Contents {
		if tc, ok := c.(TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}

	return sb.String()
}

// FunctionCalls returns all FunctionCallContent items in the message.
func (m ChatMessage) FunctionCalls() []FunctionCallContent {
	var calls []FunctionCallContent
	for _, c := range m.Contents {
		if fc, ok := c.(FunctionCallContent); ok {
			calls = append(calls, fc)
		}
	}

	return calls
}

// FunctionResults returns all FunctionResultContent items in the message.
func (m ChatMessage) FunctionResults() []FunctionResultContent {
	var results []FunctionResultContent
	for _, c := range m.Contents {
		if fr, ok := c.(FunctionResultContent); ok {
			results = append(results, fr)
		}
	}

	return results
}

// HasFunctionCalls returns true if the message contains any function calls.
func (m ChatMessage) HasFunctionCalls() bool {
	for _, c := range m.Contents {
		if _, ok := c.(FunctionCallContent); ok {
			return true
		}
	}

	return false
}

// NewUserMessage creates a user message with text content.
func NewUserMessage(text string) ChatMessage {
	return ChatMessage{
		Role:     ChatRoleUser,
		Contents: []AIContent{TextContent{Text: text}},
	}
}

// NewAssistantMessage creates an assistant message with text content.
func NewAssistantMessage(text string, authorName string) ChatMessage {
	return ChatMessage{
		Role:       ChatRoleAssistant,
		AuthorName: authorName,
		Contents:   []AIContent{TextContent{Text: text}},
	}
}

// NewSystemMessage creates a system message with text content.
func NewSystemMessage(text string) ChatMessage {
	return ChatMessage{
		Role:     ChatRoleSystem,
		Contents: []AIContent{TextContent{Text: text}},
	}
}

// NewToolResultMessage creates a tool result message.
func NewToolResultMessage(callID, result string) ChatMessage {
	return ChatMessage{
		Role: ChatRoleTool,
		Contents: []AIContent{FunctionResultContent{
			ToolCallID: callID,
			Result:     result,
		}},
	}
}

// NewToolErrorMessage creates a tool error message.
func NewToolErrorMessage(callID, errorMsg string) ChatMessage {
	return ChatMessage{
		Role: ChatRoleTool,
		Contents: []AIContent{FunctionResultContent{
			ToolCallID: callID,
			Error:      errorMsg,
		}},
	}
}
