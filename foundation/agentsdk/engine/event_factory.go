package engine

import (
	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/internal/llm"
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

// AssistantStarted creates an AssistantMessageStarted.
func (f *EventFactory) AssistantStarted(threadID, messageID string) *core.AssistantMessageStarted {
	return &core.AssistantMessageStarted{
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
	toolCalls []core.ToolCall,
) *core.AssistantMessageCompleted {
	return &core.AssistantMessageCompleted{
		ThreadID:  threadID,
		MessageID: messageID,
		Content:   content,
		ToolCalls: toolCalls,
		AgentName: f.agentName,
		Timestamp: time.Now(),
	}
}

// AssistantContentStreamed creates an AssistantContentStreamed.
func (f *EventFactory) AssistantContentStreamed(threadID, messageID, delta string) *core.AssistantContentStreamed {
	return &core.AssistantContentStreamed{
		ThreadID:  threadID,
		MessageID: messageID,
		Delta:     delta,
		Timestamp: time.Now(),
	}
}

// TokenUsageReported creates a TokenUsageReported event.
func (f *EventFactory) TokenUsageReported(
	threadID string,
	messageID string,
	provider string,
	model string,
	usage *llm.Usage,
) *core.TokenUsageReported {
	return &core.TokenUsageReported{
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
) *core.ToolStart {
	return &core.ToolStart{
		ThreadID:      threadID,
		MessageID:     messageID,
		ToolCallID:    toolCallID,
		ToolName:      toolName,
		ToolArguments: arguments,
		AgentName:     f.agentName,
		Timestamp:     time.Now(),
	}
}

// ToolContentStreamed creates a ToolContentStreamed event.
func (f *EventFactory) ToolContentStreamed(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	delta string,
	field string,
) *core.ToolContentStreamed {
	return &core.ToolContentStreamed{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Delta:      delta,
		Field:      field,
		Timestamp:  time.Now(),
	}
}

// ToolResultAdded creates a ToolResultAdded event (unified for success/error).
func (f *EventFactory) ToolResultAdded(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	result string,
	isError bool,
	arguments string,
) *core.ToolResultAdded {
	return &core.ToolResultAdded{
		ThreadID:   threadID,
		MessageID:  messageID,
		ToolCallID: toolCallID,
		ToolName:   toolName,
		Result:     result,
		IsError:    isError,
		AgentName:  f.agentName,
		Timestamp:  time.Now(),
	}
}

// ToolNeedsInput creates a ToolNeedsInput event.
func (f *EventFactory) ToolNeedsInput(
	threadID string,
	messageID string,
	toolCallID string,
	toolName string,
	elicitation *core.ElicitationSpec,
) *core.ToolNeedsInput {
	return &core.ToolNeedsInput{
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
) *core.ToolNeedsAuth {
	return &core.ToolNeedsAuth{
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
) *core.InputProvided {
	return &core.InputProvided{
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
) *core.AuthGranted {
	return &core.AuthGranted{
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
) *core.AuthDenied {
	return &core.AuthDenied{
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
) *core.ToolCancelled {
	return &core.ToolCancelled{
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
	toolCalls []core.ToolCall,
) *core.ActionsDetected {
	return &core.ActionsDetected{
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
) *core.TurnFinished {
	return &core.TurnFinished{
		ThreadID:  threadID,
		MessageID: messageID,
		Timestamp: time.Now(),
	}
}

// ExecutionStateReported creates an ExecutionStateReported event.
func (f *EventFactory) ExecutionStateReported(
	threadID string,
	iteration int,
	total int,
	loopStateHex string,
) *core.ExecutionStateReported {
	return &core.ExecutionStateReported{
		ThreadID:     threadID,
		Iteration:    iteration,
		Total:        total,
		LoopStateHex: loopStateHex,
		Timestamp:    time.Now(),
	}
}
