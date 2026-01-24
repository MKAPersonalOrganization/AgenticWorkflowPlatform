package taskmanagement

import (
	"school-with-ai/domain"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ToAguiEvents converts domain events to AGUI events.
// It returns a slice of events because some domain events map to multiple AGUI events.
func ToAguiEvents(event any) []any {
	switch evt := event.(type) {
	case *domain.RunStartedEvent:
		return []any{aguievents.NewRunStartedEvent(evt.ThreadID, evt.RunID)}

	case *domain.RunFinishedEvent:
		return []any{aguievents.NewRunFinishedEvent(evt.ThreadID, evt.RunID)}

	case *domain.RunErrorEvent:
		return []any{aguievents.NewRunErrorEvent(
			evt.Message,
			aguievents.WithErrorCode(evt.ErrorCode),
		)}

	case *domain.ThreadTitleUpdatedEvent:
		return []any{aguievents.NewCustomEvent(
			"THREAD_TITLE_UPDATED",
			aguievents.WithValue(map[string]any{
				"threadId": evt.ThreadID,
				"title":    evt.Title,
			}),
		)}

	case *domain.AssistantMessageStartedEvent:
		return []any{aguievents.NewTextMessageStartEvent(evt.MessageID, aguievents.WithRole("assistant"))}

	case *domain.AssistantContentStreamedEvent:
		return []any{aguievents.NewTextMessageContentEvent(evt.MessageID, evt.Delta)}

	case *domain.AssistantMessageCompletedEvent:
		return []any{aguievents.NewTextMessageEndEvent(evt.MessageID)}

	case *domain.TokenUsageReported:
		return []any{aguievents.NewCustomEvent(
			"TOKEN_USAGE_REPORTED",
			aguievents.WithValue(map[string]any{
				"threadId":        evt.ThreadID,
				"messageId":       evt.MessageID,
				"provider":        evt.Provider,
				"model":           evt.Model,
				"prompt":          evt.PromptTokens,
				"completion":      evt.CompletionTokens,
				"total":           evt.TotalTokens,
				"cached":          evt.CachedTokens,
				"reasoning":       evt.ReasoningTokens,
				"audioInput":      evt.AudioInputTokens,
				"audioOutput":     evt.AudioOutputTokens,
				"acceptedPredict": evt.AcceptedPredictionTokens,
				"rejectedPredict": evt.RejectedPredictionTokens,
			}),
		)}

	case *domain.ToolStateChangedEvent:
		return []any{aguievents.NewCustomEvent(
			"TOOL_STATE_CHANGED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
				"oldState":   evt.OldState,
				"newState":   evt.NewState,
				"result":     evt.Result,
				"isError":    evt.IsError,
				"arguments":  evt.ToolArguments,
				"agentName":  evt.AgentName,
				"timestamp":  evt.Timestamp,
			}),
		)}

	case *domain.ToolAwaitingInputEvent:
		return []any{aguievents.NewCustomEvent(
			"TOOL_AWAITING_INPUT",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
				"question":   evt.Elicitation.Question,
				"schema":     evt.Elicitation.Schema,
				"agentName":  evt.AgentName,
			}),
		)}

	case *domain.ToolAwaitingAuthEvent:
		return []any{aguievents.NewCustomEvent(
			"TOOL_AWAITING_AUTH",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
				"authSpec":   evt.AuthSpec,
				"agentName":  evt.AgentName,
			}),
		)}

	case *domain.ToolInputProvidedEvent:
		return []any{aguievents.NewCustomEvent(
			"TOOL_INPUT_PROVIDED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"messageId":  evt.MessageID,
				"input":      evt.Input,
			}),
		)}

	case *domain.ToolAuthGrantedEvent:
		return []any{aguievents.NewCustomEvent(
			"TOOL_AUTH_GRANTED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
			}),
		)}

	case *domain.MessageAdded, *domain.ThreadCreated, *domain.AgentHandoffEvent:
		// These are handled by the frontend locally or via snapshots, no need to stream
		return []any{}

	default:
		return nil
	}
}
