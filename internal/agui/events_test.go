package agui

import (
	"encoding/json"
	"testing"
)

// Helper function to create pointer to string
func stringPtr(s string) *string {
	return &s
}

// Helper function to create pointer to int64
func int64Ptr(i int64) *int64 {
	return &i
}

// TestToolCallStartEvent verifies TOOL_CALL_START event structure
func TestToolCallStartEvent(t *testing.T) {
	parentMsgID := "msg_456"
	timestamp := int64(1234567890)

	event := ToolCallStartEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_START",
			Timestamp: &timestamp,
		},
		ToolCallID:      "tool_123",
		ToolCallName:    "get_weather",
		ParentMessageID: &parentMsgID,
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
	timestamp := int64(1234567890)

	event := ToolCallArgsEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_ARGS",
			Timestamp: &timestamp,
		},
		ToolCallID: "tool_123",
		Delta:      `{"location":"`,
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
	timestamp := int64(1234567890)

	event := ToolCallEndEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_END",
			Timestamp: &timestamp,
		},
		ToolCallID: "tool_123",
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
	role := "tool"
	timestamp := int64(1234567890)

	event := ToolCallResultEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_RESULT",
			Timestamp: &timestamp,
		},
		MessageID:  "msg_789",
		ToolCallID: "tool_123",
		Content:    `{"temperature":72,"condition":"sunny"}`,
		Role:       &role,
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
	timestamp := int64(1234567890)

	event := ToolCallResultEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_RESULT",
			Timestamp: &timestamp,
		},
		MessageID:  "msg_789",
		ToolCallID: "tool_123",
		Content:    `{"result":"success"}`,
		// Role omitted - should use default "tool" or be omitted
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
	parentMsgID := messageID
	timestamp := int64(1234567890)

	// Start event
	startEvent := ToolCallStartEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_START",
			Timestamp: int64Ptr(timestamp),
		},
		ToolCallID:      toolCallID,
		ToolCallName:    "get_weather",
		ParentMessageID: &parentMsgID,
	}

	// Args events (streaming)
	argsEvent1 := ToolCallArgsEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_ARGS",
			Timestamp: int64Ptr(timestamp + 1),
		},
		ToolCallID: toolCallID,
		Delta:      `{"location":"`,
	}

	argsEvent2 := ToolCallArgsEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_ARGS",
			Timestamp: int64Ptr(timestamp + 2),
		},
		ToolCallID: toolCallID,
		Delta:      `San Francisco"}`,
	}

	// End event
	endEvent := ToolCallEndEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_END",
			Timestamp: int64Ptr(timestamp + 3),
		},
		ToolCallID: toolCallID,
	}

	// Result event
	role := "tool"
	resultEvent := ToolCallResultEvent{
		BaseEvent: BaseEvent{
			Type:      "TOOL_CALL_RESULT",
			Timestamp: int64Ptr(timestamp + 4),
		},
		MessageID:  GenerateMessageID(),
		ToolCallID: toolCallID,
		Content:    `{"temperature":72,"condition":"sunny"}`,
		Role:       &role,
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
