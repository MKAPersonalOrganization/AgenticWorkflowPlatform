package adkgo

import (
	"aicompass/modules/chat/thread"
	"encoding/json"
	"strings"

	"google.golang.org/adk/session"
	adktool "google.golang.org/adk/tool"
)

// translateFunctionResponses walks the event's FunctionResponse parts
// and emits one ToolResultAdded per non-confirmation response. The
// convention for success vs error shape is established in tools.go
// resultToMap() — see isErrorResponse.
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
