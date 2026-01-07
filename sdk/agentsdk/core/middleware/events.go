package middleware

import (
	"time"

	"school-with-ai/domain"
)

// WhiteboardUpdatedEvent is emitted when the whiteboard content changes
type WhiteboardUpdatedEvent struct {
	ID        string
	Type      string
	Timestamp time.Time
	ThreadID  string
	AgentName string
	Content   string
	Action    string
}

// Implement Event interface
func (e *WhiteboardUpdatedEvent) EventType() string {
	return "whiteboard.updated"
}

func (e *WhiteboardUpdatedEvent) EventTimestamp() time.Time {
	return e.Timestamp
}

func (e *WhiteboardUpdatedEvent) AggregateID() string {
	return e.ThreadID
}

// NewWhiteboardUpdatedEvent creates a new whiteboard updated event
func NewWhiteboardUpdatedEvent(threadID, agentName, content, action string) *WhiteboardUpdatedEvent {
	return &WhiteboardUpdatedEvent{
		ID:        domain.GenerateEventID(),
		Type:      "whiteboard.updated",
		Timestamp: time.Now(),
		ThreadID:  threadID,
		AgentName: agentName,
		Content:   content,
		Action:    action,
	}
}
