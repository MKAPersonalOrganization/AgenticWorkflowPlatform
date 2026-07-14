package adkgo

import (
	"aicompass/modules/chat/thread"
	"encoding/json"
	"strings"
	"time"

	"google.golang.org/adk/v2/session"
	"google.golang.org/adk/v2/tool/toolconfirmation"
	"google.golang.org/genai"
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
//   - State delta extraction → StateChangeReported events (events_state.go)
//   - Tool elicitation detection → ToolNeedsInput/ToolNeedsConfirmation events
//   - Agent transfer detection → AgentTransferRequested events
//   - Escalation detection → thread metadata
//   - Error response detection with convention-based heuristics (events_tool.go)
//
// File split:
//   - events.go                    — dispatcher + user/model translators + content helpers
//   - events_state.go              — state delta + artifact delta + a2a markers
//   - events_tool.go               — function response translation + error heuristics
//   - events_code.go               — built-in code execution translation
//   - events_confirmation.go       — adk_request_confirmation argument parsing
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
	switch evt.Author {
	case "user":
		events = append(events, translateUserEvent(evt, ctx)...)
	default:
		// "model", "", or agent name — all are model/assistant events.
		events = append(events, translateModelEvent(evt, ctx)...)
	}
	events = append(events, translateStateDelta(evt, ctx)...)
	events = append(events, translateArtifactDelta(evt, ctx)...)
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

	// ToolStart / ToolNeedsInput / ToolNeedsConfirmation for each function call.
	for _, tc := range toolCalls {
		events = append(events, toolCallEvents(evt, ctx, tc, now)...)
	}

	// Tool results (function responses) within the same event.
	events = append(events, translateFunctionResponses(evt, ctx)...)

	// Built-in code execution parts (Gemini today; OpenAI / Anthropic
	// later plug into the same translation). See events_code.go.
	events = append(events, translateCodeExecution(evt, ctx)...)

	// Token usage.
	if u := translateUsageMetadata(evt, ctx); u != nil {
		events = append(events, u)
	}
	return events
}

// toolCallEvents emits the correct lifecycle event for a single
// FunctionCall: ToolStart for plain tools, or the confirmation/input
// elicitation variants when the call is an adk_request_confirmation
// wrapper. The wrapper's own call ID is what the frontend must send back
// on resume.
func toolCallEvents(evt *session.Event, ctx EventContext, tc thread.ToolCall, now time.Time) []thread.Event {
	if tc.Name != toolconfirmation.FunctionCallName {
		return []thread.Event{&thread.ToolStart{
			ThreadID:      ctx.ThreadID,
			MessageID:     evt.ID,
			ToolCallID:    tc.ID,
			ToolName:      tc.Name,
			ToolArguments: tc.Arguments,
			AgentName:     ctx.AgentName,
			Timestamp:     now,
		}}
	}
	info := extractConfirmationInfo(tc.Arguments)
	if info.inputSchema != nil {
		return []thread.Event{&thread.ToolNeedsInput{
			ThreadID:           ctx.ThreadID,
			MessageID:          evt.ID,
			ToolCallID:         tc.ID,
			OriginalToolCallID: info.originalID,
			ToolName:           info.originalName,
			Elicitation: &thread.ElicitationSpec{
				Question: info.hint,
				Schema:   info.inputSchema,
			},
			AgentName: ctx.AgentName,
			Timestamp: now,
		}}
	}
	return []thread.Event{&thread.ToolNeedsConfirmation{
		ThreadID:           ctx.ThreadID,
		MessageID:          evt.ID,
		ToolCallID:         tc.ID,
		OriginalToolCallID: info.originalID,
		ToolName:           info.originalName,
		Question:           info.hint,
		AgentName:          ctx.AgentName,
		Timestamp:          now,
	}}
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
