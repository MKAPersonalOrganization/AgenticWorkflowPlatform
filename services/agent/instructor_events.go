package agent

import "time"

// AgentCalledEvent is emitted when any agent calls another agent (unified event-driven approach)
type AgentCalledEvent struct {
	BaseEvent
	CallerAgent     string
	CalledAgent     string
	Reason          string
	Context         string
	WhiteboardState string
}

func NewAgentCalled(threadID, caller, called, reason, context, whiteboardState string) *AgentCalledEvent {
	return &AgentCalledEvent{
		BaseEvent: BaseEvent{
			Type:      "AgentCalled",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		CallerAgent:     caller,
		CalledAgent:     called,
		Reason:          reason,
		Context:         context,
		WhiteboardState: whiteboardState,
	}
}

type WhiteboardUpdatedEvent struct {
	BaseEvent
	AgentName string
	Content   string
	Action    string // "write" | "append" | "clear"
}

func NewWhiteboardUpdated(threadID, agentName, content, action string) *WhiteboardUpdatedEvent {
	return &WhiteboardUpdatedEvent{
		BaseEvent: BaseEvent{
			Type:      "WhiteboardUpdated",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		AgentName: agentName,
		Content:   content,
		Action:    action,
	}
}

// Private instructor events

type InstructorThoughtRecordedEvent struct {
	BaseEvent
	InstructorName string
	Thought        string
	IsPrivate      bool
}

func NewInstructorThoughtRecorded(threadID, instructorName, thought string, isPrivate bool) *InstructorThoughtRecordedEvent {
	return &InstructorThoughtRecordedEvent{
		BaseEvent: BaseEvent{
			Type:      "InstructorThoughtRecorded",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		InstructorName: instructorName,
		Thought:        thought,
		IsPrivate:      isPrivate,
	}
}

type InstructorStrategyChangedEvent struct {
	BaseEvent
	InstructorName string
	OldStrategy    string
	NewStrategy    string
	Reason         string
}

func NewInstructorStrategyChanged(threadID, instructorName, oldStrategy, newStrategy, reason string) *InstructorStrategyChangedEvent {
	return &InstructorStrategyChangedEvent{
		BaseEvent: BaseEvent{
			Type:      "InstructorStrategyChanged",
			Timestamp: time.Now(),
			ThreadID:  threadID,
		},
		InstructorName: instructorName,
		OldStrategy:    oldStrategy,
		NewStrategy:    newStrategy,
		Reason:         reason,
	}
}
