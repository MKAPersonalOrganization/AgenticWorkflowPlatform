package adkgo

import (
	"encoding/json"
	"strings"
	"time"

	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/genai"

	"aicompass/modules/chat/thread"
)

// ──────────────────────────────────────────────────────────────────────────────
// Event Translation
//
// TranslateEvent converts a single adk-go session.Event into zero or more
// thread domain events. A single adk-go event can fan out to multiple domain
// events (e.g. assistant message + tool start for each function call).
//
// The translation is stateless — streaming lifecycle tracking (message start/
// end) is handled by the caller in runner.go.
//
// Production-ready features:
//   - State delta extraction → StateChangeReported events
//   - Tool confirmation detection → ToolNeedsInput events
//   - Agent transfer detection → AgentTransferRequested events
//   - Escalation detection → thread metadata
//   - Error response detection with convention-based heuristics
// ──────────────────────────────────────────────────────────────────────────────

// EventContext carries per-run context needed for event translation.
type EventContext struct {
	ThreadID  string
	RunID     string
	AgentName string
	Provider  string
	Model     string
}

// TranslateEvent translates a single adk-go session event to thread domain events.
func TranslateEvent(evt *session.Event, ctx EventContext) []thread.Event {
	if evt == nil || evt.Content == nil {
		return translateNonContentEvent(evt, ctx)
	}

	var events []thread.Event

	// Translate based on author.
	switch evt.Author {
	case "user":
		events = append(events, translateUserEvent(evt, ctx)...)
	default:
		// "model", "", or agent name — all are model/assistant events.
		events = append(events, translateModelEvent(evt, ctx)...)
	}

	// Extract state delta events (ADK-Go EventActions.StateDelta).
	events = append(events, translateStateDelta(evt, ctx)...)

	// Extract artifact delta events (ADK-Go EventActions.ArtifactDelta).
	events = append(events, translateArtifactDelta(evt, ctx)...)

	// Extract tool confirmation events (ADK-Go HITL).
	events = append(events, translateToolConfirmations(evt, ctx)...)

	return events
}

// ──────────────────────────────────────────────────────────────────────────────
// Non-Content Events (usage-only, state-delta-only, etc.)
// ──────────────────────────────────────────────────────────────────────────────

func translateNonContentEvent(evt *session.Event, ctx EventContext) []thread.Event {
	var events []thread.Event

	if u := translateUsageMetadata(evt, ctx); u != nil {
		events = append(events, u)
	}

	// State deltas can exist even without content.
	events = append(events, translateStateDelta(evt, ctx)...)

	return events
}

// ──────────────────────────────────────────────────────────────────────────────
// User Events
// ──────────────────────────────────────────────────────────────────────────────

func translateUserEvent(evt *session.Event, ctx EventContext) []thread.Event {
	var events []thread.Event

	if text := extractTextFromContent(evt.Content); text != "" {
		events = append(events, &thread.UserMessageAdded{
			ThreadID:  ctx.ThreadID,
			MessageID: evt.ID,
			Content:   text,
			Timestamp: evt.Timestamp,
		})
	}

	// Function responses (tool results returned by the runtime on behalf of user).
	events = append(events, translateFunctionResponses(evt, ctx)...)

	return events
}

// ──────────────────────────────────────────────────────────────────────────────
// Model / Assistant Events
// ──────────────────────────────────────────────────────────────────────────────

func translateModelEvent(evt *session.Event, ctx EventContext) []thread.Event {
	var events []thread.Event
	now := coalesceTime(evt.Timestamp)

	text := extractTextFromContent(evt.Content)
	toolCalls := extractToolCalls(evt.Content)

	// Streaming delta — partial content, no tool calls.
	if evt.Partial {
		if text != "" {
			events = append(events, &thread.AssistantContentStreamed{
				ThreadID:  ctx.ThreadID,
				MessageID: evt.ID,
				Delta:     text,
				Timestamp: now,
			})
		}
		return events
	}

	// Complete model turn — assistant message with optional tool calls.
	if text != "" || len(toolCalls) > 0 {
		events = append(events, &thread.AssistantMessageAdded{
			ThreadID:  ctx.ThreadID,
			MessageID: evt.ID,
			Content:   text,
			ToolCalls: toolCalls,
			AgentName: ctx.AgentName,
			Timestamp: now,
		})
	}

	// ToolStart for each function call the LLM wants to invoke.
	for _, tc := range toolCalls {
		events = append(events, &thread.ToolStart{
			ThreadID:      ctx.ThreadID,
			MessageID:     evt.ID,
			ToolCallID:    tc.ID,
			ToolName:      tc.Name,
			ToolArguments: tc.Arguments,
			AgentName:     ctx.AgentName,
			Timestamp:     now,
		})
	}

	// Tool results (function responses) within the same event.
	events = append(events, translateFunctionResponses(evt, ctx)...)

	// Token usage.
	if u := translateUsageMetadata(evt, ctx); u != nil {
		events = append(events, u)
	}

	return events
}

// ──────────────────────────────────────────────────────────────────────────────
// State Delta Translation
//
// ADK-Go accumulates state changes in EventActions.StateDelta. These are
// transactional — applied when the event is committed. We translate non-internal
// state changes to thread.StateChangeReported events.
// ──────────────────────────────────────────────────────────────────────────────

func translateStateDelta(evt *session.Event, ctx EventContext) []thread.Event {
	if evt == nil || len(evt.Actions.StateDelta) == 0 {
		return nil
	}

	// Check for A2A artifact marker embedded by buildRemoteAgentNode in workflow.go.
	// When a remote A2A agent yields an artifact chunk, the workflow node encodes
	// the artifact metadata into StateDelta["__a2a_artifact__"] and emits a
	// content-less session event so artifacts reach the frontend as a separate
	// thread.A2AArtifactReceived event — distinct from the streaming text bubble.
	// Check for file attachment marker.
	if fileRaw, ok := evt.Actions.StateDelta["__a2a_file__"]; ok {
		if fileJSON, ok := fileRaw.(string); ok && fileJSON != "" {
			var info struct {
				FileURL  string `json:"fileURL"`
				FileName string `json:"fileName"`
				MimeType string `json:"mimeType"`
			}
			if err := json.Unmarshal([]byte(fileJSON), &info); err == nil && info.FileURL != "" {
				return []thread.Event{&thread.FileAttachmentReceived{
					ThreadID:  ctx.ThreadID,
					MessageID: evt.ID,
					AgentName: ctx.AgentName,
					FileURL:   info.FileURL,
					FileName:  info.FileName,
					MimeType:  info.MimeType,
					Timestamp: coalesceTime(evt.Timestamp),
				}}
			}
		}
		return nil
	}

	if artifactRaw, ok := evt.Actions.StateDelta["__a2a_artifact__"]; ok {
		if artifactJSON, ok := artifactRaw.(string); ok && artifactJSON != "" {
			var info struct {
				ArtifactID string `json:"artifactId"`
				Title      string `json:"title"`
				Content    string `json:"content"`
			}
			if err := json.Unmarshal([]byte(artifactJSON), &info); err == nil && info.Content != "" {
				return []thread.Event{&thread.A2AArtifactReceived{
					ThreadID:   ctx.ThreadID,
					MessageID:  evt.ID,
					AgentName:  ctx.AgentName,
					ArtifactID: info.ArtifactID,
					Title:      info.Title,
					Content:    info.Content,
					Timestamp:  coalesceTime(evt.Timestamp),
				}}
			}
		}
		return nil
	}

	// Emit StateChangeReported for user-visible state mutations.
	// Suppressed: internal markers ("__" prefix) and ADK-Go ephemeral scope ("temp:" prefix).
	// Reported: session-scoped, app-scoped ("app:"), and user-scoped ("user:") keys.
	var events []thread.Event
	now := coalesceTime(evt.Timestamp)
	for k, v := range evt.Actions.StateDelta {
		if strings.HasPrefix(k, "__") || strings.HasPrefix(k, "temp:") {
			continue
		}
		events = append(events, &thread.StateChangeReported{
			ThreadID:  ctx.ThreadID,
			AgentName: ctx.AgentName,
			Key:       k,
			Value:     v,
			Timestamp: now,
		})
	}
	return events
}

// ──────────────────────────────────────────────────────────────────────────────
// Artifact Delta Translation
//
// ADK-Go tracks artifact saves in EventActions.ArtifactDelta (map[string]int64).
// Each entry maps an artifact filename to the version that was saved during this
// event. We translate these to ArtifactVersionSaved domain events so the
// frontend can display artifact cards with version info.
// ──────────────────────────────────────────────────────────────────────────────

func translateArtifactDelta(evt *session.Event, ctx EventContext) []thread.Event {
	if evt == nil || len(evt.Actions.ArtifactDelta) == 0 {
		return nil
	}

	var events []thread.Event
	now := coalesceTime(evt.Timestamp)
	for fileName, version := range evt.Actions.ArtifactDelta {
		events = append(events, &thread.ArtifactVersionSaved{
			ThreadID:  ctx.ThreadID,
			AgentName: ctx.AgentName,
			FileName:  fileName,
			Version:   version,
			Timestamp: now,
		})
	}
	return events
}

// ──────────────────────────────────────────────────────────────────────────────
// Tool Confirmation Translation (HITL)
//
// ADK-Go's tool confirmation system stores pending confirmations in
// EventActions.RequestedToolConfirmations. We translate these to
// thread.ToolNeedsInput events so the frontend can display approval dialogs.
// ──────────────────────────────────────────────────────────────────────────────

func translateToolConfirmations(evt *session.Event, ctx EventContext) []thread.Event {
	if evt == nil || len(evt.Actions.RequestedToolConfirmations) == 0 {
		return nil
	}

	// Build a callID → toolName lookup from any FunctionCall parts in the event.
	// ADK-Go keys RequestedToolConfirmations by function call ID, not tool name.
	// The matching FunctionCall parts (if present) carry the actual tool name.
	callIDToName := make(map[string]string)
	if evt.Content != nil {
		for _, p := range evt.Content.Parts {
			if p.FunctionCall != nil && p.FunctionCall.ID != "" {
				callIDToName[p.FunctionCall.ID] = p.FunctionCall.Name
			}
		}
	}

	var events []thread.Event
	now := coalesceTime(evt.Timestamp)

	for callID, confirmation := range evt.Actions.RequestedToolConfirmations {
		toolName := callIDToName[callID]
		if toolName == "" {
			toolName = callID // fallback: use call ID when name can't be resolved
		}
		events = append(events, &thread.ToolNeedsInput{
			ThreadID:   ctx.ThreadID,
			MessageID:  evt.ID,
			ToolCallID: callID,
			ToolName:   toolName,
			Elicitation: &thread.ElicitationSpec{
				Question: confirmation.Hint,
			},
			AgentName: ctx.AgentName,
			Timestamp: now,
		})
	}

	return events
}

// ──────────────────────────────────────────────────────────────────────────────
// Function Response → ToolResultAdded
//
// Convention established in tools.go resultToMap():
//   - Success: map has content keys at top level (e.g. "success", "audio_url")
//   - Error:   map has an "error" key and NO "result"/"results"/"success" keys
//
// This is the single source of truth for error detection. tools.go guarantees
// the shape; events.go just reads it.
// ──────────────────────────────────────────────────────────────────────────────

func translateFunctionResponses(evt *session.Event, ctx EventContext) []thread.Event {
	var events []thread.Event
	now := coalesceTime(evt.Timestamp)

	for _, p := range evt.Content.Parts {
		if p.FunctionResponse == nil {
			continue
		}

		resultJSON, _ := json.Marshal(p.FunctionResponse.Response)

		events = append(events, &thread.ToolResultAdded{
			ThreadID:   ctx.ThreadID,
			MessageID:  evt.ID,
			ToolCallID: p.FunctionResponse.ID,
			ToolName:   p.FunctionResponse.Name,
			Result:     string(resultJSON),
			IsError:    isErrorResponse(p.FunctionResponse.Response),
			AgentName:  ctx.AgentName,
			Timestamp:  now,
		})
	}

	return events
}

// isErrorResponse returns true when the FunctionResponse map represents an
// error result. The convention (from tools.go) is: the map contains an "error"
// key and no success-indicator keys ("result", "results", "success").
//
// ADK-Go v1.0.0 introduced ErrConfirmationRequired and ErrConfirmationRejected
// as exported sentinel errors. When a native ADK tool using tool.WithConfirmation()
// surfaces one of these, the error string is embedded in the result map. We
// detect these and return false so the frontend does not show a red error
// indicator for HITL flow-control signals.
func isErrorResponse(resp map[string]any) bool {
	if resp == nil {
		return false
	}
	if _, hasError := resp[errorKey]; !hasError {
		return false
	}
	// If the map also has success indicators, it's a partial failure but
	// not a pure error (the tool returned usable data alongside the error).
	for _, key := range []string{"result", "results", "success"} {
		if _, exists := resp[key]; exists {
			return false
		}
	}
	// Confirmation-flow errors are HITL signals, not failures.
	// Our toolErrorCallback converts these to {"status":"confirmation_required",...}
	// maps, but if they bypass that path the raw error string may appear here.
	if errVal, ok := resp[errorKey].(string); ok {
		if strings.Contains(errVal, adktool.ErrConfirmationRequired.Error()) ||
			strings.Contains(errVal, adktool.ErrConfirmationRejected.Error()) {
			return false
		}
	}
	return true
}

// ──────────────────────────────────────────────────────────────────────────────
// Usage Metadata
// ──────────────────────────────────────────────────────────────────────────────

func translateUsageMetadata(evt *session.Event, ctx EventContext) thread.Event {
	if evt == nil || evt.UsageMetadata == nil || evt.UsageMetadata.TotalTokenCount == 0 {
		return nil
	}
	return &thread.TokenUsageReported{
		ThreadID:         ctx.ThreadID,
		MessageID:        evt.ID,
		Provider:         ctx.Provider,
		Model:            ctx.Model,
		PromptTokens:     int(evt.UsageMetadata.PromptTokenCount),
		CompletionTokens: int(evt.UsageMetadata.CandidatesTokenCount),
		TotalTokens:      int(evt.UsageMetadata.TotalTokenCount),
		Timestamp:        coalesceTime(evt.Timestamp),
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Content Extraction Helpers
// ──────────────────────────────────────────────────────────────────────────────

// extractTextFromContent concatenates non-thought text parts from genai.Content.
// Thought parts (internal reasoning produced by thinking models like Gemini 2.5)
// are filtered out — they must not be shown in the chat UI.
func extractTextFromContent(c *genai.Content) string {
	if c == nil {
		return ""
	}
	var sb strings.Builder
	for _, p := range c.Parts {
		if p.Text != "" && !p.Thought {
			sb.WriteString(p.Text)
		}
	}
	return sb.String()
}

// extractToolCalls extracts FunctionCall parts as thread.ToolCall slice.
func extractToolCalls(c *genai.Content) []thread.ToolCall {
	if c == nil {
		return nil
	}
	var calls []thread.ToolCall
	for _, p := range c.Parts {
		if p.FunctionCall != nil {
			argsJSON, _ := json.Marshal(p.FunctionCall.Args)
			calls = append(calls, thread.ToolCall{
				ID:        p.FunctionCall.ID,
				Type:      "function",
				Name:      p.FunctionCall.Name,
				Arguments: string(argsJSON),
			})
		}
	}
	return calls
}

// coalesceTime returns t if non-zero, otherwise time.Now().
func coalesceTime(t time.Time) time.Time {
	if t.IsZero() {
		return time.Now()
	}
	return t
}
