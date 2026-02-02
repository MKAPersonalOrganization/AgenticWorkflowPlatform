package taskmanagement

import (
	"school-with-ai/domain"
	"school-with-ai/foundation/agent"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ToAguiEvents converts agent events to AGUI events.
// It returns a slice of events because some agent events map to multiple AGUI events.
func ToAguiEvents(event agent.Event) []any {
	if event == nil {
		return nil
	}

	switch event.Type() {
	case agent.EventTypeMessage:
		data, ok := event.Data().(*agent.MessageEvent)
		if !ok || data.Message == nil {
			return nil
		}
		msg := data.Message
		events := []any{}
		switch msg.Role {
		case agent.RoleAssistant:
			events = append(events, aguievents.NewTextMessageStartEvent(msg.ID, aguievents.WithRole("assistant")))
			// Extract text content
			for _, c := range msg.Content {
				if c.Type == agent.ContentTypeText {
					events = append(events, aguievents.NewTextMessageContentEvent(msg.ID, c.Text))
				}
			}
			events = append(events, aguievents.NewTextMessageEndEvent(msg.ID))
		case agent.RoleUser:
			// Usually handled locally, but can be added if needed
		}
		return events

	case agent.EventTypeToolCall:
		data, ok := event.Data().(*agent.ToolCallEvent)
		if !ok || data.ToolCall == nil {
			return nil
		}
		tc := data.ToolCall
		return []any{aguievents.NewCustomEvent(
			"TOOL_START",
			aguievents.WithValue(map[string]any{
				"toolCallId": tc.ID,
				"toolName":   tc.Name,
				"arguments":  tc.Arguments,
				"timestamp":  event.Timestamp(),
			}),
		)}

	case agent.EventTypeToolResult:
		data, ok := event.Data().(*agent.ToolResultEvent)
		if !ok || data.Result == nil {
			return nil
		}
		tr := data.Result
		if tr.IsError {
			return []any{aguievents.NewCustomEvent(
				"TOOL_ERROR",
				aguievents.WithValue(map[string]any{
					"toolCallId": tr.ToolCallID,
					"error":      tr.Output,
					"timestamp":  event.Timestamp(),
				}),
			)}
		}
		return []any{aguievents.NewCustomEvent(
			"TOOL_SUCCESS",
			aguievents.WithValue(map[string]any{
				"toolCallId": tr.ToolCallID,
				"result":     tr.Output,
				"timestamp":  event.Timestamp(),
			}),
		)}

	case agent.EventTypeToolInteraction:
		data, ok := event.Data().(*agent.ToolInteractionEvent)
		if !ok {
			return nil
		}
		if data.Type == "input" {
			return []any{aguievents.NewCustomEvent(
				"TOOL_NEEDS_INPUT",
				aguievents.WithValue(map[string]any{
					"toolCallId": data.ToolCallID,
					"toolName":   data.ToolName,
					"question":   data.Prompt,
					"schema":     data.Schema,
				}),
			)}
		} else if data.Type == "auth" {
			return []any{aguievents.NewCustomEvent(
				"TOOL_NEEDS_AUTH",
				aguievents.WithValue(map[string]any{
					"toolCallId": data.ToolCallID,
					"toolName":   data.ToolName,
					"authSpec":   data.AuthSpec,
				}),
			)}
		}

	case agent.EventTypeUsage:
		data, ok := event.Data().(*agent.UsageEvent)
		if !ok || data.Usage == nil {
			return nil
		}
		u := data.Usage
		return []any{aguievents.NewCustomEvent(
			"TOKEN_USAGE_REPORTED",
			aguievents.WithValue(map[string]any{
				"prompt":     u.PromptTokens,
				"completion": u.CompletionTokens,
				"total":      u.TotalTokens,
			}),
		)}

	case agent.EventTypeError:
		data, ok := event.Data().(*agent.ErrorEvent)
		if !ok {
			return nil
		}
		return []any{aguievents.NewRunErrorEvent(
			data.Error,
			aguievents.WithErrorCode(data.Code),
		)}

	default:
		// Fallback for domain events: unwrap the data payload
		if event.Data() != nil {
			return toAguiEventsLegacy(event.Data())
		}
		return toAguiEventsLegacy(event)
	}
	return nil
}

func toAguiEventsLegacy(event any) []any {
	switch evt := event.(type) {
	case *domain.RunStarted:
		return []any{aguievents.NewRunStartedEvent(evt.ThreadID, evt.RunID)}

	case *domain.RunFinished:
		return []any{aguievents.NewRunFinishedEvent(evt.ThreadID, evt.RunID)}

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
