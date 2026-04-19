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
		return singleton(aguievents.NewRunStartedEvent(evt.ThreadID, evt.RunID))
	case *thread.RunFinished:
		return singleton(aguievents.NewRunFinishedEvent(evt.ThreadID, evt.RunID))
	case *thread.RunError:
		return singleton(aguievents.NewRunErrorEvent(evt.Message, aguievents.WithErrorCode(evt.Code)))
	case *thread.AssistantMessageStarted:
		return singleton(aguievents.NewTextMessageStartEvent(evt.MessageID, aguievents.WithRole("assistant")))
	case *thread.AssistantContentStreamed:
		return singleton(aguievents.NewTextMessageContentEvent(evt.MessageID, evt.Delta))
	case *thread.AssistantMessageCompleted:
		return singleton(aguievents.NewTextMessageEndEvent(evt.MessageID))
	case *thread.ThreadTitleUpdated:
		return customEvent("THREAD_TITLE_UPDATED", map[string]any{
			"threadId": evt.ThreadID,
			"title":    evt.Title,
		})
	case *thread.TokenUsageReported:
		return customEvent("TOKEN_USAGE_REPORTED", tokenUsagePayload(evt))
	case *thread.ToolStart:
		return customEvent("TOOL_START", map[string]any{
			"toolCallId": evt.ToolCallID,
			"toolName":   evt.ToolName,
			"arguments":  evt.ToolArguments,
			"agentName":  evt.AgentName,
			"timestamp":  evt.Timestamp,
		})
	case *thread.ToolResultAdded:
		return toolResultEvent(evt)
	case *thread.ToolNeedsConfirmation:
		return customEvent("TOOL_NEEDS_CONFIRMATION", map[string]any{
			"toolCallId":         evt.ToolCallID,
			"originalToolCallId": evt.OriginalToolCallID,
			"toolName":           evt.ToolName,
			"question":           evt.Question,
			"agentName":          evt.AgentName,
		})
	case *thread.ToolNeedsInput:
		return customEvent("TOOL_NEEDS_INPUT", map[string]any{
			"toolCallId":         evt.ToolCallID,
			"originalToolCallId": evt.OriginalToolCallID,
			"toolName":           evt.ToolName,
			"question":           evt.Elicitation.Question,
			"schema":             evt.Elicitation.Schema,
			"agentName":          evt.AgentName,
		})
	case *thread.ToolNeedsAuth:
		return customEvent("TOOL_NEEDS_AUTH", map[string]any{
			"toolCallId": evt.ToolCallID,
			"toolName":   evt.ToolName,
			"authSpec":   evt.AuthSpec,
			"agentName":  evt.AgentName,
		})
	case *thread.InputProvided:
		return customEvent("INPUT_PROVIDED", map[string]any{
			"toolCallId": evt.ToolCallID,
			"messageId":  evt.MessageID,
			"input":      evt.Input,
		})
	case *thread.AuthGranted:
		return customEvent("AUTH_GRANTED", map[string]any{"toolCallId": evt.ToolCallID})
	case *thread.AuthDenied:
		return customEvent("AUTH_DENIED", map[string]any{"toolCallId": evt.ToolCallID})
	case *thread.ToolCancelled:
		return customEvent("TOOL_CANCELLED", map[string]any{
			"toolCallId": evt.ToolCallID,
			"toolName":   evt.ToolName,
		})
	case *thread.AgentTransferStarted:
		return customEvent("AGENT_TRANSFER_STARTED", agentTransferPayload(evt))
	case *thread.A2AArtifactReceived:
		return a2aArtifactEvent(evt)
	case *thread.FileAttachmentReceived:
		return customEvent("FILE_ATTACHMENT", map[string]any{
			"fileUrl":   evt.FileURL,
			"fileName":  evt.FileName,
			"mimeType":  evt.MimeType,
			"agentName": evt.AgentName,
		})
	case *thread.WorkflowNodeCompleted:
		return customEvent("WORKFLOW_NODE_COMPLETED", map[string]any{
			"agentName":  evt.AgentName,
			"durationMs": evt.DurationMs,
			"outcome":    evt.Outcome,
			"timestamp":  evt.Timestamp,
		})
	case *thread.StateChangeReported:
		return customEvent("STATE_CHANGE_REPORTED", map[string]any{
			"agentName": evt.AgentName,
			"key":       evt.Key,
			"value":     evt.Value,
			"timestamp": evt.Timestamp,
		})
	case *thread.ArtifactVersionSaved:
		return customEvent("ARTIFACT_VERSION_SAVED", map[string]any{
			"agentName": evt.AgentName,
			"fileName":  evt.FileName,
			"version":   evt.Version,
			"timestamp": evt.Timestamp,
		})
	case *thread.CodeExecutionRan:
		// Surfaces Gemini-executed (and OpenAI code-interpreter) snippets
		// to the chat stream. Frontend renders a card with the source in
		// a highlighted block and the sandbox output below. BlockID
		// uniquely identifies this block so the live stream and the
		// persisted-history replay reconcile.
		return customEvent("CODE_EXECUTION_RAN", map[string]any{
			"messageId": evt.MessageID,
			"blockId":   evt.BlockID,
			"language":  evt.Language,
			"code":      evt.Code,
			"output":    evt.Output,
			"outcome":   evt.Outcome,
			"agentName": evt.AgentName,
			"timestamp": evt.Timestamp,
		})
	case *thread.UserMessageAdded, *thread.AssistantMessageAdded, *thread.ThreadCreated:
		// Handled by the frontend locally or via snapshots — no need to stream.
		return []any{}
	default:
		return nil
	}
}

// singleton wraps a single AGUI event as the returned slice. Many event
// types translate 1:1 and this keeps the per-arm code to one line.
func singleton(e any) []any { return []any{e} }

// customEvent emits a single AGUI CustomEvent carrying the given name
// and payload map. Used for every domain event that doesn't have a
// native AGUI equivalent.
func customEvent(name string, payload map[string]any) []any {
	return []any{aguievents.NewCustomEvent(name, aguievents.WithValue(payload))}
}

func tokenUsagePayload(evt *thread.TokenUsageReported) map[string]any {
	return map[string]any{
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
	}
}

func agentTransferPayload(evt *thread.AgentTransferStarted) map[string]any {
	return map[string]any{
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
	}
}

// toolResultEvent picks the event name (TOOL_ERROR or TOOL_SUCCESS)
// from the IsError flag, then builds a common payload.
func toolResultEvent(evt *thread.ToolResultAdded) []any {
	name := "TOOL_SUCCESS"
	if evt.IsError {
		name = "TOOL_ERROR"
	}
	return customEvent(name, map[string]any{
		"toolCallId": evt.ToolCallID,
		"toolName":   evt.ToolName,
		"result":     evt.Result,
		"isError":    evt.IsError,
		"agentName":  evt.AgentName,
		"timestamp":  evt.Timestamp,
	})
}

// a2aArtifactEvent emits a generic ARTIFACT custom event — the frontend
// treats this as a UI affordance (document card) with no knowledge of
// A2A internals. ArtifactID falls back to MessageID when absent so
// every artifact has a stable identifier.
func a2aArtifactEvent(evt *thread.A2AArtifactReceived) []any {
	id := evt.ArtifactID
	if id == "" {
		id = evt.MessageID
	}
	return customEvent("ARTIFACT", map[string]any{
		"id":        id,
		"title":     evt.Title,
		"content":   evt.Content,
		"mimeType":  "text/markdown",
		"agentName": evt.AgentName,
	})
}
