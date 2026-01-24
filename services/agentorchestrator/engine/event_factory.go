package engine

import (
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
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

// TokenUsageReported creates a TokenUsageReported event.
func (f *EventFactory) TokenUsageReported(
	threadID string,
	messageID string,
	provider string,
	model string,
	usage *llm.UsageDetails,
) *domain.TokenUsageReported {
	return &domain.TokenUsageReported{
		ThreadID:                 threadID,
		MessageID:                messageID,
		Provider:                 provider,
		Model:                    model,
		PromptTokens:             usage.InputTokens,
		CompletionTokens:         usage.OutputTokens,
		TotalTokens:              usage.TotalTokens,
		CachedTokens:             usage.CachedTokens,
		AudioInputTokens:         usage.AudioInputTokens,
		ReasoningTokens:          usage.ReasoningTokens,
		AudioOutputTokens:        usage.AudioOutputTokens,
		AcceptedPredictionTokens: usage.AcceptedPredictionTokens,
		RejectedPredictionTokens: usage.RejectedPredictionTokens,
		Timestamp:                time.Now(),
	}
}

// ToolStateChanged creates a ToolStateChangedEvent.
func (f *EventFactory) ToolStateChanged(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	oldState string,
	newState string,
	result *domain.ToolExecutionResult,
) *domain.ToolStateChangedEvent {
	evt := &domain.ToolStateChangedEvent{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		OldState:   oldState,
		NewState:   newState,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}

	if result != nil {
		evt.Result = result.Result
		evt.IsError = result.IsError
		evt.ToolArguments = result.Arguments
	}

	return evt
}

// ToolAwaitingInput creates a ToolAwaitingInputEvent.
func (f *EventFactory) ToolAwaitingInput(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	elicitation *core.ElicitationSpec,
) *domain.ToolAwaitingInputEvent {
	return &domain.ToolAwaitingInputEvent{
		ThreadID:    threadID,
		MessageID:   messageID,
		ToolCallID:  toolCallID,
		ToolName:    toolName,
		Elicitation: elicitation,
		AgentName:   f.agentName,
		Timestamp:   time.Now(),
	}
}

// ToolAwaitingAuth creates a ToolAwaitingAuthEvent.
func (f *EventFactory) ToolAwaitingAuth(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	authSpec *core.AuthorizationSpec,
) *domain.ToolAwaitingAuthEvent {
	return &domain.ToolAwaitingAuthEvent{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		AuthSpec:   authSpec,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}
}

// ToolInputProvided creates a ToolInputProvidedEvent.
func (f *EventFactory) ToolInputProvided(
	threadID string,
	messageID string,
	toolCallID string,
	input string,
) *domain.ToolInputProvidedEvent {
	return &domain.ToolInputProvidedEvent{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		Input:      input,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}
}

// ToolAuthGranted creates a ToolAuthGrantedEvent.
func (f *EventFactory) ToolAuthGranted(
	threadID string,
	messageID string,
	toolCallID string,
) *domain.ToolAuthGrantedEvent {
	return &domain.ToolAuthGrantedEvent{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}
}
