package taskmgmt

import "time"

// Task Management Types

// TaskState defines the possible lifecycle states of a Task.
type TaskState string

const (
	TaskStateSubmitted TaskState = "submitted" // Task has been submitted and is awaiting execution
	TaskStateWorking   TaskState = "working"   // Agent is actively working on the task
	TaskStateCompleted TaskState = "completed" // Task has been successfully completed (terminal)
	TaskStateFailed    TaskState = "failed"    // Task failed due to an error (terminal)
	TaskStateUnknown   TaskState = "unknown"   // Unknown or indeterminate state
)

// TaskStatus represents the status of a task at a specific point in time.
type TaskStatus struct {
	State     TaskState  `json:"state"`               // Current state of the task
	Timestamp *time.Time `json:"timestamp,omitempty"` // Datetime when status was recorded
}

// Task represents an asynchronous task submitted to the system.
type Task struct {
	ID        string         `json:"id"`        // Unique identifier (UUID) for the task
	ContextID string         `json:"contextId"` // Unique identifier for the contextual collection (threadID)
	Status    TaskStatus     `json:"status"`    // Current status of the task
	Metadata  map[string]any `json:"metadata"`  // Custom metadata
}

// NewTaskStatus creates a new TaskStatus with current timestamp.
func NewTaskStatus(state TaskState) TaskStatus {
	now := time.Now().UTC()

	return TaskStatus{
		State:     state,
		Timestamp: &now,
	}
}

// NewTask creates a new Task.
func NewTask(id, contextID string, state TaskState) Task {
	return Task{
		ID:        id,
		ContextID: contextID,
		Status:    NewTaskStatus(state),
		Metadata:  make(map[string]any),
	}
}
