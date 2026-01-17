package agui

import (
	"school-with-ai/infra/eventstore"
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// EventEmitter is a function that emits real-time events to connected clients.
type EventEmitter func(threadID string, event any) error

// AguiProjection is a read model optimized for the AG-UI protocol.
type AguiProjection interface {
	// GetThreadState retrieves the UI state for a thread.
	GetThreadState(threadID string) (*AguiThreadState, bool)

	// GetActiveRun retrieves information about the currently active run for a thread.
	GetActiveRun(threadID string) (*AguiRunState, bool)

	// GetMessages retrieves all UI-formatted messages for a thread.
	GetMessages(threadID string) []aguievents.Message

	// ListThreads returns all thread IDs in the projection.
	ListThreads() []string
}

// AguiThreadState represents the complete UI state for a thread.
type AguiThreadState struct {
	ThreadID          string                        `json:"threadId"`
	Title             string                        `json:"title,omitempty"`
	Messages          []aguievents.Message          `json:"messages"` // AG-UI SDK format
	ActiveRun         *AguiRunState                 `json:"activeRun"`
	ToolCalls         []AguiToolCall                `json:"toolCalls"`
	AgentActivity     map[string]*AguiAgentActivity `json:"agentActivity"`
	WhiteboardContent string                        `json:"whiteboardContent,omitempty"`
	CreatedAt         time.Time                     `json:"createdAt"`
	UpdatedAt         time.Time                     `json:"updatedAt"`
	Version           int64                         `json:"version"` // Event version
	IsDeleted         bool                          `json:"isDeleted"`
}

// AguiRunState tracks the lifecycle of an agent run.
type AguiRunState struct {
	RunID      string    `json:"runId"`
	AgentName  string    `json:"agentName"`
	Status     string    `json:"status"` // "running", "completed", "error"
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Error      string    `json:"error,omitempty"`
}

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
	CompletedAt     time.Time `json:"completedAt"`
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

// AguiProjectionImpl is an implementation of AguiProjection.
// We keep the implementation in infra, but the interface is in domain.
type AguiProjectionImpl interface {
	AguiProjection
	eventstore.Projection
}
