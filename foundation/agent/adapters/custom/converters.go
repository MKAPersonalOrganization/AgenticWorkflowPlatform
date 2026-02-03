package custom

import (
	"school-with-ai/domain"
	"school-with-ai/foundation/agentsdk/core"
)

// ToDomainEvent translates a core.Event to a domain.Event according to EVENT_CONTRACTS.md.
// This function implements ADR 001 by:
// 1. Only translating events that are permitted in the agent framework events section
// 2. Discarding framework lifecycle events (RunStarted, RunFinished, RunError, etc.)
// 3. Ensuring strict architectural boundary between framework and domain layer
func ToDomainEvent(e core.Event) domain.Event {
	if e == nil {
		return nil
	}

	// Translate only permitted agent framework events per EVENT_CONTRACTS.md
	switch evt := e.(type) {

	// ✅ PERMITTED PERSISTED EVENTS (Agent Framework Events)
	case *core.AssistantMessageAdded:
		return &domain.AssistantMessageAdded{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Content:   evt.Content,
			ToolCalls: MapCoreToolCallsToDomain(evt.ToolCalls),
			AgentName: evt.AgentName,
			Timestamp: evt.Timestamp,
		}

	case *core.ToolResultAdded:
		return &domain.ToolResultAdded{
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
		return &domain.TokenUsageReported{
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
		return &domain.ToolNeedsInput{
			ThreadID:    evt.ThreadID,
			MessageID:   evt.MessageID,
			ToolCallID:  evt.ToolCallID,
			ToolName:    evt.ToolName,
			Elicitation: convertElicitationSpec(evt.Elicitation),
			AgentName:   evt.AgentName,
			Timestamp:   evt.Timestamp,
		}

	case *core.ToolNeedsAuth:
		return &domain.ToolNeedsAuth{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			ToolName:   evt.ToolName,
			AuthSpec:   convertAuthorizationSpec(evt.AuthSpec),
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	case *core.InputProvided:
		return &domain.InputProvided{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			Input:      evt.Input,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	case *core.AuthGranted:
		return &domain.AuthGranted{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	case *core.AuthDenied:
		return &domain.AuthDenied{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	case *core.ToolCancelled:
		return &domain.ToolCancelled{
			ThreadID:   evt.ThreadID,
			MessageID:  evt.MessageID,
			ToolCallID: evt.ToolCallID,
			ToolName:   evt.ToolName,
			AgentName:  evt.AgentName,
			Timestamp:  evt.Timestamp,
		}

	// ✅ PERMITTED TRANSIENT EVENTS (Agent Framework Events)
	case *core.AssistantMessageStarted:
		return &domain.AssistantMessageStarted{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Timestamp: evt.Timestamp,
		}

	case *core.AssistantContentStreamed:
		return &domain.AssistantContentStreamed{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Delta:     evt.Delta,
			Timestamp: evt.Timestamp,
		}

	case *core.AssistantMessageCompleted:
		return &domain.AssistantMessageCompleted{
			ThreadID:  evt.ThreadID,
			MessageID: evt.MessageID,
			Content:   evt.Content,
			AgentName: evt.AgentName,
			ToolCalls: MapCoreToolCallsToDomain(evt.ToolCalls),
			Timestamp: evt.Timestamp,
		}

	case *core.ToolStart:
		return &domain.ToolStart{
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
		if de, ok := e.(domain.Event); ok {
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

// MapCoreToolCallsToDomain maps core.ToolCall to domain.ToolCall
func MapCoreToolCallsToDomain(calls []core.ToolCall) []domain.ToolCall {
	if calls == nil {
		return nil
	}
	res := make([]domain.ToolCall, len(calls))
	for i, c := range calls {
		res[i] = domain.ToolCall{
			ID:        c.ID,
			Type:      c.Type,
			Name:      c.Name,
			Arguments: c.Arguments,
		}
	}
	return res
}

// convertElicitationSpec converts core.ElicitationSpec to domain.ElicitationSpec
func convertElicitationSpec(spec *core.ElicitationSpec) *domain.ElicitationSpec {
	if spec == nil {
		return nil
	}
	return &domain.ElicitationSpec{
		Question: spec.Question,
		Schema:   spec.Schema,
	}
}

// convertAuthorizationSpec converts core.AuthorizationSpec to domain.AuthorizationSpec
func convertAuthorizationSpec(spec *core.AuthorizationSpec) *domain.AuthorizationSpec {
	if spec == nil {
		return nil
	}
	return &domain.AuthorizationSpec{
		Resource: spec.Resource,
		Action:   spec.Action,
		Reason:   spec.Reason,
		Scopes:   spec.Scopes,
	}
}
