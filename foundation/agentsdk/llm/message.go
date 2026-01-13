package llm

import (
	"strings"
)

// Message represents a message in the domain layer.
// Duplicated here to avoid circular import with core.
type Message struct {
	ID         string
	Role       string
	Content    string
	Timestamp  interface{} // time.Time
	ToolCalls  []ToolCall
	ToolCallID string
	AgentName  string
	Metadata   map[string]any
}

// ToolCall represents a tool call.
type ToolCall struct {
	ID       string
	Type     string
	Function ToolFunction
}

// ToolFunction represents a function in a tool call.
type ToolFunction struct {
	Name      string
	Arguments string
}

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
			CallID: callID,
			Result: result,
		}},
	}
}

// NewToolErrorMessage creates a tool error message.
func NewToolErrorMessage(callID, errorMsg string) ChatMessage {
	return ChatMessage{
		Role: ChatRoleTool,
		Contents: []AIContent{FunctionResultContent{
			CallID: callID,
			Error:  errorMsg,
		}},
	}
}

// FromDomainMessage converts a Message to ChatMessage.
// This bridges the persistence layer (Message) with the LLM layer (ChatMessage).
func FromDomainMessage(msg Message) ChatMessage {
	cm := ChatMessage{
		Role:       ChatRole(msg.Role),
		AuthorName: msg.AgentName,
		Contents:   make([]AIContent, 0),
	}

	// Add text content if present
	if msg.Content != "" && msg.Role != "tool" {
		cm.Contents = append(cm.Contents, TextContent{Text: msg.Content})
	}

	// Add function calls from tool calls
	for _, tc := range msg.ToolCalls {
		cm.Contents = append(cm.Contents, FunctionCallContent{
			CallID:       tc.ID,
			Name:         tc.Function.Name,
			RawArguments: tc.Function.Arguments,
		})
	}

	// Add function result for tool messages
	if msg.Role == "tool" && msg.ToolCallID != "" {
		cm.Contents = append(cm.Contents, FunctionResultContent{
			CallID: msg.ToolCallID,
			Result: msg.Content,
		})
	}

	return cm
}

// ToDomainMessage converts a ChatMessage to Message.
// This bridges the LLM layer (ChatMessage) with the persistence layer (Message).
func ToDomainMessage(cm ChatMessage) Message {
	msg := Message{
		Role:      string(cm.Role),
		AgentName: cm.AuthorName,
		Content:   cm.Text(),
	}

	// Convert function calls to tool calls
	for _, fc := range cm.FunctionCalls() {
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:   fc.CallID,
			Type: "function",
			Function: ToolFunction{
				Name:      fc.Name,
				Arguments: fc.ArgumentsJSON(),
			},
		})
	}

	// Handle tool result messages
	for _, fr := range cm.FunctionResults() {
		msg.ToolCallID = fr.CallID
		if fr.IsError() {
			msg.Content = fr.Error
		} else {
			msg.Content = fr.Result
		}
	}

	return msg
}

// FromDomainMessages converts a slice of Message to ChatMessage.
// Filters out system messages with agent_handoff metadata as they're UI-only.
func FromDomainMessages(msgs []Message) []ChatMessage {
	result := make([]ChatMessage, 0, len(msgs))
	for _, m := range msgs {
		// Skip agent handoff messages - they're for UI display only
		if m.Metadata != nil {
			if msgType, ok := m.Metadata["type"].(string); ok && msgType == "agent_handoff" {
				continue
			}
		}
		result = append(result, FromDomainMessage(m))
	}

	return result
}

// ToDomainMessages converts a slice of ChatMessage to Message.
func ToDomainMessages(msgs []ChatMessage) []Message {
	result := make([]Message, len(msgs))
	for i, m := range msgs {
		result[i] = ToDomainMessage(m)
	}

	return result
}
