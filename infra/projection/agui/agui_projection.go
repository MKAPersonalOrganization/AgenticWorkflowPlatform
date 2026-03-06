package agui

import (
	"context"
	"school-with-ai/domain"
	"time"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// EventEmitter is a function that emits real-time events to connected clients.
type EventEmitter func(threadID string, event any) error

// AguiProjection is a read model optimized for the AG-UI protocol.
type AguiProjection interface {
	// GetThreadState retrieves the UI state for a thread.
	GetThreadState(ctx context.Context, threadID string) (*AguiThreadState, bool)

	// GetMessages retrieves all UI-formatted messages for a thread.
	GetMessages(ctx context.Context, threadID string) []aguievents.Message

	// ListThreads returns paginated threads for a user.
	ListThreads(ctx context.Context, req ThreadListRequest) (*ThreadListResponse, error)
}

// AguiBroadcaster is an interface for subscribing to real-time events.
type AguiBroadcaster interface {
	// Subscribe subscribes to real-time events for a thread.
	// Returns a channel of events (type domain.Event) and a cancel function.
	Subscribe(threadID string) (<-chan domain.Event, func())
}

// AguiThreadState represents the complete UI state for a thread.
type AguiThreadState struct {
	ThreadID           string                         `json:"threadId"`
	UserID             string                         `json:"userId"`
	Title              string                         `json:"title,omitempty"`
	Messages           []aguievents.Message           `json:"messages"` // AG-UI SDK format
	ToolCalls          []AguiToolCall                 `json:"toolCalls"`
	AgentActivity      map[string]*AguiAgentActivity  `json:"agentActivity"`
	ToolStates         map[string]any                 `json:"toolStates,omitempty"`
	TokenUsage         *TokenUsageInfo                `json:"tokenUsage,omitempty"`
	MessageAttachments map[string][]domain.Attachment `json:"messageAttachments,omitempty"` // messageID -> attachment refs
	CreatedAt          time.Time                      `json:"createdAt"`
	UpdatedAt          time.Time                      `json:"updatedAt"`
	Version            int64                          `json:"version"` // Event version
	IsDeleted          bool                           `json:"isDeleted"`
}

// TokenUsageInfo represents token usage for a thread.
type TokenUsageInfo struct {
	Total           int    `json:"total"`
	Prompt          int    `json:"prompt"`
	Completion      int    `json:"completion"`
	Cached          int    `json:"cached,omitempty"`
	Reasoning       int    `json:"reasoning,omitempty"`
	AudioInput      int    `json:"audioInput,omitempty"`
	AudioOutput     int    `json:"audioOutput,omitempty"`
	AcceptedPredict int    `json:"acceptedPredict,omitempty"`
	RejectedPredict int    `json:"rejectedPredict,omitempty"`
	Model           string `json:"model,omitempty"`
	Provider        string `json:"provider,omitempty"`
	MessageID       string `json:"messageId,omitempty"`
	ThreadID        string `json:"threadId,omitempty"`
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
	Query  string // Optional: search threads by title (case-insensitive)
}

// ThreadListResponse contains paginated thread results.
type ThreadListResponse struct {
	Threads    []ThreadSummary `json:"threads"`
	NextCursor string          `json:"nextCursor,omitempty"`
	HasMore    bool            `json:"hasMore"`
}

// ThreadSummary is a lightweight thread representation for list views.
type ThreadSummary struct {
	ID         string    `json:"id"`
	Title      string    `json:"title,omitempty"`
	GroupTitle string    `json:"groupTitle,omitempty"` // Localized date group title
	UpdatedAt  time.Time `json:"updatedAt"`
}
