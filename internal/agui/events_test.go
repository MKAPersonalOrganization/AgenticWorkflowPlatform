package agui

import (
	"encoding/json"
	"testing"
	"time"
)

// TestToolCallStartEvent verifies TOOL_CALL_START event structure
func TestToolCallStartEvent(t *testing.T) {
	event := ToolCallStartEvent{
		Type:            "TOOL_CALL_START",
		ToolCallID:      "tool_123",
		ToolCallName:    "get_weather",
		ParentMessageID: "msg_456",
		Timestamp:       time.Now().UnixMilli(),
	}

	// Verify it serializes to JSON correctly
	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal ToolCallStartEvent: %v", err)
	}

	// Verify JSON structure matches AG-UI spec
	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check required fields
	if decoded["type"] != "TOOL_CALL_START" {
		t.Errorf("Expected type TOOL_CALL_START, got %v", decoded["type"])
	}
	if decoded["toolCallId"] != "tool_123" {
		t.Errorf("Expected toolCallId tool_123, got %v", decoded["toolCallId"])
	}
	if decoded["toolCallName"] != "get_weather" {
		t.Errorf("Expected toolCallName get_weather, got %v", decoded["toolCallName"])
	}
	if decoded["parentMessageId"] != "msg_456" {
		t.Errorf("Expected parentMessageId msg_456, got %v", decoded["parentMessageId"])
	}
}

// TestToolCallArgsEvent verifies TOOL_CALL_ARGS event structure
func TestToolCallArgsEvent(t *testing.T) {
	event := ToolCallArgsEvent{
		Type:       "TOOL_CALL_ARGS",
		ToolCallID: "tool_123",
		Delta:      `{"location":"`,
		Timestamp:  time.Now().UnixMilli(),
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal ToolCallArgsEvent: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded["type"] != "TOOL_CALL_ARGS" {
		t.Errorf("Expected type TOOL_CALL_ARGS, got %v", decoded["type"])
	}
	if decoded["toolCallId"] != "tool_123" {
		t.Errorf("Expected toolCallId tool_123, got %v", decoded["toolCallId"])
	}
	if decoded["delta"] != `{"location":"` {
		t.Errorf("Expected delta, got %v", decoded["delta"])
	}
}

// TestToolCallEndEvent verifies TOOL_CALL_END event structure
func TestToolCallEndEvent(t *testing.T) {
	event := ToolCallEndEvent{
		Type:       "TOOL_CALL_END",
		ToolCallID: "tool_123",
		Timestamp:  time.Now().UnixMilli(),
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal ToolCallEndEvent: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded["type"] != "TOOL_CALL_END" {
		t.Errorf("Expected type TOOL_CALL_END, got %v", decoded["type"])
	}
	if decoded["toolCallId"] != "tool_123" {
		t.Errorf("Expected toolCallId tool_123, got %v", decoded["toolCallId"])
	}
}

// TestToolCallResultEvent verifies TOOL_CALL_RESULT event structure
func TestToolCallResultEvent(t *testing.T) {
	event := ToolCallResultEvent{
		Type:       "TOOL_CALL_RESULT",
		MessageID:  "msg_789",
		ToolCallID: "tool_123",
		Content:    `{"temperature":72,"condition":"sunny"}`,
		Role:       "tool",
		Timestamp:  time.Now().UnixMilli(),
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal ToolCallResultEvent: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded["type"] != "TOOL_CALL_RESULT" {
		t.Errorf("Expected type TOOL_CALL_RESULT, got %v", decoded["type"])
	}
	if decoded["messageId"] != "msg_789" {
		t.Errorf("Expected messageId msg_789, got %v", decoded["messageId"])
	}
	if decoded["toolCallId"] != "tool_123" {
		t.Errorf("Expected toolCallId tool_123, got %v", decoded["toolCallId"])
	}
	if decoded["role"] != "tool" {
		t.Errorf("Expected role tool, got %v", decoded["role"])
	}
}

// TestToolCallResultEventOmitsEmptyRole verifies role is omitted when empty
func TestToolCallResultEventOmitsEmptyRole(t *testing.T) {
	event := ToolCallResultEvent{
		Type:       "TOOL_CALL_RESULT",
		MessageID:  "msg_789",
		ToolCallID: "tool_123",
		Content:    `{"result":"success"}`,
		// Role omitted - should use default "tool" or be omitted
		Timestamp: time.Now().UnixMilli(),
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal ToolCallResultEvent: %v", err)
	}

	// Verify role is omitted when empty
	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Role should not be present in JSON when empty (due to omitempty)
	if _, hasRole := decoded["role"]; hasRole {
		t.Errorf("Expected role to be omitted when empty, but it was present")
	}
}

// TestGenerateToolCallID verifies tool call ID generation
func TestGenerateToolCallID(t *testing.T) {
	id1 := GenerateToolCallID()
	id2 := GenerateToolCallID()

	// IDs should start with "tool_"
	if len(id1) < 6 || id1[:5] != "tool_" {
		t.Errorf("Expected tool call ID to start with 'tool_', got %s", id1)
	}

	// IDs should be unique
	if id1 == id2 {
		t.Errorf("Expected unique tool call IDs, got duplicates: %s", id1)
	}
}

// TestToolCallEventSequence verifies a complete tool call event sequence
func TestToolCallEventSequence(t *testing.T) {
	toolCallID := GenerateToolCallID()
	messageID := GenerateMessageID()
	timestamp := time.Now().UnixMilli()

	// Start event
	startEvent := ToolCallStartEvent{
		Type:            "TOOL_CALL_START",
		ToolCallID:      toolCallID,
		ToolCallName:    "get_weather",
		ParentMessageID: messageID,
		Timestamp:       timestamp,
	}

	// Args events (streaming)
	argsEvent1 := ToolCallArgsEvent{
		Type:       "TOOL_CALL_ARGS",
		ToolCallID: toolCallID,
		Delta:      `{"location":"`,
		Timestamp:  timestamp + 1,
	}

	argsEvent2 := ToolCallArgsEvent{
		Type:       "TOOL_CALL_ARGS",
		ToolCallID: toolCallID,
		Delta:      `San Francisco"}`,
		Timestamp:  timestamp + 2,
	}

	// End event
	endEvent := ToolCallEndEvent{
		Type:       "TOOL_CALL_END",
		ToolCallID: toolCallID,
		Timestamp:  timestamp + 3,
	}

	// Result event
	resultEvent := ToolCallResultEvent{
		Type:       "TOOL_CALL_RESULT",
		MessageID:  GenerateMessageID(),
		ToolCallID: toolCallID,
		Content:    `{"temperature":72,"condition":"sunny"}`,
		Role:       "tool",
		Timestamp:  timestamp + 4,
	}

	// Verify all events can be marshaled
	events := []interface{}{startEvent, argsEvent1, argsEvent2, endEvent, resultEvent}
	for i, event := range events {
		if _, err := json.Marshal(event); err != nil {
			t.Errorf("Failed to marshal event %d: %v", i, err)
		}
	}

	// Verify all events use the same toolCallId
	if argsEvent1.ToolCallID != toolCallID {
		t.Errorf("Args event 1 has wrong toolCallId")
	}
	if argsEvent2.ToolCallID != toolCallID {
		t.Errorf("Args event 2 has wrong toolCallId")
	}
	if endEvent.ToolCallID != toolCallID {
		t.Errorf("End event has wrong toolCallId")
	}
	if resultEvent.ToolCallID != toolCallID {
		t.Errorf("Result event has wrong toolCallId")
	}
}
