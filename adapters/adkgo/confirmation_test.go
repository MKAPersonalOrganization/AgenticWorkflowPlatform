package adkgo

import (
	"aicompass/modules/chat/thread"
	"context"
	"encoding/json"
	"testing"
	"time"

	"google.golang.org/adk/model"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool/toolconfirmation"
	"google.golang.org/genai"
)

// ──────────────────────────────────────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────────────────────────────────────

const (
	testThreadID = "thread-confirm-test"
	testRunID    = "run-001"
	testAgent    = "test-agent"

	// IDs used throughout the workflow:
	// - The LLM generates a FunctionCall to invoke the original tool.
	originalToolCallID = "call-original-abc"
	// - ADK wraps this in an adk_request_confirmation FunctionCall with a new ID.
	confirmationCallID = "call-confirm-xyz"
)

func testEventContext() EventContext {
	return EventContext{
		ThreadID:  testThreadID,
		RunID:     testRunID,
		AgentName: testAgent,
	}
}

// buildConfirmationFunctionCallEvent constructs the session.Event that ADK
// generates via generateRequestConfirmationEvent. It contains an
// adk_request_confirmation FunctionCall wrapping the original tool call.
func buildConfirmationFunctionCallEvent() *session.Event {
	originalCall := map[string]any{
		"id":   originalToolCallID,
		"name": "delete_file",
		"args": map[string]any{"path": "/tmp/important.txt"},
	}
	confirmation := map[string]any{
		"hint":      "Tool 'delete_file' requires your confirmation before executing.",
		"confirmed": false,
		"payload":   "delete_file",
	}
	args := map[string]any{
		"originalFunctionCall": originalCall,
		"toolConfirmation":     confirmation,
	}

	evt := session.NewEvent("inv-1")
	evt.ID = "evt-confirm"
	evt.Author = testAgent
	evt.Timestamp = time.Now()
	evt.LLMResponse = model.LLMResponse{
		Content: &genai.Content{
			Role: "model",
			Parts: []*genai.Part{{
				FunctionCall: &genai.FunctionCall{
					ID:   confirmationCallID,
					Name: toolconfirmation.FunctionCallName,
					Args: args,
				},
			}},
		},
	}
	evt.LongRunningToolIDs = []string{confirmationCallID}
	return evt
}

// buildFunctionResponseEvent constructs the session.Event that ADK generates
// when the original tool runs and requests confirmation. This event carries
// RequestedToolConfirmations keyed by the ORIGINAL tool call ID.
func buildFunctionResponseEvent() *session.Event {
	evt := session.NewEvent("inv-1")
	evt.ID = "evt-func-resp"
	evt.Author = testAgent
	evt.Timestamp = time.Now()
	evt.Actions.RequestedToolConfirmations = map[string]toolconfirmation.ToolConfirmation{
		originalToolCallID: {
			Hint:      "Tool 'delete_file' requires your confirmation.",
			Confirmed: false,
			Payload:   "delete_file",
		},
	}
	evt.Actions.SkipSummarization = true
	evt.LLMResponse = model.LLMResponse{
		Content: &genai.Content{
			Role: "user", // function responses use "user" role in ADK
			Parts: []*genai.Part{{
				FunctionResponse: &genai.FunctionResponse{
					ID:       originalToolCallID,
					Name:     "delete_file",
					Response: map[string]any{"status": "confirmation_required"},
				},
			}},
		},
	}
	return evt
}

// ──────────────────────────────────────────────────────────────────────────────
// 1. Event translation: adk_request_confirmation → ToolNeedsInput
// ──────────────────────────────────────────────────────────────────────────────

func TestTranslateEvent_ConfirmationFunctionCall_EmitsToolNeedsConfirmation(t *testing.T) {
	evt := buildConfirmationFunctionCallEvent()
	ctx := testEventContext()

	domainEvents := TranslateEvent(evt, ctx)

	// Should contain AssistantMessageAdded + ToolNeedsConfirmation (no ToolStart).
	var needsConfirm *thread.ToolNeedsConfirmation
	var assistantMsg *thread.AssistantMessageAdded
	var toolStarts []*thread.ToolStart

	for _, de := range domainEvents {
		switch e := de.(type) {
		case *thread.ToolNeedsConfirmation:
			needsConfirm = e
		case *thread.AssistantMessageAdded:
			assistantMsg = e
		case *thread.ToolStart:
			toolStarts = append(toolStarts, e)
		}
	}

	// ToolNeedsConfirmation must be present with the CONFIRMATION call ID and original ID.
	if needsConfirm == nil {
		t.Fatal("expected ToolNeedsConfirmation event, got none")
	}
	if needsConfirm.ToolCallID != confirmationCallID {
		t.Errorf("ToolNeedsConfirmation.ToolCallID = %q, want %q", needsConfirm.ToolCallID, confirmationCallID)
	}
	if needsConfirm.OriginalToolCallID != originalToolCallID {
		t.Errorf("ToolNeedsConfirmation.OriginalToolCallID = %q, want %q", needsConfirm.OriginalToolCallID, originalToolCallID)
	}
	if needsConfirm.ToolName != "delete_file" {
		t.Errorf("ToolNeedsConfirmation.ToolName = %q, want %q", needsConfirm.ToolName, "delete_file")
	}
	if needsConfirm.Question == "" {
		t.Error("ToolNeedsConfirmation.Question is empty")
	}
	if needsConfirm.AgentName != testAgent {
		t.Errorf("ToolNeedsConfirmation.AgentName = %q, want %q", needsConfirm.AgentName, testAgent)
	}

	// AssistantMessageAdded should include the adk_request_confirmation tool call.
	if assistantMsg == nil {
		t.Fatal("expected AssistantMessageAdded event, got none")
	}
	if len(assistantMsg.ToolCalls) != 1 {
		t.Fatalf("AssistantMessageAdded has %d tool calls, want 1", len(assistantMsg.ToolCalls))
	}
	if assistantMsg.ToolCalls[0].Name != toolconfirmation.FunctionCallName {
		t.Errorf("tool call name = %q, want %q", assistantMsg.ToolCalls[0].Name, toolconfirmation.FunctionCallName)
	}

	// No ToolStart should be emitted for adk_request_confirmation.
	if len(toolStarts) > 0 {
		t.Errorf("expected no ToolStart for adk_request_confirmation, got %d", len(toolStarts))
	}
}

func TestTranslateEvent_RegularToolCall_StillEmitsToolStart(t *testing.T) {
	evt := session.NewEvent("inv-1")
	evt.ID = "evt-regular"
	evt.Author = testAgent
	evt.Timestamp = time.Now()
	evt.LLMResponse = model.LLMResponse{
		Content: &genai.Content{
			Role: "model",
			Parts: []*genai.Part{{
				FunctionCall: &genai.FunctionCall{
					ID:   "call-regular",
					Name: "search_web",
					Args: map[string]any{"query": "hello"},
				},
			}},
		},
	}

	domainEvents := TranslateEvent(evt, testEventContext())

	var hasToolStart bool
	var hasToolNeedsInput bool
	for _, de := range domainEvents {
		switch de.(type) {
		case *thread.ToolStart:
			hasToolStart = true
		case *thread.ToolNeedsInput:
			hasToolNeedsInput = true
		}
	}

	if !hasToolStart {
		t.Error("expected ToolStart for regular tool call")
	}
	if hasToolNeedsInput {
		t.Error("unexpected ToolNeedsInput for regular tool call")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// 2. translateToolConfirmations logic is no longer used
// Handled by verification in TestTranslateEvent_ConfirmationFunctionCall_EmitsToolNeedsConfirmation

// ──────────────────────────────────────────────────────────────────────────────
// 3. extractConfirmationInfo
// ──────────────────────────────────────────────────────────────────────────────

func TestExtractConfirmationInfo(t *testing.T) {
	args := map[string]any{
		"originalFunctionCall": map[string]any{
			"id":   "call-abc",
			"name": "delete_file",
			"args": map[string]any{"path": "/tmp/file.txt"},
		},
		"toolConfirmation": map[string]any{
			"hint":      "Confirm delete?",
			"confirmed": false,
			"payload":   "delete_file",
		},
	}
	argsJSON, _ := json.Marshal(args)

	info := extractConfirmationInfo(string(argsJSON))
	if info.originalID != "call-abc" {
		t.Errorf("originalID = %q, want %q", info.originalID, "call-abc")
	}
	if info.originalName != "delete_file" {
		t.Errorf("originalName = %q, want %q", info.originalName, "delete_file")
	}
	if info.hint != "Confirm delete?" {
		t.Errorf("hint = %q, want %q", info.hint, "Confirm delete?")
	}
	if info.inputSchema != nil {
		t.Errorf("inputSchema = %v, want nil for regular confirmation", info.inputSchema)
	}
}

func TestExtractConfirmationInfo_CustomInput(t *testing.T) {
	args := map[string]any{
		"originalFunctionCall": map[string]any{
			"id":   "call-xyz",
			"name": "file_search",
		},
		"toolConfirmation": map[string]any{
			"hint":      "What file should I search for?",
			"confirmed": false,
			"payload": map[string]any{
				"type":   "custom_input",
				"schema": map[string]any{"type": "string"},
			},
		},
	}
	argsJSON, _ := json.Marshal(args)

	info := extractConfirmationInfo(string(argsJSON))
	if info.originalID != "call-xyz" {
		t.Errorf("originalID = %q, want %q", info.originalID, "call-xyz")
	}
	if info.originalName != "file_search" {
		t.Errorf("originalName = %q, want %q", info.originalName, "file_search")
	}
	if info.hint != "What file should I search for?" {
		t.Errorf("hint = %q, want %q", info.hint, "What file should I search for?")
	}
	if info.inputSchema == nil {
		t.Fatal("inputSchema = nil, want non-nil for custom input")
	}
	if info.inputSchema["type"] != "string" {
		t.Errorf("inputSchema[type] = %v, want %q", info.inputSchema["type"], "string")
	}
}

func TestExtractConfirmationInfo_InvalidJSON(t *testing.T) {
	info := extractConfirmationInfo("not json")
	if info.originalID != "" || info.originalName != "" || info.hint != "" {
		t.Errorf("expected empty strings for invalid JSON, got id=%q name=%q hint=%q", info.originalID, info.originalName, info.hint)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// 4. buildConfirmationResponse
// ──────────────────────────────────────────────────────────────────────────────

func TestBuildConfirmationResponse(t *testing.T) {
	content := buildConfirmationResponse("call-confirm-xyz", "call-original-abc")

	if content.Role != "user" {
		t.Errorf("role = %q, want %q", content.Role, "user")
	}
	if len(content.Parts) != 1 {
		t.Fatalf("parts count = %d, want 1", len(content.Parts))
	}

	fr := content.Parts[0].FunctionResponse
	if fr == nil {
		t.Fatal("expected FunctionResponse, got nil")
	}
	if fr.Name != toolconfirmation.FunctionCallName {
		t.Errorf("FunctionResponse.Name = %q, want %q", fr.Name, toolconfirmation.FunctionCallName)
	}
	if fr.ID != "call-confirm-xyz" {
		t.Errorf("FunctionResponse.ID = %q, want %q", fr.ID, "call-confirm-xyz")
	}
	confirmed, ok := fr.Response["confirmed"].(bool)
	if !ok || !confirmed {
		t.Errorf("FunctionResponse.Response[confirmed] = %v, want true", fr.Response["confirmed"])
	}
	// Verify original tool call ID is included for traceability
	origID, ok := fr.Response["original_tool_call_id"].(string)
	if !ok || origID != "call-original-abc" {
		t.Errorf("FunctionResponse.Response[original_tool_call_id] = %v, want %q", fr.Response["original_tool_call_id"], "call-original-abc")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// 5. Session reconstruction: confirmation FunctionResponses are fixed
// ──────────────────────────────────────────────────────────────────────────────

func TestGetConfirmationWrapperID(t *testing.T) {
	argsJSON, _ := json.Marshal(map[string]any{
		"originalFunctionCall": map[string]any{"id": originalToolCallID, "name": "delete_file"},
		"toolConfirmation":     map[string]any{"hint": "confirm?"},
	})
	msgs := []thread.Message{
		{ID: "msg-1", Role: "user", Content: "delete the file"},
		{ID: "msg-2", Role: "assistant", ToolCalls: []thread.ToolCall{
			{ID: confirmationCallID, Name: toolconfirmation.FunctionCallName, Arguments: string(argsJSON)},
		}},
	}

	// Lookup with original tool call ID should return wrapper ID
	wrapperID := GetConfirmationWrapperID(msgs, originalToolCallID)
	if wrapperID != confirmationCallID {
		t.Errorf("GetConfirmationWrapperID() = %q, want %q", wrapperID, confirmationCallID)
	}

	// Lookup with non-existent ID should return ""
	wrapperID = GetConfirmationWrapperID(msgs, "non-existent")
	if wrapperID != "" {
		t.Errorf("GetConfirmationWrapperID(non-existent) = %q, want empty string", wrapperID)
	}
}

func TestMessageToSessionEvent_ConfirmationToolResult(t *testing.T) {
	mappings := confirmationMappings{
		wrapperIDs:     map[string]bool{confirmationCallID: true},
		originalToWrap: map[string]string{originalToolCallID: confirmationCallID},
	}

	msg := thread.Message{
		ID:                 "msg-response",
		Role:               "tool",
		Content:            "yes",
		ToolCallID:         confirmationCallID,
		ToolName:           "delete_file",
		Timestamp:          time.Now(),
		OriginalToolCallID: originalToolCallID,
	}

	evt := messageToSessionEvent(msg, mappings)
	if evt == nil {
		t.Fatal("expected session event, got nil")
	}

	// The event must be authored by "user" (ADK convention for function responses).
	if evt.Author != "user" {
		t.Errorf("Author = %q, want %q", evt.Author, "user")
	}

	// Extract the FunctionResponse part.
	parts := evt.Content.Parts
	if len(parts) != 1 {
		t.Fatalf("parts count = %d, want 1", len(parts))
	}
	fr := parts[0].FunctionResponse
	if fr == nil {
		t.Fatal("expected FunctionResponse part, got nil")
	}

	// Name must be adk_request_confirmation, NOT the original tool name.
	if fr.Name != toolconfirmation.FunctionCallName {
		t.Errorf("FunctionResponse.Name = %q, want %q", fr.Name, toolconfirmation.FunctionCallName)
	}
	if fr.ID != confirmationCallID {
		t.Errorf("FunctionResponse.ID = %q, want %q", fr.ID, confirmationCallID)
	}
	// Response must be {"confirmed": true}.
	confirmed, ok := fr.Response["confirmed"].(bool)
	if !ok || !confirmed {
		t.Errorf("FunctionResponse.Response = %v, want {confirmed: true}", fr.Response)
	}
}

func TestMessageToSessionEvent_RegularToolResult_Unchanged(t *testing.T) {
	mappings := confirmationMappings{
		wrapperIDs:     map[string]bool{confirmationCallID: true},
		originalToWrap: map[string]string{},
	}

	msg := thread.Message{
		ID:         "msg-regular",
		Role:       "tool",
		Content:    `{"result": "done"}`,
		ToolCallID: "call-regular-tool",
		ToolName:   "search_web",
		Timestamp:  time.Now(),
	}

	evt := messageToSessionEvent(msg, mappings)
	if evt == nil {
		t.Fatal("expected session event, got nil")
	}

	fr := evt.Content.Parts[0].FunctionResponse
	if fr.Name != "search_web" {
		t.Errorf("FunctionResponse.Name = %q, want %q", fr.Name, "search_web")
	}
	// Should preserve original JSON response.
	if _, ok := fr.Response["result"]; !ok {
		t.Error("expected original result to be preserved")
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// 6. Full E2E workflow: translate → persist → rebuild session
//
// Simulates the complete lifecycle:
//   Phase 1: Agent runs, tool needs confirmation → events translated
//   Phase 2: Session rebuilt from thread → confirmation response is correct
//   Phase 3: Confirmation response content is well-formed for ADK
// ──────────────────────────────────────────────────────────────────────────────

func TestConfirmationWorkflow_E2E(t *testing.T) {
	ctx := context.Background()
	evtCtx := testEventContext()

	// ── Phase 1: Event translation ────────────────────────────────────────

	// Step 1a: The function response event (tool returned confirmation_required).
	// This should NOT produce ToolNeedsInput - ADK's native flow handles it via
	// the adk_request_confirmation FunctionCall event instead.
	funcRespEvt := buildFunctionResponseEvent()
	funcRespDomainEvents := TranslateEvent(funcRespEvt, evtCtx)

	for _, de := range funcRespDomainEvents {
		if _, ok := de.(*thread.ToolNeedsInput); ok {
			t.Error("function response event should NOT produce ToolNeedsInput (handled by FunctionCall path)")
		}
	}

	// Step 1b: The adk_request_confirmation FunctionCall event - this is the one that should emit ToolNeedsConfirmation.
	confirmEvt := buildConfirmationFunctionCallEvent()
	confirmDomainEvents := TranslateEvent(confirmEvt, evtCtx)

	var needsConfirm *thread.ToolNeedsConfirmation
	var assistantMsg *thread.AssistantMessageAdded
	for _, de := range confirmDomainEvents {
		switch e := de.(type) {
		case *thread.ToolNeedsConfirmation:
			needsConfirm = e
		case *thread.AssistantMessageAdded:
			assistantMsg = e
		}
	}
	if needsConfirm == nil {
		t.Fatal("Phase 1: expected ToolNeedsConfirmation from confirmation event")
	}
	if needsConfirm.ToolCallID != confirmationCallID {
		t.Fatalf("Phase 1: ToolNeedsConfirmation.ToolCallID = %q, want %q", needsConfirm.ToolCallID, confirmationCallID)
	}
	if needsConfirm.OriginalToolCallID != originalToolCallID {
		t.Fatalf("Phase 1: ToolNeedsConfirmation.OriginalToolCallID = %q, want %q", needsConfirm.OriginalToolCallID, originalToolCallID)
	}

	// ── Phase 2: Build thread aggregate mimicking what persistence would create ──

	argsJSON := assistantMsg.ToolCalls[0].Arguments

	agg := thread.NewThreadAggregate(testThreadID)
	agg.Messages = []thread.Message{
		// User's original message.
		{ID: "msg-user", Role: "user", Content: "delete /tmp/important.txt", Timestamp: time.Now()},
		// Assistant message with the adk_request_confirmation tool call.
		{
			ID:   confirmEvt.ID,
			Role: "assistant",
			ToolCalls: []thread.ToolCall{{
				ID:        confirmationCallID,
				Type:      "function",
				Name:      toolconfirmation.FunctionCallName,
				Arguments: argsJSON,
			}},
			AgentName: testAgent,
			Timestamp: time.Now(),
		},
		// User's confirmation response (created by InputProvided.Apply()).
		{
			ID:                 "msg-input-provided",
			Role:               "tool",
			Content:            "yes, go ahead",
			ToolCallID:         confirmationCallID,
			ToolName:           "delete_file", // stored as original tool name
			AgentName:          testAgent,
			Timestamp:          time.Now(),
			OriginalToolCallID: originalToolCallID, // preserves original ID for session rebuild
		},
	}

	// ── Phase 3: Rebuild session and verify ──────────────────────────────

	svc := session.InMemoryService()
	sess, err := BuildSessionFromThread(ctx, svc, agg, "user-1")
	if err != nil {
		t.Fatalf("Phase 3: BuildSessionFromThread failed: %v", err)
	}

	// Walk the session events and find the confirmation FunctionResponse.
	var confirmationResp *genai.FunctionResponse
	for evt := range sess.Events().All() {
		if evt.Content == nil {
			continue
		}
		for _, p := range evt.Content.Parts {
			if p.FunctionResponse != nil && p.FunctionResponse.ID == confirmationCallID {
				confirmationResp = p.FunctionResponse
			}
		}
	}

	if confirmationResp == nil {
		t.Fatal("Phase 3: no FunctionResponse with confirmation call ID found in session")
	}
	if confirmationResp.Name != toolconfirmation.FunctionCallName {
		t.Errorf("Phase 3: FunctionResponse.Name = %q, want %q", confirmationResp.Name, toolconfirmation.FunctionCallName)
	}
	confirmed, ok := confirmationResp.Response["confirmed"].(bool)
	if !ok || !confirmed {
		t.Errorf("Phase 3: FunctionResponse.Response = %v, want {confirmed: true}", confirmationResp.Response)
	}

	// ── Phase 4: Verify confirmation content for ADK runner ──────────────

	resumeContent := buildConfirmationResponse(confirmationCallID, originalToolCallID)
	resumeFR := resumeContent.Parts[0].FunctionResponse

	if resumeFR.Name != toolconfirmation.FunctionCallName {
		t.Errorf("Phase 4: resume Name = %q, want %q", resumeFR.Name, toolconfirmation.FunctionCallName)
	}
	if resumeFR.ID != confirmationCallID {
		t.Errorf("Phase 4: resume ID = %q, want %q", resumeFR.ID, confirmationCallID)
	}

	// Verify the session now has the adk_request_confirmation FunctionCall
	// that the processor will match against.
	var foundConfirmationCall bool
	for evt := range sess.Events().All() {
		if evt.Content == nil {
			continue
		}
		for _, p := range evt.Content.Parts {
			if p.FunctionCall != nil &&
				p.FunctionCall.Name == toolconfirmation.FunctionCallName &&
				p.FunctionCall.ID == confirmationCallID {
				foundConfirmationCall = true
			}
		}
	}
	if !foundConfirmationCall {
		t.Error("Phase 4: session missing adk_request_confirmation FunctionCall — processor won't find a match")
	}
}
