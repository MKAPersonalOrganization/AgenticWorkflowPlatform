package domain

import "school-with-ai/sdk/agentsdk/core"

// Commands represent user intentions to change the system state.
// Commands can be rejected if business rules are violated.
// When accepted, commands result in one or more events being emitted.

// CreateThread command creates a new conversation thread.
type CreateThread struct {
	ThreadID string
	UserID   string
	Title    string
}

// AddMessage command adds a message to a thread.
type AddMessage struct {
	ThreadID   string
	MessageID  string
	Role       string
	Content    string
	ToolCalls  []core.ToolCall
	ToolCallID string
	AgentName  string
	Metadata   map[string]any
}

// ChangeThreadTitle command updates a thread's title.
type ChangeThreadTitle struct {
	ThreadID string
	NewTitle string
}

// ClearMessages command removes all messages from a thread.
type ClearMessages struct {
	ThreadID string
}

// DeleteThread command marks a thread as deleted.
type DeleteThread struct {
	ThreadID string
}
