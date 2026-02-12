package tasks

import (
	"github.com/google/uuid"
)

// TaskResponse represents a single task returned by the API.
type TaskResponse struct {
	TrackID      string                 `json:"track_id"`
	TenantID     string                 `json:"tenant_id"`
	WorkspaceID  string                 `json:"workspace_id"`
	TaskType     string                 `json:"task_type"`
	Status       string                 `json:"status"`
	CreatedAt    string                 `json:"created_at"`
	UpdatedAt    string                 `json:"updated_at"`
	StartedAt    *string                `json:"started_at,omitempty"`
	CompletedAt  *string                `json:"completed_at,omitempty"`
	ErrorMessage *string                `json:"error_message,omitempty"`
	Error        *TaskErrorResponse     `json:"error,omitempty"`
	RetryCount   int32                  `json:"retry_count"`
	MaxRetries   int32                  `json:"max_retries"`
	Progress     map[string]interface{} `json:"progress,omitempty"`
	Result       map[string]interface{} `json:"result,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TaskErrorResponse represents detailed error information for a failed task.
type TaskErrorResponse struct {
	Message    string `json:"message"`
	Step       string `json:"step"`
	Reason     string `json:"reason"`
	Suggestion string `json:"suggestion"`
	Retryable  bool   `json:"retryable"`
}

// TaskListResponse represents a paginated list of tasks with statistics.
type TaskListResponse struct {
	Tasks      []TaskResponse `json:"tasks"`
	Pagination PaginationInfo `json:"pagination"`
	Statistics StatisticsInfo `json:"statistics"`
}

// PaginationInfo represents pagination metadata.
type PaginationInfo struct {
	Total      uint64 `json:"total"`
	Page       uint32 `json:"page"`
	PageSize   uint32 `json:"page_size"`
	TotalPages uint32 `json:"total_pages"`
}

// StatisticsInfo represents task statistics by status.
type StatisticsInfo struct {
	Pending    uint64 `json:"pending"`
	Processing uint64 `json:"processing"`
	Indexed    uint64 `json:"indexed"`
	Failed     uint64 `json:"failed"`
	Cancelled  uint64 `json:"cancelled"`
}

// ListTasksParams represents query parameters for listing tasks.
type ListTasksParams struct {
	TenantID    *uuid.UUID
	WorkspaceID *uuid.UUID
	Status      *string
	TaskType    *string
	Page        *uint32
	PageSize    *uint32
	Sort        *string
	Order       *string
}
