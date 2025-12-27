package agui_test

import (
	"encoding/json"
	"fmt"
	"time"

	"school-with-ai/internal/agui"
)

// Example demonstrates how to create and use tool call events
func Example_toolCallEvents() {
	// Generate IDs
	toolCallID := agui.GenerateToolCallID()
	messageID := agui.GenerateMessageID()

	// 1. Create TOOL_CALL_START event
	startEvent := agui.ToolCallStartEvent{
		Type:            "TOOL_CALL_START",
		ToolCallID:      toolCallID,
		ToolCallName:    "get_weather",
		ParentMessageID: messageID,
		Timestamp:       time.Now().UnixMilli(),
	}

	// Serialize to JSON (as would be sent via SSE)
	startJSON, _ := json.Marshal(startEvent)
	fmt.Printf("Start: %s\n", startJSON)

	// 2. Create TOOL_CALL_ARGS events (streaming arguments)
	argsEvent1 := agui.ToolCallArgsEvent{
		Type:       "TOOL_CALL_ARGS",
		ToolCallID: toolCallID,
		Delta:      `{"location":"`,
		Timestamp:  time.Now().UnixMilli(),
	}
	args1JSON, _ := json.Marshal(argsEvent1)
	fmt.Printf("Args 1: %s\n", args1JSON)

	argsEvent2 := agui.ToolCallArgsEvent{
		Type:       "TOOL_CALL_ARGS",
		ToolCallID: toolCallID,
		Delta:      `San Francisco, CA"}`,
		Timestamp:  time.Now().UnixMilli(),
	}
	args2JSON, _ := json.Marshal(argsEvent2)
	fmt.Printf("Args 2: %s\n", args2JSON)

	// 3. Create TOOL_CALL_END event
	endEvent := agui.ToolCallEndEvent{
		Type:       "TOOL_CALL_END",
		ToolCallID: toolCallID,
		Timestamp:  time.Now().UnixMilli(),
	}
	endJSON, _ := json.Marshal(endEvent)
	fmt.Printf("End: %s\n", endJSON)

	// 4. Create TOOL_CALL_RESULT event (after tool execution)
	resultEvent := agui.ToolCallResultEvent{
		Type:       "TOOL_CALL_RESULT",
		MessageID:  agui.GenerateMessageID(),
		ToolCallID: toolCallID,
		Content:    `{"temperature":72,"unit":"fahrenheit","condition":"sunny"}`,
		Role:       "tool",
		Timestamp:  time.Now().UnixMilli(),
	}
	resultJSON, _ := json.Marshal(resultEvent)
	fmt.Printf("Result: %s\n", resultJSON)

	// Output demonstrates the AG-UI protocol tool call event sequence
}

// Example_minimalToolCallResult shows that role can be omitted (defaults to "tool")
func Example_minimalToolCallResult() {
	resultEvent := agui.ToolCallResultEvent{
		Type:       "TOOL_CALL_RESULT",
		MessageID:  "msg_123",
		ToolCallID: "tool_456",
		Content:    `{"result":"success"}`,
		// Role omitted - will be omitted in JSON due to omitempty
	}

	resultJSON, _ := json.Marshal(resultEvent)
	fmt.Printf("%s\n", resultJSON)

	// Output shows role field is not present when empty
	// Output: {"type":"TOOL_CALL_RESULT","messageId":"msg_123","toolCallId":"tool_456","content":"{\"result\":\"success\"}"}
}
