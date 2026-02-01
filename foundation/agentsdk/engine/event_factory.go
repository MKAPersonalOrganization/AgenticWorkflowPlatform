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

// AssistantStarted creates an AssistantMessageStarted.
func (f *EventFactory) AssistantStarted(threadID, messageID string) *domain.AssistantMessageStarted {
	return &domain.AssistantMessageStarted{
		ThreadID:  threadID,
		MessageID: messageID,
		Timestamp: time.Now(),
	}
}

// AssistantCompleted creates an AssistantMessageCompleted.
func (f *EventFactory) AssistantCompleted(
	threadID string,
	messageID string,
	content string,
	toolCalls []domain.ToolCall,
) *domain.AssistantMessageCompleted {
	return &domain.AssistantMessageCompleted{
		ThreadID:  threadID,
		MessageID: messageID,
		Content:   content,
		ToolCalls: toolCalls,
		AgentName: f.agentName,
		Timestamp: time.Now(),
	}
}

// AssistantContentStreamed creates an AssistantContentStreamed.
func (f *EventFactory) AssistantContentStreamed(threadID, messageID, delta string) *domain.AssistantContentStreamed {
	return &domain.AssistantContentStreamed{
		ThreadID:  threadID,
		MessageID: messageID,
		Delta:     delta,
		Timestamp: time.Now(),
	}
}

// RunStarted creates a RunStarted.
func (f *EventFactory) RunStarted(threadID, runID string) *domain.RunStarted {
	return &domain.RunStarted{
		ThreadID:  threadID,
		RunID:     runID,
		Timestamp: time.Now(),
	}
}

// RunFinished creates a RunFinished.
func (f *EventFactory) RunFinished(threadID, runID string) *domain.RunFinished {
	return &domain.RunFinished{
		ThreadID:  threadID,
		RunID:     runID,
		Timestamp: time.Now(),
	}
}

// RunError creates a RunError.
func (f *EventFactory) RunError(threadID, message, errorCode string) *domain.RunError {
	return &domain.RunError{
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

// ToolStart creates a ToolStart event.
func (f *EventFactory) ToolStart(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	arguments string,
) *domain.ToolStart {
	return &domain.ToolStart{
		ThreadID:      threadID,
		MessageID:     messageID,
		ToolCallID:    toolCallID,
		ToolName:      toolName,
		ToolArguments: arguments,
		AgentName:     f.agentName,
		Timestamp:     time.Now(),
	}
}

// ToolSuccess creates a ToolSuccess event.
func (f *EventFactory) ToolSuccess(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	result string,
	arguments string,
) *domain.ToolSuccess {
	return &domain.ToolSuccess{
		ThreadID:      threadID,
		MessageID:     messageID,
		ToolCallID:    toolCallID,
		ToolName:      toolName,
		Result:        result,
		ToolArguments: arguments,
		AgentName:     f.agentName,
		Timestamp:     time.Now(),
	}
}

// ToolError creates a ToolError event.
func (f *EventFactory) ToolError(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	errorMsg string,
	arguments string,
) *domain.ToolError {
	return &domain.ToolError{
		ThreadID:      threadID,
		MessageID:     messageID,
		ToolCallID:    toolCallID,
		ToolName:      toolName,
		Error:         errorMsg,
		ToolArguments: arguments,
		AgentName:     f.agentName,
		Timestamp:     time.Now(),
	}
}

// ToolNeedsInput creates a ToolNeedsInput event.
func (f *EventFactory) ToolNeedsInput(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	elicitation *core.ElicitationSpec,
) *domain.ToolNeedsInput {
	return &domain.ToolNeedsInput{
		ThreadID:    threadID,
		MessageID:   messageID,
		ToolCallID:  toolCallID,
		ToolName:    toolName,
		Elicitation: elicitation,
		AgentName:   f.agentName,
		Timestamp:   time.Now(),
	}
}

// ToolNeedsAuth creates a ToolNeedsAuth event.
func (f *EventFactory) ToolNeedsAuth(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	authSpec *core.AuthorizationSpec,
) *domain.ToolNeedsAuth {
	return &domain.ToolNeedsAuth{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		AuthSpec:   authSpec,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}
}

// InputProvided creates an InputProvided event.
func (f *EventFactory) InputProvided(
	threadID string,
	messageID string,
	toolCallID string,
	input string,
) *domain.InputProvided {
	return &domain.InputProvided{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		Input:      input,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}
}

// AuthGranted creates an AuthGranted event.
func (f *EventFactory) AuthGranted(
	threadID string,
	messageID string,
	toolCallID string,
) *domain.AuthGranted {
	return &domain.AuthGranted{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}
}

// AuthDenied creates an AuthDenied event.
func (f *EventFactory) AuthDenied(
	threadID string,
	messageID string,
	toolCallID string,
) *domain.AuthDenied {
	return &domain.AuthDenied{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}
}

// ToolCancelled creates a ToolCancelled event.
func (f *EventFactory) ToolCancelled(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
) *domain.ToolCancelled {
	return &domain.ToolCancelled{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}
}

// ActionsDetected creates an ActionsDetected event.
func (f *EventFactory) ActionsDetected(
	threadID string,
	messageID string,
	toolCalls []domain.ToolCall,
) *domain.ActionsDetected {
	return &domain.ActionsDetected{
		ThreadID:  threadID,
		MessageID: messageID,
		ToolCalls: toolCalls,
		Timestamp: time.Now(),
	}
}

// TurnFinished creates a TurnFinished event.
func (f *EventFactory) TurnFinished(
	threadID string,
	messageID string,
) *domain.TurnFinished {
	return &domain.TurnFinished{
		ThreadID:  threadID,
		MessageID: messageID,
		Timestamp: time.Now(),
	}
}
