package shared

import "encoding/json"

// ConversationResponse represents a conversation returned by the API.
type ConversationResponse struct {
ID                 string  `json:"id"`
TenantID           string  `json:"tenant_id"`
WorkspaceID        *string `json:"workspace_id,omitempty"`
Title              string  `json:"title"`
Mode               string  `json:"mode"`
IsPinned           bool    `json:"is_pinned"`
IsArchived         bool    `json:"is_archived"`
FolderID           *string `json:"folder_id,omitempty"`
ShareID            *string `json:"share_id,omitempty"`
MessageCount       *int    `json:"message_count,omitempty"`
LastMessagePreview *string `json:"last_message_preview,omitempty"`
CreatedAt          string  `json:"created_at"`
UpdatedAt          string  `json:"updated_at"`
}

// MessageResponse represents a message within a shared conversation.
type MessageResponse struct {
ID             string          `json:"id"`
ConversationID string          `json:"conversation_id"`
ParentID       *string         `json:"parent_id,omitempty"`
Role           string          `json:"role"`
Content        string          `json:"content"`
Mode           *string         `json:"mode,omitempty"`
TokensUsed     *int            `json:"tokens_used,omitempty"`
DurationMs     *int            `json:"duration_ms,omitempty"`
ThinkingTimeMs *int            `json:"thinking_time_ms,omitempty"`
Context        json.RawMessage `json:"context,omitempty"`
IsError        bool            `json:"is_error"`
CreatedAt      string          `json:"created_at"`
UpdatedAt      string          `json:"updated_at"`
}

// SharedConversationResponse represents a shared conversation with its messages.
type SharedConversationResponse struct {
Conversation ConversationResponse `json:"conversation"`
Messages     []MessageResponse    `json:"messages"`
}
