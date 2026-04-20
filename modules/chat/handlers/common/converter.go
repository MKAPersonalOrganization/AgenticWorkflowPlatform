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

// eventTranslators is the ordered dispatch table from domain event to
// AGUI-event slice. Each translator reports whether it handled the event
// so toAguiEventsInternal can fall through to the next; an unhandled
// event returns nil (not []any{}) to match the historical default-arm.
var eventTranslators = []func(any) ([]any, bool){
	translateRunEvent,
	translateMessageEvent,
	translateToolPhaseEvent,
	translateToolElicitationEvent,
	translateWorkflowEvent,
	translateStateEvent,
	translateIgnoredEvent,
}

func toAguiEventsInternal(event any) []any {
	for _, translate := range eventTranslators {
		if result, ok := translate(event); ok {
			return result
		}
	}
	return nil
}

// translateRunEvent handles the run-lifecycle trio (started / finished /
// error) — each maps 1:1 to a native AGUI event.
func translateRunEvent(event any) ([]any, bool) {
	switch evt := event.(type) {
	case *thread.RunStarted:
		return singleton(aguievents.NewRunStartedEvent(evt.ThreadID, evt.RunID)), true
	case *thread.RunFinished:
		return singleton(aguievents.NewRunFinishedEvent(evt.ThreadID, evt.RunID)), true
	case *thread.RunError:
		return singleton(aguievents.NewRunErrorEvent(evt.Message, aguievents.WithErrorCode(evt.Code))), true
	}
	return nil, false
}

// translateMessageEvent covers the streaming-assistant + thread-level
// bookkeeping events: message start/content/end, title updates, token
// usage reports.
func translateMessageEvent(event any) ([]any, bool) {
	switch evt := event.(type) {
	case *thread.AssistantMessageStarted:
		return singleton(aguievents.NewTextMessageStartEvent(evt.MessageID, aguievents.WithRole("assistant"))), true
	case *thread.AssistantContentStreamed:
		return singleton(aguievents.NewTextMessageContentEvent(evt.MessageID, evt.Delta)), true
	case *thread.AssistantMessageCompleted:
		return singleton(aguievents.NewTextMessageEndEvent(evt.MessageID)), true
	case *thread.ThreadTitleUpdated:
		return customEvent("THREAD_TITLE_UPDATED", map[string]any{
			"threadId": evt.ThreadID,
			"title":    evt.Title,
		}), true
	case *thread.TokenUsageReported:
		return customEvent("TOKEN_USAGE_REPORTED", tokenUsagePayload(evt)), true
	}
	return nil, false
}

// translateToolPhaseEvent covers the normal tool lifecycle: start,
// result (success or error), and external cancellation.
func translateToolPhaseEvent(event any) ([]any, bool) {
	switch evt := event.(type) {
	case *thread.ToolStart:
		return customEvent("TOOL_START", map[string]any{
			"toolCallId": evt.ToolCallID,
			"toolName":   evt.ToolName,
			"arguments":  evt.ToolArguments,
			"agentName":  evt.AgentName,
			"timestamp":  evt.Timestamp,
		}), true
	case *thread.ToolResultAdded:
		return toolResultEvent(evt), true
	case *thread.ToolCancelled:
		return customEvent("TOOL_CANCELLED", map[string]any{
			"toolCallId": evt.ToolCallID,
			"toolName":   evt.ToolName,
		}), true
	}
	return nil, false
}

// translateToolElicitationEvent covers the HITL / elicitation / auth
// branches — confirmation / input / auth requests plus their resolutions.
func translateToolElicitationEvent(event any) ([]any, bool) {
	switch evt := event.(type) {
	case *thread.ToolNeedsConfirmation:
		return customEvent("TOOL_NEEDS_CONFIRMATION", map[string]any{
			"toolCallId":         evt.ToolCallID,
			"originalToolCallId": evt.OriginalToolCallID,
			"toolName":           evt.ToolName,
			"question":           evt.Question,
			"agentName":          evt.AgentName,
		}), true
	case *thread.ToolNeedsInput:
		return customEvent("TOOL_NEEDS_INPUT", map[string]any{
			"toolCallId":         evt.ToolCallID,
			"originalToolCallId": evt.OriginalToolCallID,
			"toolName":           evt.ToolName,
			"question":           evt.Elicitation.Question,
			"schema":             evt.Elicitation.Schema,
			"agentName":          evt.AgentName,
		}), true
	case *thread.ToolNeedsAuth:
		return customEvent("TOOL_NEEDS_AUTH", map[string]any{
			"toolCallId": evt.ToolCallID,
			"toolName":   evt.ToolName,
			"authSpec":   evt.AuthSpec,
			"agentName":  evt.AgentName,
		}), true
	case *thread.InputProvided:
		return customEvent("INPUT_PROVIDED", map[string]any{
			"toolCallId": evt.ToolCallID,
			"messageId":  evt.MessageID,
			"input":      evt.Input,
		}), true
	case *thread.AuthGranted:
		return customEvent("AUTH_GRANTED", map[string]any{"toolCallId": evt.ToolCallID}), true
	case *thread.AuthDenied:
		return customEvent("AUTH_DENIED", map[string]any{"toolCallId": evt.ToolCallID}), true
	}
	return nil, false
}

// translateWorkflowEvent covers multi-agent / artifact-stream events —
// transfer hops, received A2A artifacts, file attachments, and workflow
// node completions.
func translateWorkflowEvent(event any) ([]any, bool) {
	switch evt := event.(type) {
	case *thread.AgentTransferStarted:
		return customEvent("AGENT_TRANSFER_STARTED", agentTransferPayload(evt)), true
	case *thread.A2AArtifactReceived:
		return a2aArtifactEvent(evt), true
	case *thread.FileAttachmentReceived:
		return customEvent("FILE_ATTACHMENT", map[string]any{
			"fileUrl":   evt.FileURL,
			"fileName":  evt.FileName,
			"mimeType":  evt.MimeType,
			"agentName": evt.AgentName,
		}), true
	case *thread.WorkflowNodeCompleted:
		return customEvent("WORKFLOW_NODE_COMPLETED", map[string]any{
			"agentName":  evt.AgentName,
			"durationMs": evt.DurationMs,
			"outcome":    evt.Outcome,
			"timestamp":  evt.Timestamp,
		}), true
	}
	return nil, false
}

// translateStateEvent covers the side-effect / telemetry events: state
// changes, artifact-version saves, code-execution results.
//
// CodeExecutionRan surfaces Gemini-executed (and OpenAI code-interpreter)
// snippets to the chat stream. Frontend renders a card with the source in
// a highlighted block and the sandbox output below. BlockID uniquely
// identifies this block so the live stream and the persisted-history
// replay reconcile.
func translateStateEvent(event any) ([]any, bool) {
	switch evt := event.(type) {
	case *thread.StateChangeReported:
		return customEvent("STATE_CHANGE_REPORTED", map[string]any{
			"agentName": evt.AgentName,
			"key":       evt.Key,
			"value":     evt.Value,
			"timestamp": evt.Timestamp,
		}), true
	case *thread.ArtifactVersionSaved:
		return customEvent("ARTIFACT_VERSION_SAVED", map[string]any{
			"agentName": evt.AgentName,
			"fileName":  evt.FileName,
			"version":   evt.Version,
			"timestamp": evt.Timestamp,
		}), true
	case *thread.CodeExecutionRan:
		return customEvent("CODE_EXECUTION_RAN", map[string]any{
			"messageId": evt.MessageID,
			"blockId":   evt.BlockID,
			"language":  evt.Language,
			"code":      evt.Code,
			"output":    evt.Output,
			"outcome":   evt.Outcome,
			"agentName": evt.AgentName,
			"timestamp": evt.Timestamp,
		}), true
	}
	return nil, false
}

// translateIgnoredEvent swallows domain events that the frontend handles
// locally or via snapshots — we still want ok=true so they don't bleed
// into the "unhandled" default arm, but there's no wire event to emit.
func translateIgnoredEvent(event any) ([]any, bool) {
	switch event.(type) {
	case *thread.UserMessageAdded, *thread.AssistantMessageAdded, *thread.ThreadCreated:
		return []any{}, true
	}
	return nil, false
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
