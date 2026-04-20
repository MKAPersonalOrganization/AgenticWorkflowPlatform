package adkgo

import (
	"encoding/json"
	"strings"
)

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

// extractConfirmationInfo parses the originalFunctionCall name, ID,
// toolConfirmation hint, and optional custom input payload from an
// adk_request_confirmation FunctionCall's JSON-encoded arguments.
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
