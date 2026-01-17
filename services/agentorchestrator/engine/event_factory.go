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

// AssistantContentStreamed creates an AssistantContentStreamedEvent.
func (f *EventFactory) AssistantContentStreamed(threadID, messageID, delta string) *domain.AssistantContentStreamedEvent {
	return &domain.AssistantContentStreamedEvent{
		ThreadID:  threadID,
		MessageID: messageID,
		Delta:     delta,
		Timestamp: time.Now(),
	}
}

// ToolCallStarted creates a ToolCallStartedEvent.
func (f *EventFactory) ToolCallStarted(threadID, toolCallID, toolCallName, parentMessageID string) *domain.ToolCallStartedEvent {
	return &domain.ToolCallStartedEvent{
		ThreadID:        threadID,
		ToolCallID:      toolCallID,
		ToolCallName:    toolCallName,
		ParentMessageID: parentMessageID,
		AgentName:       f.agentName,
		Timestamp:       time.Now(),
	}
}

// ToolCallArgumentsStreamed creates a ToolCallArgumentsStreamedEvent.
func (f *EventFactory) ToolCallArgumentsStreamed(threadID, toolCallID, delta string) *domain.ToolCallArgumentsStreamedEvent {
	return &domain.ToolCallArgumentsStreamedEvent{
		ThreadID:   threadID,
		ToolCallID: toolCallID,
		Delta:      delta,
		Timestamp:  time.Now(),
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

// AgentHandoff creates an AgentHandoffEvent.
func (f *EventFactory) AgentHandoff(threadID, targetAgent string) *domain.AgentHandoffEvent {
	return &domain.AgentHandoffEvent{
		ThreadID:    threadID,
		TargetAgent: targetAgent,
		Timestamp:   time.Now(),
	}
}

// RunStarted creates a RunStartedEvent.
func (f *EventFactory) RunStarted(threadID, runID string) *domain.RunStartedEvent {
	return &domain.RunStartedEvent{
		ThreadID:  threadID,
		RunID:     runID,
		Timestamp: time.Now(),
	}
}

// RunFinished creates a RunFinishedEvent.
func (f *EventFactory) RunFinished(threadID, runID string) *domain.RunFinishedEvent {
	return &domain.RunFinishedEvent{
		ThreadID:  threadID,
		RunID:     runID,
		Timestamp: time.Now(),
	}
}

// RunError creates a RunErrorEvent.
func (f *EventFactory) RunError(threadID, message, errorCode string) *domain.RunErrorEvent {
	return &domain.RunErrorEvent{
		ThreadID:  threadID,
		Message:   message,
		ErrorCode: errorCode,
		Timestamp: time.Now(),
	}
}

// HumanInputRequested creates a HumanInputRequestedEvent.
func (f *EventFactory) HumanInputRequested(threadID, messageID, toolCallID, question string, schema map[string]any) *domain.HumanInputRequestedEvent {
	return &domain.HumanInputRequestedEvent{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		Question:   question,
		Schema:     schema,
		Timestamp:  time.Now(),
	}
}
