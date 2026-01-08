package agui

import (
	"time"

	"school-with-ai/infra/eventstore"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// EventEmitter is a function that emits real-time events to connected clients.
type EventEmitter func(threadID string, event interface{}) error

// AguiProjection is a read model optimized for the AG-UI protocol.
//
// This projection maintains UI-specific state derived from domain events,
// making it efficient to serve UI queries without processing the entire event stream.
//
// Key responsibilities:
//   - Track active agent runs and their lifecycle
//   - Maintain streaming message state (in-progress, completed)
//   - Store tool call execution history
//   - Provide real-time agent activity snapshots
//   - Support UI features like message history, agent status, etc.
//
// This projection is rebuilt from the EventStore on startup and updated
// as new events are appended.
type AguiProjection interface {
	eventstore.Projection

	// GetThreadState retrieves the UI state for a thread.
	// Returns nil if thread doesn't exist.
	GetThreadState(threadID string) (*AguiThreadState, bool)

	// GetActiveRun retrieves information about the currently active run for a thread.
	// Returns nil if no run is active.
	GetActiveRun(threadID string) (*AguiRunState, bool)

	// GetMessages retrieves all UI-formatted messages for a thread.
	GetMessages(threadID string) []aguievents.Message

	// GetToolCalls retrieves all tool calls for a thread.
	GetToolCalls(threadID string) []AguiToolCall

	// GetAgentActivity retrieves agent activity summary for a thread.
	GetAgentActivity(threadID string) map[string]*AguiAgentActivity

	// ListThreads returns all thread IDs in the projection.
	ListThreads() []string
}

// AguiThreadState represents the complete UI state for a thread.
type AguiThreadState struct {
	ThreadID          string                        `json:"threadId"`
	Title             string                        `json:"title,omitempty"`
	Messages          []aguievents.Message          `json:"messages"` // AG-UI SDK format
	ActiveRun         *AguiRunState                 `json:"activeRun,omitempty"`
	ToolCalls         []AguiToolCall                `json:"toolCalls"`
	AgentActivity     map[string]*AguiAgentActivity `json:"agentActivity"`
	WhiteboardContent string                        `json:"whiteboardContent,omitempty"`
	CreatedAt         time.Time                     `json:"createdAt"`
	UpdatedAt         time.Time                     `json:"updatedAt"`
	Version           int64                         `json:"version"` // Event version
}

// AguiRunState tracks the lifecycle of an agent run.
type AguiRunState struct {
	RunID      string    `json:"runId"`
	AgentName  string    `json:"agentName"`
	Status     string    `json:"status"` // "running", "completed", "error"
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// NOTE: Messages are now stored using aguievents.Message (AG-UI SDK format) directly.
// This eliminates the need for conversion between internal and SDK formats.

// AguiToolCall represents a complete tool call execution.
type AguiToolCall struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Arguments       string    `json:"arguments"`
	Result          string    `json:"result,omitempty"`
	AgentName       string    `json:"agentName"`
	ParentMessageID string    `json:"parentMessageId"`
	Status          string    `json:"status"` // "started", "args_complete", "completed"
	StartedAt       time.Time `json:"startedAt"`
	CompletedAt     time.Time `json:"completedAt,omitempty"`
}

// AguiAgentActivity tracks an agent's activity in a thread.
type AguiAgentActivity struct {
	AgentName       string    `json:"agentName"`
	MessageCount    int       `json:"messageCount"`
	ToolCallCount   int       `json:"toolCallCount"`
	LastActiveAt    time.Time `json:"lastActiveAt"`
	FirstActiveAt   time.Time `json:"firstActiveAt"`
	TotalRunTime    int64     `json:"totalRunTime"` // milliseconds
	CurrentlyActive bool      `json:"currentlyActive"`
}
