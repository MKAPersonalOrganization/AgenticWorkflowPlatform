package custom

import (
	"school-with-ai/modules/chat/thread"
	"school-with-ai/foundation/agentsdk/core"
)

// ToDomainEvent translates a core.Event to a thread.Event according to EVENT_CONTRACTS.md.
// This function implements ADR 001 by:
// 1. Only translating events that are permitted in the agent framework events section
// 2. Discarding framework lifecycle events (RunStarted, RunFinished, RunError, etc.)
// 3. Ensuring strict architectural boundary between framework and domain layer
func ToDomainEvent(e core.Event) thread.Event {
	if e == nil {
		return nil
	}

	// Translate only permitted agent framework events per EVENT_CONTRACTS.md
	switch evt := e.(type) {

	// ✅ PERMITTED PERSISTED EVENTS (Agent Framework Events)
	case *core.AssistantMessageAdded:
		return &thread.AssistantMessageAdded{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Content:   evt.Content,
			ToolCalls: MapCoreToolCallsToDomain(evt.ToolCalls),
			AgentName: evt.AgentName,
			Timestamp: evt.Timestamp,
		}

	case *core.ToolResultAdded:
		return &thread.ToolResultAdded{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			ToolName:   evt.ToolName,
			Result:     evt.Result,
			IsError:    evt.IsError,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	case *core.TokenUsageReported:
		return &thread.TokenUsageReported{
			ThreadID:                 evt.ThreadID,
			MessageID:                evt.MessageID,
			Provider:                 evt.Provider,
			Model:                    evt.Model,
			PromptTokens:             evt.PromptTokens,
			CompletionTokens:         evt.CompletionTokens,
			TotalTokens:              evt.TotalTokens,
			CachedTokens:             evt.CachedTokens,
			AudioInputTokens:         evt.AudioInputTokens,
			ReasoningTokens:          evt.ReasoningTokens,
			AudioOutputTokens:        evt.AudioOutputTokens,
			AcceptedPredictionTokens: evt.AcceptedPredictionTokens,
			RejectedPredictionTokens: evt.RejectedPredictionTokens,
			Timestamp:                evt.Timestamp,
		}

	case *core.ToolNeedsInput:
		return &thread.ToolNeedsInput{
			ThreadID:    evt.ThreadID,
			MessageID:   evt.MessageID,
			ToolCallID:  evt.ToolCallID,
			ToolName:    evt.ToolName,
			Elicitation: convertElicitationSpec(evt.Elicitation),
			AgentName:   evt.AgentName,
			Timestamp:   evt.Timestamp,
		}

	case *core.ToolNeedsAuth:
		return &thread.ToolNeedsAuth{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			ToolName:   evt.ToolName,
			AuthSpec:   convertAuthorizationSpec(evt.AuthSpec),
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	case *core.InputProvided:
		return &thread.InputProvided{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			Input:      evt.Input,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	case *core.AuthGranted:
		return &thread.AuthGranted{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	case *core.AuthDenied:
		return &thread.AuthDenied{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	case *core.ToolCancelled:
		return &thread.ToolCancelled{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			ToolName:   evt.ToolName,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	// ✅ PERMITTED TRANSIENT EVENTS (Agent Framework Events)
	case *core.AssistantMessageStarted:
		return &thread.AssistantMessageStarted{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Timestamp: evt.Timestamp,
		}

	case *core.AssistantContentStreamed:
		return &thread.AssistantContentStreamed{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Delta:     evt.Delta,
			Timestamp: evt.Timestamp,
		}

	case *core.AssistantMessageCompleted:
		return &thread.AssistantMessageCompleted{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Content:   evt.Content,
			AgentName: evt.AgentName,
			ToolCalls: MapCoreToolCallsToDomain(evt.ToolCalls),
			Timestamp: evt.Timestamp,
		}

	case *core.ToolStart:
		return &thread.ToolStart{
			ThreadID:      evt.ThreadID,
			MessageID:     evt.MessageID,
			ToolCallID:    evt.ToolCallID,
			ToolName:      evt.ToolName,
			ToolArguments: evt.ToolArguments,
			AgentName:     evt.AgentName,
			Timestamp:     evt.Timestamp,
		}

	// ❌ PROHIBITED EVENTS (Framework Lifecycle - DISCARD per ADR 001)
	// Note: RunStarted, RunFinished, RunError are now domain events emitted at service layer

	case *core.EngineStateChanged:
		// Discard: Framework internal state, not business fact
		return nil

	case *core.ActionsDetected:
		// Discard: Framework coordination event, not business fact
		return nil

	case *core.TurnFinished:
		// Discard: Framework coordination event, not business fact
		return nil

	case *core.ExecutionStateReported:
		// Discard: Framework iteration tracking, not business fact
		return nil

	// ❌ APPLICATION-LEVEL EVENTS (Should never come from agent adapters)
	// These events are emitted by application services, NOT by adapters
	// If the SDK emits these, we discard them to maintain proper boundaries
	default:
		// If it's already a domain event but came from SDK, we need to be careful
		if de, ok := e.(thread.Event); ok {
			eventType := de.EventType()
			switch eventType {
			case "ThreadCreated", "ThreadDeleted", "ThreadTitleUpdated", "UserMessageAdded":
				// These are application-level events - discard if coming from agent adapter
				return nil
			default:
				// Unknown domain event, log warning and discard for safety
				return nil
			}
		}

		// Unknown core event type - discard for safety
		// This ensures we only emit events explicitly defined in the contract
		return nil
	}
}

// MapCoreToolCallsToDomain maps core.ToolCall to thread.ToolCall
func MapCoreToolCallsToDomain(calls []core.ToolCall) []thread.ToolCall {
	if calls == nil {
		return nil
	}
	res := make([]thread.ToolCall, len(calls))
	for i, c := range calls {
		res[i] = thread.ToolCall{
			ID:        c.ID,
			Type:      c.Type,
			Name:      c.Name,
			Arguments: c.Arguments,
		}
	}
	return res
}

// convertElicitationSpec converts core.ElicitationSpec to thread.ElicitationSpec
func convertElicitationSpec(spec *core.ElicitationSpec) *thread.ElicitationSpec {
	if spec == nil {
		return nil
	}
	return &thread.ElicitationSpec{
		Question: spec.Question,
		Schema:   spec.Schema,
	}
}

// convertAuthorizationSpec converts core.AuthorizationSpec to thread.AuthorizationSpec
func convertAuthorizationSpec(spec *core.AuthorizationSpec) *thread.AuthorizationSpec {
	if spec == nil {
		return nil
	}
	return &thread.AuthorizationSpec{
		Resource: spec.Resource,
		Action:   spec.Action,
		Reason:   spec.Reason,
		Scopes:   spec.Scopes,
	}
}
