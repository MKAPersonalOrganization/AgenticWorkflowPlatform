package common

import (
	"aicompass/modules/chat/thread"

	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
)

// ToAguiEvents converts domain events to AGUI events.
// It returns a slice of events because some domain events map to multiple AGUI events.
func ToAguiEvents(event thread.Event) []any {
	if event == nil {
		return nil
	}

	return toAguiEventsInternal(event)
}

func toAguiEventsInternal(event any) []any {
	switch evt := event.(type) {
	case *thread.RunStarted:
		return []any{aguievents.NewRunStartedEvent(evt.ThreadID, evt.RunID)}

	case *thread.RunFinished:
		return []any{aguievents.NewRunFinishedEvent(evt.ThreadID, evt.RunID)}

	case *thread.RunError:
		return []any{aguievents.NewRunErrorEvent(evt.Message, aguievents.WithErrorCode(evt.Code))}

	case *thread.ThreadTitleUpdated:
		return []any{aguievents.NewCustomEvent(
			"THREAD_TITLE_UPDATED",
			aguievents.WithValue(map[string]any{
				"threadId": evt.ThreadID,
				"title":    evt.Title,
			}),
		)}

	case *thread.AssistantMessageStarted:
		return []any{aguievents.NewTextMessageStartEvent(evt.MessageID, aguievents.WithRole("assistant"))}

	case *thread.AssistantContentStreamed:
		return []any{aguievents.NewTextMessageContentEvent(evt.MessageID, evt.Delta)}

	case *thread.AssistantMessageCompleted:
		return []any{aguievents.NewTextMessageEndEvent(evt.MessageID)}

	case *thread.TokenUsageReported:
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

	case *thread.ToolStart:
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

	case *thread.ToolResultAdded:
		// Emit event names based on IsError flag
		var eventName string
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

	case *thread.ToolNeedsConfirmation:
		return []any{aguievents.NewCustomEvent(
			"TOOL_NEEDS_CONFIRMATION",
			aguievents.WithValue(map[string]any{
				"toolCallId":         evt.ToolCallID,
				"originalToolCallId": evt.OriginalToolCallID,
				"toolName":           evt.ToolName,
				"question":           evt.Question,
				"agentName":          evt.AgentName,
			}),
		)}

	case *thread.ToolNeedsInput:
		return []any{aguievents.NewCustomEvent(
			"TOOL_NEEDS_INPUT",
			aguievents.WithValue(map[string]any{
				"toolCallId":         evt.ToolCallID,
				"originalToolCallId": evt.OriginalToolCallID,
				"toolName":           evt.ToolName,
				"question":           evt.Elicitation.Question,
				"schema":             evt.Elicitation.Schema,
				"agentName":          evt.AgentName,
			}),
		)}

	case *thread.ToolNeedsAuth:
		return []any{aguievents.NewCustomEvent(
			"TOOL_NEEDS_AUTH",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
				"authSpec":   evt.AuthSpec,
				"agentName":  evt.AgentName,
			}),
		)}

	case *thread.InputProvided:
		return []any{aguievents.NewCustomEvent(
			"INPUT_PROVIDED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"messageId":  evt.MessageID,
				"input":      evt.Input,
			}),
		)}

	case *thread.AuthGranted:
		return []any{aguievents.NewCustomEvent(
			"AUTH_GRANTED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
			}),
		)}

	case *thread.AuthDenied:
		return []any{aguievents.NewCustomEvent(
			"AUTH_DENIED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
			}),
		)}

	case *thread.ToolCancelled:
		return []any{aguievents.NewCustomEvent(
			"TOOL_CANCELLED",
			aguievents.WithValue(map[string]any{
				"toolCallId": evt.ToolCallID,
				"toolName":   evt.ToolName,
			}),
		)}

	case *thread.AgentTransferStarted:
		return []any{aguievents.NewCustomEvent(
			"AGENT_TRANSFER_STARTED",
			aguievents.WithValue(map[string]any{
				"threadId":      evt.ThreadID,
				"fromAgent":     evt.FromAgent,
				"toAgent":       evt.ToAgent,
				"workflowType":  evt.WorkflowType,
				"parentAgent":   evt.ParentAgent,
				"stepIndex":     evt.StepIndex,
				"stepCount":     evt.StepCount,
				"iteration":     evt.Iteration,
				"maxIterations": evt.MaxIterations,
				"timestamp":     evt.Timestamp,
			}),
		)}

	case *thread.A2AArtifactReceived:
		// Emit a generic ARTIFACT custom event — the frontend treats this as a
		// UI affordance (document card) with no knowledge of A2A internals.
		id := evt.ArtifactID
		if id == "" {
			id = evt.MessageID
		}
		return []any{aguievents.NewCustomEvent(
			"ARTIFACT",
			aguievents.WithValue(map[string]any{
				"id":        id,
				"title":     evt.Title,
				"content":   evt.Content,
				"mimeType":  "text/markdown",
				"agentName": evt.AgentName,
			}),
		)}

	case *thread.FileAttachmentReceived:
		return []any{aguievents.NewCustomEvent(
			"FILE_ATTACHMENT",
			aguievents.WithValue(map[string]any{
				"fileUrl":   evt.FileURL,
				"fileName":  evt.FileName,
				"mimeType":  evt.MimeType,
				"agentName": evt.AgentName,
			}),
		)}

	case *thread.WorkflowNodeCompleted:
		return []any{aguievents.NewCustomEvent(
			"WORKFLOW_NODE_COMPLETED",
			aguievents.WithValue(map[string]any{
				"agentName":  evt.AgentName,
				"durationMs": evt.DurationMs,
				"outcome":    evt.Outcome,
				"timestamp":  evt.Timestamp,
			}),
		)}

	case *thread.StateChangeReported:
		return []any{aguievents.NewCustomEvent(
			"STATE_CHANGE_REPORTED",
			aguievents.WithValue(map[string]any{
				"agentName": evt.AgentName,
				"key":       evt.Key,
				"value":     evt.Value,
				"timestamp": evt.Timestamp,
			}),
		)}

	case *thread.ArtifactVersionSaved:
		return []any{aguievents.NewCustomEvent(
			"ARTIFACT_VERSION_SAVED",
			aguievents.WithValue(map[string]any{
				"agentName": evt.AgentName,
				"fileName":  evt.FileName,
				"version":   evt.Version,
				"timestamp": evt.Timestamp,
			}),
		)}

	case *thread.CodeExecutionRan:
		// Surfaces Gemini-executed (and OpenAI code-interpreter)
		// snippets to the chat stream. Frontend renders a card with
		// the source in a highlighted block and the sandbox output
		// below. BlockID uniquely identifies this block so the live
		// stream and the persisted-history replay can reconcile.
		return []any{aguievents.NewCustomEvent(
			"CODE_EXECUTION_RAN",
			aguievents.WithValue(map[string]any{
				"messageId": evt.MessageID,
				"blockId":   evt.BlockID,
				"language":  evt.Language,
				"code":      evt.Code,
				"output":    evt.Output,
				"outcome":   evt.Outcome,
				"agentName": evt.AgentName,
				"timestamp": evt.Timestamp,
			}),
		)}

	case *thread.UserMessageAdded, *thread.AssistantMessageAdded, *thread.ThreadCreated:
		// These are handled by the frontend locally or via snapshots, no need to stream
		return []any{}

	default:
		return nil
	}
}
