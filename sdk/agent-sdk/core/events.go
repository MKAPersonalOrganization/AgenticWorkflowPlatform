package sdk

// Domain events emitted by the agent

// TextMessageStartEvent indicates the start of a text message
type TextMessageStartEvent struct {
	MessageID string
	Role      string
}

// TextMessageContentEvent contains a delta of text content
type TextMessageContentEvent struct {
	MessageID string
	Delta     string
}

// TextMessageEndEvent indicates the end of a text message
type TextMessageEndEvent struct {
	MessageID string
}

// ToolCallStartEvent indicates the start of a tool call
type ToolCallStartEvent struct {
	ToolCallID      string
	ToolCallName    string
	ParentMessageID string
	AgentName       string
}

// ToolCallArgsEvent contains a delta of tool call arguments
type ToolCallArgsEvent struct {
	ToolCallID string
	Delta      string
}

// ToolCallEndEvent indicates the end of a tool call
type ToolCallEndEvent struct {
	ToolCallID string
}

// ToolCallResultEvent contains the result of a tool execution
type ToolCallResultEvent struct {
	MessageID  string
	ToolCallID string
	Content    string
	Role       string
}
