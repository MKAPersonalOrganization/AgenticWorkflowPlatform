package llm

import "encoding/json"

// AIContent is the base interface for all content types in a chat message.
// This mirrors Microsoft.Extensions.AI.AIContent from the .NET agent framework.
//
// Content types include text, function calls, function results, and more.
// Messages can contain multiple content items of different types.
type AIContent interface {
	// ContentType returns a discriminator string identifying the content type.
	// Used for type switching and serialization.
	ContentType() string
}

// TextContent represents plain text content in a message.
// This is the most common content type for user and assistant messages.
type TextContent struct {
	Text string `json:"text"`
}

// ContentType implements AIContent.
func (t TextContent) ContentType() string { return "text" }

// FunctionCallContent represents a function/tool call request from the model.
// The model generates this when it wants to invoke a tool.
type FunctionCallContent struct {
	// CallID is the unique identifier for this function call.
	// Used to match results back to the original call.
	CallID string `json:"callId"`

	// Name is the name of the function to call.
	Name string `json:"name"`

	// Arguments contains the parsed function arguments.
	// May be nil if arguments are still being streamed.
	Arguments map[string]any `json:"arguments,omitempty"`

	// ArgumentsJSON contains the raw JSON string of arguments.
	// Useful when Arguments cannot be parsed or during streaming.
	ArgumentsJSON string `json:"argumentsJson,omitempty"`
}

// ContentType implements AIContent.
func (f FunctionCallContent) ContentType() string { return "functionCall" }

// GetArgumentsJSON returns the arguments as a JSON string.
// If ArgumentsJSON is set, returns it directly.
// Otherwise marshals Arguments to JSON.
func (f FunctionCallContent) GetArgumentsJSON() string {
	if f.ArgumentsJSON != "" {
		return f.ArgumentsJSON
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
// This is added to the conversation after a tool has been executed.
type FunctionResultContent struct {
	// CallID references the FunctionCallContent.CallID that this is a result for.
	CallID string `json:"callId"`

	// Result contains the function result, typically as a JSON string.
	Result string `json:"result"`

	// Error contains an error message if the function call failed.
	// Empty string indicates success.
	Error string `json:"error,omitempty"`
}

// ContentType implements AIContent.
func (f FunctionResultContent) ContentType() string { return "functionResult" }

// IsError returns true if the function result represents an error.
func (f FunctionResultContent) IsError() bool {
	return f.Error != ""
}

// UsageContent represents token usage information.
// Typically included in the final response update.
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
