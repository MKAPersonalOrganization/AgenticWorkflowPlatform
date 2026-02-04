package llm

import "encoding/json"

// Content is the base interface for all content types in a chat message.
// Messages can contain multiple content items of different types.
type Content interface {
	// ContentType returns a discriminator string identifying the content type.
	ContentType() string
	isContent() // Marker method to restrict implementations
}

// TextContent represents plain text content in a message.
type TextContent struct {
	Text string
}

// ContentType implements Content.
func (TextContent) ContentType() string { return "text" }
func (TextContent) isContent()          {}

// BinaryContent is binary data with MIME type.
type BinaryContent struct {
	Data     []byte
	MimeType string
	Name     string // Optional filename
}

// ContentType implements Content.
func (BinaryContent) ContentType() string { return "binary" }
func (BinaryContent) isContent()          {}

// JSONContent is structured JSON data.
type JSONContent struct {
	Data json.RawMessage
}

// ContentType implements Content.
func (JSONContent) ContentType() string { return "json" }
func (JSONContent) isContent()          {}

// FunctionCall represents a request to invoke a function.
type FunctionCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

// ContentType implements Content.
func (FunctionCall) ContentType() string { return "function_call" }
func (FunctionCall) isContent()          {}

// FunctionResult is the result of a function invocation.
type FunctionResult struct {
	CallID  string
	Name    string
	Result  json.RawMessage
	IsError bool
}

// ContentType implements Content.
func (FunctionResult) ContentType() string { return "function_result" }
func (FunctionResult) isContent()          {}

// Ensure all content types implement Content at compile time.
var (
	_ Content = TextContent{}
	_ Content = BinaryContent{}
	_ Content = JSONContent{}
	_ Content = FunctionCall{}
	_ Content = FunctionResult{}
)
