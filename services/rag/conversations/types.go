package conversations

import (
	"encoding/json"

	"github.com/google/uuid"
)

// ConversationResponse represents a conversation returned by the API.
type ConversationResponse struct {
	ID                 uuid.UUID  `json:"id"`
	TenantID           uuid.UUID  `json:"tenant_id"`
	WorkspaceID        *uuid.UUID `json:"workspace_id,omitempty"`
	Title              string     `json:"title"`
	Mode               string     `json:"mode"`
	IsPinned           bool       `json:"is_pinned"`
	IsArchived         bool       `json:"is_archived"`
	FolderID           *uuid.UUID `json:"folder_id,omitempty"`
	ShareID            *string    `json:"share_id,omitempty"`
	MessageCount       *int       `json:"message_count,omitempty"`
	LastMessagePreview *string    `json:"last_message_preview,omitempty"`
	CreatedAt          string     `json:"created_at"`
	UpdatedAt          string     `json:"updated_at"`
}

// MessageResponse represents a message returned by the API.
type MessageResponse struct {
	ID             uuid.UUID        `json:"id"`
	ConversationID uuid.UUID        `json:"conversation_id"`
	ParentID       *uuid.UUID       `json:"parent_id,omitempty"`
	Role           string           `json:"role"`
	Content        string           `json:"content"`
	Mode           *string          `json:"mode,omitempty"`
	TokensUsed     *int32           `json:"tokens_used,omitempty"`
	DurationMs     *int32           `json:"duration_ms,omitempty"`
	ThinkingTimeMs *int32           `json:"thinking_time_ms,omitempty"`
	Context        *json.RawMessage `json:"context,omitempty"`
	IsError        bool             `json:"is_error"`
	CreatedAt      string           `json:"created_at"`
	UpdatedAt      string           `json:"updated_at"`
}

// PaginationMetaResponse represents pagination metadata.
type PaginationMetaResponse struct {
	NextCursor *string `json:"next_cursor,omitempty"`
	PrevCursor *string `json:"prev_cursor,omitempty"`
	Total      *int    `json:"total,omitempty"`
	HasMore    bool    `json:"has_more"`
}

// PaginatedConversationsResponse represents a paginated list of conversations.
type PaginatedConversationsResponse struct {
	Items      []ConversationResponse `json:"items"`
	Pagination PaginationMetaResponse `json:"pagination"`
}

// PaginatedMessagesResponse represents a paginated list of messages.
type PaginatedMessagesResponse struct {
	Items      []MessageResponse      `json:"items"`
	Pagination PaginationMetaResponse `json:"pagination"`
}

// ConversationWithMessagesResponse represents a conversation with its messages.
type ConversationWithMessagesResponse struct {
	Conversation ConversationResponse `json:"conversation"`
	Messages     []MessageResponse    `json:"messages"`
}

// ShareResponse represents the share info for a conversation.
type ShareResponse struct {
	ShareID  string `json:"share_id"`
	ShareURL string `json:"share_url"`
}

// CreateConversationRequest represents the payload to create a new conversation.
type CreateConversationRequest struct {
	Title    *string    `json:"title,omitempty"`
	Mode     *string    `json:"mode,omitempty"`
	FolderID *uuid.UUID `json:"folder_id,omitempty"`
}

// UpdateConversationRequest represents the payload to update a conversation.
type UpdateConversationRequest struct {
	Title      *string    `json:"title,omitempty"`
	Mode       *string    `json:"mode,omitempty"`
	IsPinned   *bool      `json:"is_pinned,omitempty"`
	IsArchived *bool      `json:"is_archived,omitempty"`
	FolderID   *uuid.UUID `json:"folder_id,omitempty"`
}

// CreateMessageRequest represents the payload to create a new message.
type CreateMessageRequest struct {
	Content  string     `json:"content"`
	Role     string     `json:"role"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
	Stream   bool       `json:"stream"`
}

// UpdateMessageRequest represents the payload to update a message.
type UpdateMessageRequest struct {
	Content        *string          `json:"content,omitempty"`
	TokensUsed     *int32           `json:"tokens_used,omitempty"`
	DurationMs     *int32           `json:"duration_ms,omitempty"`
	ThinkingTimeMs *int32           `json:"thinking_time_ms,omitempty"`
	Context        *json.RawMessage `json:"context,omitempty"`
	IsError        *bool            `json:"is_error,omitempty"`
}

// BulkOperationRequest represents a bulk operation request.
type BulkOperationRequest struct {
	ConversationIDs []uuid.UUID `json:"conversation_ids"`
}

// BulkArchiveRequest represents a bulk archive request.
type BulkArchiveRequest struct {
	ConversationIDs []uuid.UUID `json:"conversation_ids"`
	Archive         bool        `json:"archive"`
}

// BulkMoveRequest represents a bulk move request.
type BulkMoveRequest struct {
	ConversationIDs []uuid.UUID `json:"conversation_ids"`
	FolderID        *uuid.UUID  `json:"folder_id,omitempty"`
}

// BulkOperationResponse represents the result of a bulk operation.
type BulkOperationResponse struct {
	Affected int `json:"affected"`
}

// ListConversationsParams represents query parameters for listing conversations.
type ListConversationsParams struct {
	Cursor         *string
	Limit          *int
	FilterMode     *string
	FilterArchived *bool
	FilterPinned   *bool
	FilterFolderID *uuid.UUID
	FilterUnfiled  *bool
	FilterSearch   *string
	Sort           *string
	Order          *string
}

// ListMessagesParams represents query parameters for listing messages.
type ListMessagesParams struct {
	Cursor *string
	Limit  *int
}

// ImportConversationsRequest represents a request to import conversations.
type ImportConversationsRequest struct {
	Conversations []json.RawMessage `json:"conversations"`
}

// ImportConversationsResponse represents the response from importing conversations.
type ImportConversationsResponse struct {
	Imported int                   `json:"imported"`
	Failed   int                   `json:"failed"`
	Errors   []ImportErrorResponse `json:"errors"`
}

// ImportErrorResponse represents a single import error.
type ImportErrorResponse struct {
	Title string `json:"title"`
	Error string `json:"error"`
}
