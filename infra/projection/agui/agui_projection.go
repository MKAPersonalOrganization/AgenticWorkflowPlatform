package agui

import (
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// EventEmitter is a function that emits real-time events to connected clients.
type EventEmitter func(threadID string, event any) error

// AguiProjection is a read model optimized for the AG-UI protocol.
type AguiProjection interface {
	// GetThreadState retrieves the UI state for a thread.
	GetThreadState(threadID string) (*AguiThreadState, bool)

	// GetMessages retrieves all UI-formatted messages for a thread.
	GetMessages(threadID string) []aguievents.Message

	// ListThreads returns paginated threads for a user.
	ListThreads(req ThreadListRequest) (*ThreadListResponse, error)
}

// AguiBroadcaster is an interface for subscribing to real-time events.
type AguiBroadcaster interface {
	// Subscribe subscribes to real-time events for a thread.
	// Returns a channel of events (type core.Event) and a cancel function.
	Subscribe(threadID string) (<-chan any, func())
}

// AguiThreadState represents the complete UI state for a thread.
type AguiThreadState struct {
	ThreadID      string                        `json:"threadId"`
	Title         string                        `json:"title,omitempty"`
	Messages      []aguievents.Message          `json:"messages"` // AG-UI SDK format
	ToolCalls     []AguiToolCall                `json:"toolCalls"`
	AgentActivity map[string]*AguiAgentActivity `json:"agentActivity"`
	CreatedAt     time.Time                     `json:"createdAt"`
	UpdatedAt     time.Time                     `json:"updatedAt"`
	Version       int64                         `json:"version"` // Event version
	IsDeleted     bool                          `json:"isDeleted"`
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

// ThreadListRequest contains pagination parameters for listing threads.
type ThreadListRequest struct {
	UserID string // Required: filter by user
	Cursor string // Optional: RFC3339 timestamp of last item's updated_at
	Limit  int    // Optional: max items to return (default 20, max 100)
}

// ThreadListResponse contains paginated thread results.
type ThreadListResponse struct {
	Threads    []ThreadSummary `json:"threads"`
	NextCursor string          `json:"nextCursor,omitempty"`
	HasMore    bool            `json:"hasMore"`
}

// ThreadSummary is a lightweight thread representation for list views.
type ThreadSummary struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	UpdatedAt time.Time `json:"updatedAt"`
}
