package engine

import (
	"school-with-ai/domain"
	"time"
)

// EventFactory creates domain events with consistent agent name and timestamp handling.
type EventFactory struct {
	agentName string
}

// NewEventFactory creates a new EventFactory for the given agent.
func NewEventFactory(agentName string) *EventFactory {
	return &EventFactory{
		agentName: agentName,
	}
}

// AssistantStarted creates an AssistantMessageStartedEvent.
func (f *EventFactory) AssistantStarted(threadID, messageID string) *domain.AssistantMessageStartedEvent {
	return &domain.AssistantMessageStartedEvent{
		ThreadID:  threadID,
		MessageID: messageID,
		Timestamp: time.Now(),
	}
}

// AssistantCompleted creates an AssistantMessageCompletedEvent.
func (f *EventFactory) AssistantCompleted(
	threadID string,
	messageID string,
	content string,
	toolCalls []domain.ToolCall,
) *domain.AssistantMessageCompletedEvent {
	return &domain.AssistantMessageCompletedEvent{
		ThreadID:  threadID,
		MessageID: messageID,
		Content:   content,
		ToolCalls: toolCalls,
		AgentName: f.agentName,
		Timestamp: time.Now(),
	}
}

// ToolCallCompleted creates a ToolCallCompletedEvent.
func (f *EventFactory) ToolCallCompleted(threadID, toolCallID string) *domain.ToolCallCompletedEvent {
	return &domain.ToolCallCompletedEvent{
		ThreadID:   threadID,
		ToolCallID: toolCallID,
		Timestamp:  time.Now(),
	}
}

// ToolCallExecuted creates a ToolCallExecutedEvent.
func (f *EventFactory) ToolCallExecuted(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	toolArguments string,
	toolResult string,
	parentMessageID string,
) *domain.ToolCallExecutedEvent {
	return &domain.ToolCallExecutedEvent{
		ThreadID:        threadID,
		MessageID:       messageID,
		ToolCallID:      toolCallID,
		ToolName:        toolName,
		ToolArguments:   toolArguments,
		ToolResult:      toolResult,
		ParentMessageID: parentMessageID,
		AgentName:       f.agentName,
		Timestamp:       time.Now(),
	}
}
