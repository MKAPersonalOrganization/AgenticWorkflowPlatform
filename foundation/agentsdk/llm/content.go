package llm

import "encoding/json"

// AIContent is the base interface for all content types in a chat message.
// Messages can contain multiple content items of different types.
type AIContent interface {
	// ContentType returns a discriminator string identifying the content type.
	ContentType() string
}

// TextContent represents plain text content in a message.
type TextContent struct {
	Text string `json:"text"`
}

// ContentType implements AIContent.
func (t TextContent) ContentType() string { return "text" }

// FunctionCallContent represents a function/tool call request from the model.
type FunctionCallContent struct {
	// ToolCallID is the unique identifier for this function call.
	ToolCallID string `json:"toolCallId"`

	// Name is the name of the function to call.
	Name string `json:"name"`

	// Arguments contains the parsed function arguments.
	Arguments map[string]any `json:"arguments,omitempty"`

	// RawArguments contains the raw JSON string of arguments.
	RawArguments string `json:"argumentsJson,omitempty"`
}

// ContentType implements AIContent.
func (f FunctionCallContent) ContentType() string { return "functionCall" }

// ArgumentsJSON returns the arguments as a JSON string.
func (f FunctionCallContent) ArgumentsJSON() string {
	if f.RawArguments != "" {
		return f.RawArguments
	}
	if f.Arguments == nil {
		return "{}"
	}
	data, err := json.Marshal(f.Arguments)
	if err != nil {
		return "{}"
	}

	return string(data)
}

// FunctionResultContent represents the result of a function call execution.
type FunctionResultContent struct {
	// ToolCallID references the FunctionCallContent.ToolCallID that this is a result for.
	ToolCallID string `json:"toolCallId"`

	// Name is the name of the function to call.
	Name string `json:"name,omitempty"`

	// Result contains the function result, typically as a JSON string.
	Result string `json:"result"`

	// Error contains an error message if the function call failed.
	Error string `json:"error,omitempty"`
}

// ContentType implements AIContent.
func (f FunctionResultContent) ContentType() string { return "functionResult" }

// IsError returns true if the function result represents an error.
func (f FunctionResultContent) IsError() bool {
	return f.Error != ""
}

// UsageContent represents token usage information.
type UsageContent struct {
	InputTokens  int `json:"inputTokens"`
	OutputTokens int `json:"outputTokens"`
	TotalTokens  int `json:"totalTokens"`
}

// ContentType implements AIContent.
func (u UsageContent) ContentType() string { return "usage" }

// Ensure all content types implement AIContent at compile time.
var (
	_ AIContent = TextContent{}
	_ AIContent = FunctionCallContent{}
	_ AIContent = FunctionResultContent{}
	_ AIContent = UsageContent{}
)
