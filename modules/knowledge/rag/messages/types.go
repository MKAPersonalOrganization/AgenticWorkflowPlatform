package messages

import "encoding/json"

// UpdateMessageRequest is the request body for updating a message.
type UpdateMessageRequest struct {
	Content        *string          `json:"content,omitempty"`
	TokensUsed     *int             `json:"tokens_used,omitempty"`
	DurationMs     *int             `json:"duration_ms,omitempty"`
	ThinkingTimeMs *int             `json:"thinking_time_ms,omitempty"`
	Context        *json.RawMessage `json:"context,omitempty"`
	IsError        *bool            `json:"is_error,omitempty"`
}

// MessageResponse represents a message returned by the API.
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

// DeleteResponse represents the response from a delete operation.
type DeleteResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}
