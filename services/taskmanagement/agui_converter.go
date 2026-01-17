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

	case *domain.ToolCallStartedEvent:
		// First emit TOOL_CALL_AGENT to establish agent-tool call association
		agentEvent := aguievents.NewCustomEvent(
			"TOOL_CALL_AGENT",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"agentName":  evt.AgentName,
			}),
		)
		// Then emit standard TOOL_CALL_START event
		startEvent := aguievents.NewToolCallStartEvent(
			evt.ToolCallID,
			evt.ToolCallName,
			aguievents.WithParentMessageID(evt.ParentMessageID),
		)
		return []any{agentEvent, startEvent}

	case *domain.ToolCallArgumentsStreamedEvent:
		return []any{aguievents.NewToolCallArgsEvent(evt.ToolCallID, evt.Delta)}

	case *domain.ToolCallCompletedEvent:
		return []any{aguievents.NewToolCallEndEvent(evt.ToolCallID)}

	case *domain.ToolCallExecutedEvent:
		return []any{aguievents.NewToolCallResultEvent(
			evt.MessageID,
			evt.ToolCallID,
			evt.ToolResult,
		)}

	case *domain.HumanInputRequestedEvent:
		return []any{aguievents.NewCustomEvent(
			"HUMAN_INPUT_REQUESTED",
			aguievents.WithValue(map[string]any{
				"threadId":   evt.ThreadID,
				"messageId":  evt.MessageID,
				"toolCallId": evt.ToolCallID,
				"question":   evt.Question,
				"schema":     evt.Schema,
			}),
		)}

	case *domain.MessageAdded, *domain.ThreadCreated, *domain.AgentHandoffEvent:
		// These are handled by the frontend locally or via snapshots, no need to stream
		return []any{}

	default:
		return nil
	}
}
