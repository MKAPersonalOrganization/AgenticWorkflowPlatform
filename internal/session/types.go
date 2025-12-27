package session

import "time"

// Message represents a single message in a conversation
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user" | "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Thread represents a conversation thread with message history
type Thread struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Messages  []Message `json:"messages"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
