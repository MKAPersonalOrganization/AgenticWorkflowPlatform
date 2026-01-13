package taskmgmt

import "time"

// A2A (Agent-to-Agent) Protocol Types
// Specification: https://a2a.ag2ai.org/

// TaskState defines the possible lifecycle states of a Task.
type TaskState string

const (
	TaskStateUnspecified   TaskState = ""               // Missing TaskState value
	TaskStateSubmitted     TaskState = "submitted"      // Task has been submitted and is awaiting execution
	TaskStateWorking       TaskState = "working"        // Agent is actively working on the task
	TaskStateCompleted     TaskState = "completed"      // Task has been successfully completed (terminal)
	TaskStateFailed        TaskState = "failed"         // Task failed due to an error (terminal)
	TaskStateCanceled      TaskState = "canceled"       // Task was canceled by the user (terminal)
	TaskStateInputRequired TaskState = "input-required" // Task is paused and waiting for input (interrupted)
	TaskStateRejected      TaskState = "rejected"       // Task was rejected by agent and not started (terminal)
	TaskStateAuthRequired  TaskState = "auth-required"  // Authentication needed
	TaskStateUnknown       TaskState = "unknown"        // Unknown or indeterminate state
)

// TaskStatus represents the status of a task at a specific point in time.
type TaskStatus struct {
	State     TaskState  `json:"state"`               // Current state of the task
	Message   *Message   `json:"message,omitempty"`   // Optional message providing more details
	Timestamp *time.Time `json:"timestamp,omitempty"` // Datetime when status was recorded
}

// Message represents a single message in the conversation.
type Message struct {
	ID        string         `json:"messageId"`           // Unique identifier for the message
	ContextID string         `json:"contextId,omitempty"` // Context identifier
	TaskID    string         `json:"taskId,omitempty"`    // Optional task reference
	Role      string         `json:"role"`                // "user" or "agent"
	Parts     []any          `json:"parts"`               // Content parts
	Metadata  map[string]any `json:"metadata,omitempty"`  // Optional metadata
}

// Artifact represents output from a task.
type Artifact struct {
	Name        string `json:"name,omitempty"`        // Name of the artifact
	Type        string `json:"type,omitempty"`        // MIME type or artifact type
	Content     string `json:"content,omitempty"`     // Artifact content
	URL         string `json:"url,omitempty"`         // URL to access the artifact
	Description string `json:"description,omitempty"` // Human-readable description
}

// Task is the core unit of action for A2A.
type Task struct {
	ID        string         `json:"id"`                  // Unique identifier (UUID) for the task
	ContextID string         `json:"contextId"`           // Unique identifier for the contextual collection (threadID)
	Status    TaskStatus     `json:"status"`              // Current status of the task
	Artifacts []Artifact     `json:"artifacts,omitempty"` // Output artifacts
	History   []*Message     `json:"history,omitempty"`   // History of interactions (messages)
	Metadata  map[string]any `json:"metadata,omitempty"`  // Custom metadata
}

// NewTaskStatus creates a new TaskStatus with current timestamp.
func NewTaskStatus(state TaskState, msg *Message) TaskStatus {
	now := time.Now().UTC()

	return TaskStatus{
		State:     state,
		Message:   msg,
		Timestamp: &now,
	}
}

// NewTask creates a new A2A Task.
func NewTask(id, contextID string, state TaskState) Task {
	return Task{
		ID:        id,
		ContextID: contextID,
		Status:    NewTaskStatus(state, nil),
		Artifacts: []Artifact{},
		History:   []*Message{},
		Metadata:  make(map[string]any),
	}
}
