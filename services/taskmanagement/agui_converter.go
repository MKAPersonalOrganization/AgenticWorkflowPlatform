package taskmanagement

import (
	"school-with-ai/domain"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ToAguiEvents converts domain events to AGUI events.
// It returns a slice of events because some domain events map to multiple AGUI events.
func ToAguiEvents(event domain.Event) []any {
	if event == nil {
		return nil
	}

	return toAguiEventsInternal(event)
}

func toAguiEventsInternal(event any) []any {
	switch evt := event.(type) {
	case *domain.RunStarted:
		return []any{aguievents.NewRunStartedEvent(evt.ThreadID, evt.RunID)}

	case *domain.RunFinished:
		return []any{aguievents.NewRunFinishedEvent(evt.ThreadID, evt.RunID)}

	case *domain.RunError:
		return []any{aguievents.NewRunErrorEvent(evt.Message, aguievents.WithErrorCode(evt.Code))}

	case *domain.ThreadTitleUpdated:
		return []any{aguievents.NewCustomEvent(
			"THREAD_TITLE_UPDATED",
			aguievents.WithValue(map[string]any{
				"threadId": evt.ThreadID,
				"title":    evt.Title,
			}),
		)}

	case *domain.AssistantMessageStarted:
		return []any{aguievents.NewTextMessageStartEvent(evt.MessageID, aguievents.WithRole("assistant"))}

	case *domain.AssistantContentStreamed:
		return []any{aguievents.NewTextMessageContentEvent(evt.MessageID, evt.Delta)}

	case *domain.AssistantMessageCompleted:
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

	case *domain.ToolStart:
		return []any{aguievents.NewCustomEvent(
			"TOOL_START",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
				"arguments":  evt.ToolArguments,
				"agentName":  evt.AgentName,
				"timestamp":  evt.Timestamp,
			}),
		)}

	case *domain.ToolContentStreamed:
		return []any{aguievents.NewCustomEvent(
			"TOOL_CONTENT_STREAMED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
				"delta":      evt.Delta,
				"field":      evt.Field,
				"timestamp":  evt.Timestamp,
			}),
		)}

	case *domain.ToolResultAdded:
		eventName := "TOOL_RESULT_ADDED"
		// Backward compatibility: emit legacy event names based on IsError flag
		if evt.IsError {
			eventName = "TOOL_ERROR"
		} else {
			eventName = "TOOL_SUCCESS"
		}

		return []any{aguievents.NewCustomEvent(
			eventName,
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
				"result":     evt.Result,
				"isError":    evt.IsError,
				"agentName":  evt.AgentName,
				"timestamp":  evt.Timestamp,
			}),
		)}

	case *domain.ToolNeedsInput:
		return []any{aguievents.NewCustomEvent(
			"TOOL_NEEDS_INPUT",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
				"question":   evt.Elicitation.Question,
				"schema":     evt.Elicitation.Schema,
				"agentName":  evt.AgentName,
			}),
		)}

	case *domain.ToolNeedsAuth:
		return []any{aguievents.NewCustomEvent(
			"TOOL_NEEDS_AUTH",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
				"authSpec":   evt.AuthSpec,
				"agentName":  evt.AgentName,
			}),
		)}

	case *domain.InputProvided:
		return []any{aguievents.NewCustomEvent(
			"INPUT_PROVIDED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"messageId":  evt.MessageID,
				"input":      evt.Input,
			}),
		)}

	case *domain.AuthGranted:
		return []any{aguievents.NewCustomEvent(
			"AUTH_GRANTED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
			}),
		)}

	case *domain.AuthDenied:
		return []any{aguievents.NewCustomEvent(
			"AUTH_DENIED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
			}),
		)}

	case *domain.ToolCancelled:
		return []any{aguievents.NewCustomEvent(
			"TOOL_CANCELLED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
			}),
		)}

	case *domain.UserMessageAdded, *domain.AssistantMessageAdded, *domain.ThreadCreated:
		// These are handled by the frontend locally or via snapshots, no need to stream
		return []any{}

	default:
		return nil
	}
}
