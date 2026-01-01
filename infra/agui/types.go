package agui

import (
	"encoding/json"
	"fmt"
)

// This module contains the types for the Agent User Interaction Protocol Go SDK.
//
// In Python, all types inherit from ConfiguredBaseModel which provides:
//   - extra="allow": Allows additional fields (Go does this by default)
//   - alias_generator=to_camel: Converts snake_case to camelCase (we use struct tags)
//   - populate_by_name=True: Accepts both formats (Go uses single tag name)
//
// In Go, we achieve the same behavior through struct tags and standard JSON marshaling.

// FunctionCall represents the name and arguments of a function call.
//
// Function calls contain the name of the function to invoke and a JSON-encoded
// string of arguments. This structure is modeled after OpenAI's function calling
// format to maintain compatibility across providers.
type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ToolCall represents a tool call, modeled after OpenAI tool calls.
//
// Tool calls are embedded within assistant messages to indicate that the agent
// wants to invoke a tool. Each tool call has a unique ID that will be referenced
// by the corresponding tool result message.
type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // Typically "function"
	Function FunctionCall `json:"function"`
}

// BaseMessage represents a base message, modeled after OpenAI messages.
//
// All message types that follow the standard pattern embed BaseMessage to inherit
// the common id, role, content, and name fields. This mirrors the Python implementation
// where message classes inherit from BaseMessage.
type BaseMessage struct {
	ID      string  `json:"id"`
	Role    string  `json:"role"`
	Content *string `json:"content,omitempty"`
	Name    *string `json:"name,omitempty"`
}

// DeveloperMessage represents a developer message.
//
// Developer messages are typically used for internal system communication or
// debugging purposes. They are not part of the standard user-agent conversation
// flow and may be filtered out in production environments.
type DeveloperMessage struct {
	BaseMessage
	Content string `json:"content"` // Required for developer messages
}

// SystemMessage represents a system message.
//
// System messages are used to provide instructions, context, or behavioral
// guidelines to the AI agent. They typically set the agent's personality,
// capabilities, or constraints.
type SystemMessage struct {
	BaseMessage
	Content string `json:"content"` // Required for system messages
}

// AssistantMessage represents an assistant message.
//
// Assistant messages contain the AI's responses to user queries. They can include
// both text content and tool calls. When tool calls are present, the content may
// be optional.
type AssistantMessage struct {
	BaseMessage
	ToolCalls []ToolCall `json:"toolCalls,omitempty"`
	AgentName *string    `json:"agentName,omitempty"` // Name of the agent that created this message
}

// TextInputContent represents a text fragment in a multimodal user message.
type TextInputContent struct {
	Type string `json:"type"` // "text"
	Text string `json:"text"`
}

// BinaryInputContent represents a binary payload reference in a multimodal user message.
//
// At least one of ID, URL, or Data must be provided to reference the payload.
type BinaryInputContent struct {
	Type     string  `json:"type"` // "binary"
	MimeType string  `json:"mimeType"`
	ID       *string `json:"id,omitempty"`
	URL      *string `json:"url,omitempty"`
	Data     *string `json:"data,omitempty"`
	Filename *string `json:"filename,omitempty"`
}

// InputContent represents content that can be either text or binary.
// This is a discriminated union based on the "type" field.
type InputContent interface{}

// UserMessage represents a user message supporting text or multimodal content.
//
// User messages contain input from the end user. The content can be either a
// simple string for text-only messages, or an array of InputContent objects for
// multimodal messages.
type UserMessage struct {
	BaseMessage
	Content interface{} `json:"content"` // Can be string or []InputContent
}

// ToolMessage represents a tool result message.
//
// Tool messages provide the results of tool calls back to the agent. Each tool
// message references a specific tool call via ToolCallID.
//
// Note: Does NOT inherit from BaseMessage (matches Python implementation).
type ToolMessage struct {
	ID         string  `json:"id"`
	Role       string  `json:"role"` // "tool"
	Content    string  `json:"content"`
	ToolCallID string  `json:"toolCallId"`
	Error      *string `json:"error,omitempty"`
}

// ActivityMessage represents an activity progress message emitted between chat messages.
//
// Activity messages are frontend-only and are never forwarded to the agent.
// They support live, updateable UI components for progress tracking.
//
// Note: Does NOT inherit from BaseMessage (matches Python implementation).
type ActivityMessage struct {
	ID           string                 `json:"id"`
	Role         string                 `json:"role"` // "activity"
	ActivityType string                 `json:"activityType"`
	Content      map[string]interface{} `json:"content"`
}

// Message is a discriminated union of all message types.
//
// In Python, this is represented as:
//
//	Message = Annotated[Union[...], Field(discriminator="role")]
//
// In Go, we use interface{} and provide UnmarshalMessage for deserialization.
type Message interface{}

// UnmarshalMessage deserializes a message JSON into the appropriate message type
// based on the "role" field (discriminator).
func UnmarshalMessage(data []byte) (Message, error) {
	var base struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(data, &base); err != nil {
		return nil, fmt.Errorf("failed to extract role: %w", err)
	}

	switch base.Role {
	case "developer":
		var msg DeveloperMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "system":
		var msg SystemMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "assistant":
		var msg AssistantMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "user":
		var msg UserMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "tool":
		var msg ToolMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	case "activity":
		var msg ActivityMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, err
		}
		return msg, nil
	default:
		return nil, fmt.Errorf("unknown message role: %s", base.Role)
	}
}

// Role represents the possible role values for messages.
type Role string

const (
	RoleDeveloper Role = "developer"
	RoleSystem    Role = "system"
	RoleAssistant Role = "assistant"
	RoleUser      Role = "user"
	RoleTool      Role = "tool"
	RoleActivity  Role = "activity"
)

// Context represents additional context for the agent.
type Context struct {
	Description string `json:"description"`
	Value       string `json:"value"`
}

// Tool represents a tool definition.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"` // JSON Schema for the tool parameters
}

// RunAgentInput represents the input for running an agent.
type RunAgentInput struct {
	ThreadID       string      `json:"threadId"`
	RunID          string      `json:"runId"`
	ParentRunID    *string     `json:"parentRunId,omitempty"`
	State          interface{} `json:"state"`
	Messages       []Message   `json:"messages"`
	Tools          []Tool      `json:"tools"`
	Context        []Context   `json:"context"`
	ForwardedProps interface{} `json:"forwardedProps"`
}

// State can be any type
type State = interface{}
