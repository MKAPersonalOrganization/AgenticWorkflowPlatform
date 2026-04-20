package adkgo

import (
	"aicompass/foundation/agent"
	agentio "aicompass/foundation/agent/io"
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/jsonschema-go/jsonschema"

	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// ──────────────────────────────────────────────────────────────────────────────
// Tool Wrapping
//
// WrapExecutableTool bridges foundation agent.ExecutableTool → adk-go tool.Tool.
// The bridge converts agent.Result → map[string]any (FunctionResponse.Response)
// so that events.go can marshal it back to JSON for the frontend.
//
// Contract: The returned map MUST be flat — the frontend parses the marshalled
// JSON directly (e.g. {"success":true,"audio_url":"..."}) and will break if
// tool output is nested under an extra key.
//
// Production features:
//   - Panic recovery: tool execution panics are caught and returned as errors
//   - Streaming tools: recognized and executed with streaming support
//   - Error surfacing: Go errors → {"error":"..."} so the LLM sees failures
// ──────────────────────────────────────────────────────────────────────────────

// WrapExecutableTool converts a foundation ExecutableTool to an adk-go Tool.
func WrapExecutableTool(et agent.ExecutableTool, sessionID, userID string) adktool.Tool {
	exec := wrapWithPanicRecovery(et, sessionID, userID)
	cfg := functiontool.Config{
		Name:          et.Name(),
		Description:   et.Description(),
		InputSchema:   agentSchemaToJSONSchema(et.Schema()),
		IsLongRunning: agent.IsLongRunningTool(et),
	}
	t, err := functiontool.New(cfg, exec)
	if err == nil {
		return t
	}
	// Schema conversion can fail for complex schemas. Fall back to a tool
	// without schema — the LLM still sees name + description.
	cfg.InputSchema = nil
	t, _ = functiontool.New(cfg, exec)
	return t
}

// wrapWithPanicRecovery returns the adktool execution closure with a
// deferred recover() so panics in tool code surface as structured
// {"error": "..."} results instead of crashing the agent loop.
func wrapWithPanicRecovery(et agent.ExecutableTool, sessionID, userID string) func(adktool.Context, map[string]any) (map[string]any, error) {
	return func(ctx adktool.Context, args map[string]any) (result map[string]any, err error) {
		defer func() {
			if r := recover(); r != nil {
				result = map[string]any{errorKey: fmt.Sprintf("tool panic: %v", r)}
				err = nil
			}
		}()
		return executeFoundationTool(ctx, et, args, sessionID, userID)
	}
}

// WrapExecutableTools converts a slice of foundation ExecutableTools.
func WrapExecutableTools(tools []agent.ExecutableTool, sessionID, userID string) []adktool.Tool {
	if len(tools) == 0 {
		return nil
	}
	wrapped := make([]adktool.Tool, 0, len(tools))
	for _, et := range tools {
		wrapped = append(wrapped, WrapExecutableTool(et, sessionID, userID))
	}
	return wrapped
}

// ──────────────────────────────────────────────────────────────────────────────
// Tool Execution
// ──────────────────────────────────────────────────────────────────────────────

// executeFoundationTool bridges adk-go's tool interface to the foundation
// ExecutableTool.Execute contract. It converts the typed agent.Result into
// the map[string]any that ADK-Go expects as FunctionResponse.Response.
//
// ctx is adktool.Context (not plain context.Context) so we can:
//   - populate Arguments.CallID from ctx.FunctionCallID()
//   - forward Result.Actions (TransferToAgent, Escalate, StateDelta) to
//     ctx.Actions(), matching ADK's tool.Context.Actions() semantics.
func executeFoundationTool(
	ctx adktool.Context,
	et agent.ExecutableTool,
	args map[string]any,
	sessionID, userID string,
) (map[string]any, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	execCtx := ensureIOHandler(ctx)
	result, err := et.Execute(execCtx, agent.Arguments{
		Raw:       raw,
		CallID:    ctx.FunctionCallID(),
		SessionID: sessionID,
		UserID:    userID,
	})
	if err != nil {
		return handleToolExecError(ctx, et, err)
	}
	applyEventActions(ctx, result.Actions)
	return resultToMap(result), nil
}

// ensureIOHandler injects the default PauseHandler when no io handler
// is already attached to ctx. Callers (e.g. the confirmation-resume
// path) may pre-attach a CachedReadHandler; we keep it when present.
func ensureIOHandler(ctx adktool.Context) context.Context {
	if agentio.FromContext(ctx) != nil {
		return ctx
	}
	return agentio.WithHandler(ctx, agentio.NewPauseHandler())
}

// handleToolExecError translates a foundation-tool execution error into
// the adk response shape. The ErrInputRequired branch routes through
// ADK's confirmation pause (the only pause primitive), marking the
// payload as custom_input so the frontend disambiguates it from a
// yes/no confirmation prompt. All other errors surface as
// {"error": "..."} so the LLM sees the failure and can recover.
func handleToolExecError(ctx adktool.Context, et agent.ExecutableTool, err error) (map[string]any, error) {
	var inputReq *agentio.ErrInputRequired
	if errors.As(err, &inputReq) {
		return requestInputPause(ctx, et, inputReq)
	}
	return map[string]any{errorKey: err.Error()}, nil
}

// requestInputPause bridges ErrInputRequired to ADK's confirmation
// protocol: build a custom_input payload (optionally carrying a schema)
// and ask the orchestrator to pause. SkipSummarization stops the LLM
// from freely talking after the pause.
func requestInputPause(ctx adktool.Context, et agent.ExecutableTool, inputReq *agentio.ErrInputRequired) (map[string]any, error) {
	payload := map[string]any{"type": "custom_input"}
	if inputReq.Schema != nil {
		payload["schema"] = inputReq.Schema
	}
	if reqErr := ctx.RequestConfirmation(inputReq.Question, payload); reqErr != nil {
		return map[string]any{errorKey: reqErr.Error()}, nil
	}
	ctx.Actions().SkipSummarization = true
	return nil, fmt.Errorf("tool %q %w", et.Name(), adktool.ErrConfirmationRequired)
}

// applyEventActions maps foundation EventActions onto ADK's mutable EventActions.
// This is the bridge for Result.Actions → ctx.Actions(), giving foundation tools
// the same routing capability as ADK's tool.Context.Actions().
//
// Note: EventActions.EndInvocation has no equivalent in ADK's tool.Context
// (it maps to InvocationContext.EndInvocation(), which is not accessible here).
// Tools that need to stop an invocation should do so via agent-level callbacks.
func applyEventActions(ctx adktool.Context, actions *agent.EventActions) {
	if actions == nil {
		return
	}
	adkActions := ctx.Actions()
	if adkActions == nil {
		return
	}
	if actions.TransferToAgent != "" {
		adkActions.TransferToAgent = actions.TransferToAgent
	}
	if actions.Escalate {
		adkActions.Escalate = true
	}
	mergeStateDelta(adkActions, actions.StateDelta)
}

// mergeStateDelta copies src's keys into adkActions.StateDelta,
// lazy-initialising the destination map when the tool emits the first
// delta for this call.
func mergeStateDelta(adkActions *session.EventActions, src map[string]any) {
	if len(src) == 0 {
		return
	}
	if adkActions.StateDelta == nil {
		adkActions.StateDelta = make(map[string]any)
	}
	for k, v := range src {
		adkActions.StateDelta[k] = v
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// Result Conversion
//
// The foundation agent.Result.Content field is []any. Tools produce content
// in three shapes:
//
//   - JSONResult(data)  → Content = [jsonString]   (structured results)
//   - TextResult(text)  → Content = [plainString]  (calculator, etc.)
//   - ErrorResult(err)  → Content = nil, Error = err
//   - Multiple items    → Content = [a, b, ...]
//
// The map we return becomes FunctionResponse.Response. events.go marshals
// that map to produce ToolResultAdded.Result (the string the frontend parses).
// Therefore: the map must be flat — keys from the tool output sit at the top
// level so the frontend can access them directly (e.g. parsedResult.audio_url).
// ──────────────────────────────────────────────────────────────────────────────

const errorKey = "error" // Sentinel key for error results.

// resultToMap converts an agent.Result to the flat map[string]any that
// ADK-Go FunctionResponse expects.
func resultToMap(r agent.Result) map[string]any {
	out := make(map[string]any)

	// 1. Carry over explicit error from the Result struct.
	if r.Error != nil {
		out[errorKey] = r.Error.Error()
	}

	// 2. Convert content.
	switch len(r.Content) {
	case 0:
		// No content — error-only result or empty success.

	case 1:
		mergeSingleContent(out, r.Content[0])

	default:
		// Multiple content items — store as array under "results".
		out["results"] = r.Content
	}

	// 3. Merge metadata (lowest priority — doesn't overwrite content keys).
	for k, v := range r.Metadata {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}

	return out
}

// mergeSingleContent handles the single-content-item case.
// If the content is a JSON string, it is parsed and merged flat into out.
// This is the critical path for tools that return JSONResult(data):
// their output arrives as a JSON-encoded string and must be unpacked so the
// frontend sees the structured fields at the top level.
func mergeSingleContent(out map[string]any, item any) {
	s, ok := item.(string)
	if !ok {
		// Non-string content (rare) — store as-is under "result".
		out["result"] = item
		return
	}

	// Try to parse as JSON object and merge keys flat.
	var parsed map[string]any
	if json.Unmarshal([]byte(s), &parsed) == nil && len(parsed) > 0 {
		for k, v := range parsed {
			out[k] = v
		}
		return
	}

	// Not a JSON object — use as plain string.
	out["result"] = s
}

// ──────────────────────────────────────────────────────────────────────────────
// Schema Conversion: agent.Schema → jsonschema.Schema
// ──────────────────────────────────────────────────────────────────────────────

// agentSchemaToJSONSchema converts a foundation agent.Schema to the standard
// JSON Schema representation used by the adk-go framework.
func agentSchemaToJSONSchema(s *agent.Schema) *jsonschema.Schema {
	if s == nil {
		return nil
	}

	js := &jsonschema.Schema{}

	// Nullable fields use JSON Schema's multi-type array: ["string", "null"].
	// This matches the JSON Schema draft 2020-12 pattern and is understood
	// by ADK-Go's functiontool when building the LLM function declaration.
	if s.Nullable && s.Type != "" {
		js.Types = []string{string(s.Type), "null"}
	} else {
		js.Type = string(s.Type)
	}

	if s.Description != "" {
		js.Description = s.Description
	}
	if len(s.Properties) > 0 {
		props := make(map[string]*jsonschema.Schema, len(s.Properties))
		for k, v := range s.Properties {
			props[k] = agentSchemaToJSONSchema(v)
		}
		js.Properties = props
	}
	if len(s.Required) > 0 {
		js.Required = s.Required
	}
	if s.Items != nil {
		js.Items = agentSchemaToJSONSchema(s.Items)
	}
	if len(s.Enum) > 0 {
		enums := make([]any, len(s.Enum))
		for i, e := range s.Enum {
			enums[i] = e
		}
		js.Enum = enums
	}

	return js
}
