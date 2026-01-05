package sdk

import (
	"time"
)

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
	Projection

	// GetThreadState retrieves the UI state for a thread.
	// Returns nil if thread doesn't exist.
	GetThreadState(threadID string) (*AguiThreadState, bool)

	// GetActiveRun retrieves information about the currently active run for a thread.
	// Returns nil if no run is active.
	GetActiveRun(threadID string) (*AguiRunState, bool)

	// GetMessages retrieves all UI-formatted messages for a thread.
	GetMessages(threadID string) []AguiMessage

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
	Messages          []AguiMessage                 `json:"messages"`
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

// AguiMessage represents a message in the UI format.
type AguiMessage struct {
	ID        string                 `json:"id"`
	Role      string                 `json:"role"` // "user", "assistant", "system", "tool"
	Content   string                 `json:"content"`
	AgentName string                 `json:"agentName,omitempty"`
	ToolCalls []AguiMessageToolCall  `json:"toolCalls,omitempty"`
	ToolCall  *AguiMessageToolResult `json:"toolCall,omitempty"` // For tool result messages
	Timestamp time.Time              `json:"timestamp"`
	Status    string                 `json:"status,omitempty"` // "streaming", "complete"
	Metadata  map[string]any         `json:"metadata,omitempty"`
}

// AguiMessageToolCall represents a tool call within a message.
type AguiMessageToolCall struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// AguiMessageToolResult represents tool call information in a tool result message.
type AguiMessageToolResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Result string `json:"result"`
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

// Custom event types for AG-UI protocol
const (
	AguiEventTypeWhiteboardUpdated     = "WHITEBOARD_UPDATED"
	AguiEventTypeMathQuestion          = "MATH_QUESTION"
	AguiEventTypePhysicsQuestion       = "PHYSICS_QUESTION"
	AguiEventTypeChemistryQuestion     = "CHEMISTRY_QUESTION"
	AguiEventTypeStatisticsQuestion    = "STATISTICS_QUESTION"
	AguiEventTypeLinearAlgebraQuestion = "LINEARALGEBRA_QUESTION"
	AguiEventTypeDiagrammingQuestion   = "DIAGRAMMING_QUESTION"
	AguiEventTypeQuestionAnswered      = "QUESTION_ANSWERED"
	AguiEventTypeAgentHandoff          = "AGENT_HANDOFF"
	AguiEventTypeToolCallAgent         = "TOOL_CALL_AGENT"
)
