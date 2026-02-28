package llm

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Message represents a single message in a conversation.
type Message struct {
	// Role indicates who authored this message.
	Role Role

	// Content contains the message content.
	Content []Content

	// Name is an optional author identifier for multi-agent scenarios.
	Name string

	// Metadata holds additional message data.
	Metadata Metadata

	// CreatedAt indicates when the message was created.
	CreatedAt time.Time
}

// Text returns the concatenated text content of the message.
// Only TextContent items are included; other content types are ignored.
func (m Message) Text() string {
	var sb strings.Builder
	for _, c := range m.Content {
		if tc, ok := c.(TextContent); ok {
			sb.WriteString(tc.Text)
		}
	}

	return sb.String()
}

// FunctionCalls returns all FunctionCall items in the message.
func (m Message) FunctionCalls() []FunctionCall {
	var calls []FunctionCall
	for _, c := range m.Content {
		if fc, ok := c.(FunctionCall); ok {
			calls = append(calls, fc)
		}
	}

	return calls
}

// FunctionResults returns all FunctionResult items in the message.
func (m Message) FunctionResults() []FunctionResult {
	var results []FunctionResult
	for _, c := range m.Content {
		if fr, ok := c.(FunctionResult); ok {
			results = append(results, fr)
		}
	}

	return results
}

// HasFunctionCalls returns true if the message contains any function calls.
func (m Message) HasFunctionCalls() bool {
	for _, c := range m.Content {
		if _, ok := c.(FunctionCall); ok {
			return true
		}
	}

	return false
}

// ImageURLs returns all ImageURLContent items in the message.
func (m Message) ImageURLs() []ImageURLContent {
	var urls []ImageURLContent
	for _, c := range m.Content {
		if img, ok := c.(ImageURLContent); ok {
			urls = append(urls, img)
		}
	}
	return urls
}

// HasImages returns true if the message contains any image content.
func (m Message) HasImages() bool {
	for _, c := range m.Content {
		if _, ok := c.(ImageURLContent); ok {
			return true
		}
	}
	return false
}

// AudioContents returns all AudioContent items in the message.
func (m Message) AudioContents() []AudioContent {
	var result []AudioContent
	for _, c := range m.Content {
		if a, ok := c.(AudioContent); ok {
			result = append(result, a)
		}
	}
	return result
}

// FileContents returns all FileContent items in the message.
func (m Message) FileContents() []FileContent {
	var result []FileContent
	for _, c := range m.Content {
		if f, ok := c.(FileContent); ok {
			result = append(result, f)
		}
	}
	return result
}

// HasMultimodal returns true if the message contains any non-text media
// (images, audio, or file attachments).
func (m Message) HasMultimodal() bool {
	for _, c := range m.Content {
		switch c.(type) {
		case ImageURLContent, AudioContent, FileContent:
			return true
		}
	}
	return false
}

// NewUserMessage creates a user message with text content.
func NewUserMessage(text string) Message {
	return Message{
		Role:      RoleUser,
		Content:   []Content{TextContent{Text: text}},
		CreatedAt: time.Now(),
	}
}

// NewAssistantMessage creates an assistant message with text content.
func NewAssistantMessage(text string, name string) Message {
	return Message{
		Role:      RoleAssistant,
		Name:      name,
		Content:   []Content{TextContent{Text: text}},
		CreatedAt: time.Now(),
	}
}

// NewSystemMessage creates a system message with text content.
func NewSystemMessage(text string) Message {
	return Message{
		Role:      RoleSystem,
		Content:   []Content{TextContent{Text: text}},
		CreatedAt: time.Now(),
	}
}

// NewToolResultMessage creates a tool result message.
func NewToolResultMessage(callID, name string, result json.RawMessage) Message {
	return Message{
		Role: RoleTool,
		Content: []Content{FunctionResult{
			CallID: callID,
			Name:   name,
			Result: result,
		}},
		CreatedAt: time.Now(),
	}
}

// NewToolErrorMessage creates a tool error message.
func NewToolErrorMessage(callID, name string, errorMsg string) Message {
	return Message{
		Role: RoleTool,
		Content: []Content{FunctionResult{
			CallID:  callID,
			Name:    name,
			Result:  json.RawMessage(fmt.Sprintf("%q", errorMsg)),
			IsError: true,
		}},
		CreatedAt: time.Now(),
	}
}
