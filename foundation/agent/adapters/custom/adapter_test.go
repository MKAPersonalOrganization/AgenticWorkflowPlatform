package custom

import (
	"testing"
	"time"

	"school-with-ai/domain"
	"school-with-ai/foundation/agent"
)

func TestConvertAgentMessageToCore(t *testing.T) {
	msg := &agent.Message{
		ID:   "m1",
		Role: agent.RoleAssistant,
		Content: []agent.Content{
			{Type: agent.ContentTypeText, Text: "Hello"},
			{Type: agent.ContentTypeToolCall, ToolCall: &agent.ToolCall{
				ID:        "tc1",
				Name:      "test_tool",
				Arguments: "{}",
			}},
		},
		Timestamp: time.Now(),
	}

	coreMsg := convertAgentMessageToCore(msg)

	if coreMsg.ID != msg.ID {
		t.Errorf("expected ID %s, got %s", msg.ID, coreMsg.ID)
	}
	if coreMsg.Role != string(msg.Role) {
		t.Errorf("expected role %s, got %s", msg.Role, coreMsg.Role)
	}
	if coreMsg.Content != "Hello" {
		t.Errorf("expected content Hello, got %s", coreMsg.Content)
	}
	if len(coreMsg.ToolCalls) != 1 {
		t.Errorf("expected 1 tool call, got %d", len(coreMsg.ToolCalls))
	}
}

func TestConvertDomainMessageToAgent(t *testing.T) {
	msg := domain.Message{
		ID:      "m1",
		Role:    "assistant",
		Content: "Hello",
		ToolCalls: []domain.ToolCall{
			{
				ID:        "tc1",
				Type:      "function",
				Name:      "test_tool",
				Arguments: "{}",
			},
		},
		Timestamp: time.Now(),
	}

	agentMsg := convertDomainMessageToAgent(msg)

	if agentMsg.ID != msg.ID {
		t.Errorf("expected ID %s, got %s", msg.ID, agentMsg.ID)
	}
	if len(agentMsg.Content) != 2 {
		t.Errorf("expected 2 content items (text + tool call), got %d", len(agentMsg.Content))
	}
	if agentMsg.Content[0].Text != "Hello" {
		t.Errorf("expected text Hello, got %s", agentMsg.Content[0].Text)
	}
	if agentMsg.Content[1].ToolCall.Name != "test_tool" {
		t.Errorf("expected tool name test_tool, got %s", agentMsg.Content[1].ToolCall.Name)
	}
}

func TestConvertCoreEventToAgent(t *testing.T) {
	// Test ToolStart (CamelCase from EventFactory)
	ts := &domain.ToolStart{
		ThreadID:      "t1",
		ToolCallID:    "tc1",
		ToolName:      "test_tool",
		ToolArguments: "{}",
		Timestamp:     time.Now(),
	}

	agentEvent := ConvertCoreEventToAgent(ts)

	if agentEvent.Type() != agent.EventTypeToolCall {
		t.Errorf("expected event type %s, got %s", agent.EventTypeToolCall, agentEvent.Type())
	}

	data := agentEvent.Data().(*agent.ToolCallEvent)
	if data.ToolCall.ID != "tc1" {
		t.Errorf("expected tool call ID tc1, got %s", data.ToolCall.ID)
	}

	// Test ToolSuccess
	tr := &domain.ToolSuccess{
		ThreadID:   "t1",
		ToolCallID: "tc1",
		Result:     "success",
		Timestamp:  time.Now(),
	}

	agentEventResult := ConvertCoreEventToAgent(tr)
	if agentEventResult.Type() != agent.EventTypeToolResult {
		t.Errorf("expected event type %s, got %s", agent.EventTypeToolResult, agentEventResult.Type())
	}

	resultData := agentEventResult.Data().(*agent.ToolResultEvent)
	if resultData.Result.Output != "success" {
		t.Errorf("expected output success, got %s", resultData.Result.Output)
	}
}
