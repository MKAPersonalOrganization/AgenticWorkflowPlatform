package adkgo

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"aicompass/modules/chat/thread"

	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/toolconfirmation"
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
//   - State delta extraction → StateChangeReported events
//   - Tool elicitation detection → ToolNeedsInput/ToolNeedsConfirmation events
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
	//
	// When the call is an adk_request_confirmation wrapper, we emit
	// ToolNeedsInput or ToolNeedsConfirmation (with the wrapper's call ID,
	// which the frontend must send back) instead of ToolStart.
	for _, tc := range toolCalls {
		if tc.Name == toolconfirmation.FunctionCallName {
			info := extractConfirmationInfo(tc.Arguments)
			if info.inputSchema != nil {
				// Structured input request (e.g. io.Read)
				events = append(events, &thread.ToolNeedsInput{
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
				})
			} else {
				// Simple binary Allow/Deny confirmation
				events = append(events, &thread.ToolNeedsConfirmation{
					ThreadID:           ctx.ThreadID,
					MessageID:          evt.ID,
					ToolCallID:         tc.ID,
					OriginalToolCallID: info.originalID,
					ToolName:           info.originalName,
					Question:           info.hint,
					AgentName:          ctx.AgentName,
					Timestamp:          now,
				})
			}
			continue
		}
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

	// Built-in code execution parts (Gemini today; OpenAI / Anthropic
	// later plug into the same translation). The provider emits
	// ExecutableCode (source) and CodeExecutionResult (stdout/stderr)
	// as distinct genai.Parts. We pair them into a single
	// CodeExecutionRan domain event keyed by their shared ID so the UI
	// can render code + output together.
	events = append(events, translateCodeExecution(evt, ctx)...)

	// Token usage.
	if u := translateUsageMetadata(evt, ctx); u != nil {
		events = append(events, u)
	}

	return events
}

// confirmationInfo holds parsed fields from an adk_request_confirmation
// FunctionCall's JSON-encoded arguments.
type confirmationInfo struct {
	originalID   string
	originalName string
	hint         string
	// inputSchema is non-nil when the confirmation is actually a custom input
	// request (Python ADK's adk_request_input pattern). The payload contains
	// {"type":"custom_input","schema":{...}}.
	inputSchema map[string]any
}

// findKey case-insensitively finds a value in a map for the given key.
func findKey(m map[string]any, keys ...string) (any, bool) {
	for k, v := range m {
		for _, target := range keys {
			if strings.EqualFold(k, target) {
				return v, true
			}
		}
	}
	return nil, false
}

// extractConfirmationInfo parses the originalFunctionCall name, ID,
// toolConfirmation hint, and optional custom input payload from an
// adk_request_confirmation FunctionCall's JSON-encoded arguments.
// extractOriginalToolCall populates info.originalID / originalName from
// the confirmation payload. It prefers the nested "originalFunctionCall"
// object (current shape), falling back to the flat top-level fields used
// by older ADK-Go versions.
func extractOriginalToolCall(raw map[string]any, info *confirmationInfo) {
	if ofcVal, ok := findKey(raw, "originalFunctionCall"); ok {
		if ofc, ok := ofcVal.(map[string]any); ok {
			if id, ok := findKey(ofc, "id"); ok {
				info.originalID, _ = id.(string)
			}
			if name, ok := findKey(ofc, "name"); ok {
				info.originalName, _ = name.(string)
			}
		}
	}
	if info.originalID == "" {
		if id, ok := findKey(raw, "originalToolCallId", "originalId", "id"); ok {
			info.originalID, _ = id.(string)
		}
	}
	if info.originalName == "" {
		if name, ok := findKey(raw, "originalToolName", "originalName", "name"); ok {
			info.originalName, _ = name.(string)
		}
	}
}

// extractToolConfirmation populates info.hint and returns the payload
// map (if any). Prefers the nested "toolConfirmation" shape, with
// top-level hint/payload as a fallback for older payloads.
func extractToolConfirmation(raw map[string]any, info *confirmationInfo) map[string]any {
	var payload map[string]any
	if tcVal, ok := findKey(raw, "toolConfirmation"); ok {
		if tc, ok := tcVal.(map[string]any); ok {
			if hint, ok := findKey(tc, "hint"); ok {
				info.hint, _ = hint.(string)
			}
			if pVal, ok := findKey(tc, "payload"); ok {
				payload, _ = pVal.(map[string]any)
			}
		}
	}
	if info.hint == "" {
		if hint, ok := findKey(raw, "hint"); ok {
			info.hint, _ = hint.(string)
		}
	}
	if payload == nil {
		if pVal, ok := findKey(raw, "payload"); ok {
			payload, _ = pVal.(map[string]any)
		}
	}
	return payload
}

// applyCustomInputPayload sets info.inputSchema when the payload carries
// a custom-input request. The fallback schema ensures the UI still
// renders a text input if the payload omits an explicit schema.
func applyCustomInputPayload(payload map[string]any, info *confirmationInfo) {
	typeVal, _ := findKey(payload, "type")
	payloadType, _ := typeVal.(string)
	if !strings.EqualFold(payloadType, "custom_input") {
		return
	}
	if sVal, ok := findKey(payload, "schema"); ok {
		if schema, ok := sVal.(map[string]any); ok {
			info.inputSchema = schema
		}
	}
	if info.inputSchema == nil {
		info.inputSchema = map[string]any{"type": "string"}
	}
}

func extractConfirmationInfo(argsJSON string) confirmationInfo {
	var raw map[string]any
	if err := json.Unmarshal([]byte(argsJSON), &raw); err != nil {
		return confirmationInfo{}
	}

	info := confirmationInfo{}
	extractOriginalToolCall(raw, &info)
	payload := extractToolConfirmation(raw, &info)
	if payload != nil {
		applyCustomInputPayload(payload, &info)
	}
	return info
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

	// A2A sub-agents encode file / artifact metadata as "__a2a_*" markers
	// in the state delta (see buildRemoteAgentNode in workflow.go). These
	// short-circuit the normal state-change emission path so the frontend
	// sees them as dedicated events, not as opaque state writes.
	if fileRaw, ok := evt.Actions.StateDelta["__a2a_file__"]; ok {
		return a2aFileEvent(fileRaw, evt, ctx)
	}
	if artifactRaw, ok := evt.Actions.StateDelta["__a2a_artifact__"]; ok {
		return a2aArtifactEvent(artifactRaw, evt, ctx)
	}
	return stateChangeEvents(evt, ctx)
}

// a2aFileEvent decodes an "__a2a_file__" marker into a
// FileAttachmentReceived event, or returns nil when the marker is
// malformed or empty (never promoted into a regular state change).
func a2aFileEvent(raw any, evt *session.Event, ctx EventContext) []thread.Event {
	fileJSON, ok := raw.(string)
	if !ok || fileJSON == "" {
		return nil
	}
	var info struct {
		FileURL  string `json:"fileURL"`
		FileName string `json:"fileName"`
		MimeType string `json:"mimeType"`
	}
	if err := json.Unmarshal([]byte(fileJSON), &info); err != nil || info.FileURL == "" {
		return nil
	}
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

// a2aArtifactEvent decodes an "__a2a_artifact__" marker into an
// A2AArtifactReceived event, or returns nil when malformed.
func a2aArtifactEvent(raw any, evt *session.Event, ctx EventContext) []thread.Event {
	artifactJSON, ok := raw.(string)
	if !ok || artifactJSON == "" {
		return nil
	}
	var info struct {
		ArtifactID string `json:"artifactId"`
		Title      string `json:"title"`
		Content    string `json:"content"`
	}
	if err := json.Unmarshal([]byte(artifactJSON), &info); err != nil || info.Content == "" {
		return nil
	}
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

// stateChangeEvents emits one StateChangeReported per user-visible key.
// Internal markers ("__" prefix) and ephemeral ADK scope ("temp:" prefix)
// are suppressed; session- / app: / user:-scoped keys are reported.
func stateChangeEvents(evt *session.Event, ctx EventContext) []thread.Event {
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

// isConfirmationRequiredResponse returns true when the FunctionResponse indicates
//
// Convention established in tools.go resultToMap():
//   - Success: map has content keys at top level (e.g. "success", "audio_url")
//   - Error:   map has an "error" key and NO "result"/"results"/"success" keys
//
// This is the single source of truth for error detection. tools.go guarantees
// the shape; events.go just reads it.
// ──────────────────────────────────────────────────────────────────────────────

// translateCodeExecution walks an event's content parts and emits one
// CodeExecutionRan domain event per (ExecutableCode, CodeExecutionResult)
// pair that came back from the provider. Pairs are matched by the `ID`
// field — Gemini sets the same ID on both parts so the UI can render
// the source and its output in a single card.
//
// Edge cases:
//   - An ExecutableCode without a matching result is still emitted (with
//     empty Output) so the UI can at least show the generated source.
//   - A CodeExecutionResult without a prior ExecutableCode is dropped
//     silently — it would render as output-without-context, which is
//     worse than not rendering at all.
//   - Multiple pairs in the same event are supported (the model can
//     execute several times per turn).
//
// Provider-agnostic by design: the function reads `genai.Part` fields
// (ExecutableCode, CodeExecutionResult) that exist for every provider
// in the genai types; only Gemini populates them today, but when OpenAI
// or Anthropic adapters surface their own code-interpreter outputs they
// will translate through this same path.
func translateCodeExecution(evt *session.Event, ctx EventContext) []thread.Event {
	if evt == nil || evt.Content == nil {
		return nil
	}

	codes, codeOrder, results := collectCodeParts(evt.Content.Parts)
	if len(codes) == 0 {
		return nil
	}

	now := coalesceTime(evt.Timestamp)
	events := make([]thread.Event, 0, len(codes))
	for idx, id := range codeOrder {
		events = append(events, buildCodeExecutionEvent(evt.ID, idx, id, codes[id], results[id], ctx, now))
	}
	return events
}

// codeEntry is the per-ID pair of language + source collected from a stream
// of genai parts before emission.
type codeEntry struct {
	language string
	code     string
}

// collectCodeParts walks the parts once and partitions them into ordered
// code blocks (keyed by ID) and execution results (keyed by the same ID so
// they can be paired in the emission phase).
func collectCodeParts(parts []*genai.Part) (map[string]codeEntry, []string, map[string]*genai.CodeExecutionResult) {
	codes := make(map[string]codeEntry)
	var codeOrder []string
	results := make(map[string]*genai.CodeExecutionResult)

	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.ExecutableCode != nil {
			id := p.ExecutableCode.ID
			codes[id] = codeEntry{
				language: strings.ToLower(string(p.ExecutableCode.Language)),
				code:     p.ExecutableCode.Code,
			}
			codeOrder = append(codeOrder, id)
		}
		if p.CodeExecutionResult != nil {
			results[p.CodeExecutionResult.ID] = p.CodeExecutionResult
		}
	}
	return codes, codeOrder, results
}

// buildCodeExecutionEvent pairs a collected code block with its matching
// execution result (if any) and materialises a thread.CodeExecutionRan.
// When the provider omitted the part ID (synthetic Outcome-only case),
// a deterministic blockID is synthesised from evt.ID + position so replay
// dedupes correctly.
func buildCodeExecutionEvent(evtID string, idx int, id string, c codeEntry, res *genai.CodeExecutionResult, ctx EventContext, now time.Time) *thread.CodeExecutionRan {
	blockID := id
	if blockID == "" {
		blockID = fmt.Sprintf("%s-%d", evtID, idx)
	}
	out := &thread.CodeExecutionRan{
		ThreadID:  ctx.ThreadID,
		MessageID: evtID,
		BlockID:   blockID,
		Language:  c.language,
		Code:      c.code,
		AgentName: ctx.AgentName,
		Timestamp: now,
	}
	if res != nil {
		out.Output = res.Output
		out.Outcome = string(res.Outcome)
	}
	return out
}

func translateFunctionResponses(evt *session.Event, ctx EventContext) []thread.Event {
	var events []thread.Event
	now := coalesceTime(evt.Timestamp)

	for _, p := range evt.Content.Parts {
		if p.FunctionResponse == nil {
			continue
		}

		resp := p.FunctionResponse.Response

		if isConfirmationRequiredResponse(resp) {
			continue
		}

		resultJSON, _ := json.Marshal(resp)

		events = append(events, &thread.ToolResultAdded{
			ThreadID:   ctx.ThreadID,
			MessageID:  evt.ID,
			ToolCallID: p.FunctionResponse.ID,
			ToolName:   p.FunctionResponse.Name,
			Result:     string(resultJSON),
			IsError:    isErrorResponse(resp),
			AgentName:  ctx.AgentName,
			Timestamp:  now,
		})
	}

	return events
}

// isConfirmationRequiredResponse returns true when the FunctionResponse indicates
// the tool is waiting for user confirmation. These responses should NOT emit
// ToolResultAdded because the ToolNeedsInput event already handles this case.
func isConfirmationRequiredResponse(resp map[string]any) bool {
	if resp == nil {
		return false
	}
	status, ok := resp["status"].(string)
	return ok && status == "confirmation_required"
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
