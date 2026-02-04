package engine

import (
	"encoding/json"
	"reflect"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/internal/llm"
)

func TestMapFromCoreToLLMMessages(t *testing.T) {
	t.Parallel()
	// Setup test data
	userMsg := core.Message{
		Role:    "user",
		Content: "Hello",
	}

	assistantMsg := core.Message{
		Role:      "assistant",
		Content:   "Hi there",
		AgentName: "TestAgent",
	}

	toolCallMsg := core.Message{
		Role: "assistant",
		ToolCalls: []core.ToolCall{
			{
				ID:        "call_123",
				Name:      "add",
				Arguments: `{"a":2,"b":2}`,
			},
		},
	}

	toolResultMsg := core.Message{
		Role:       "tool",
		ToolCallID: "call_123",
		Content:    `{"result":4}`,
	}

	input := []core.Message{userMsg, assistantMsg, toolCallMsg, toolResultMsg}

	// Calculate expected output
	expected := []llm.Message{
		{
			Role:    llm.RoleUser,
			Content: []llm.Content{llm.TextContent{Text: "Hello"}},
		},
		{
			Role:    llm.RoleAssistant,
			Name:    "TestAgent",
			Content: []llm.Content{llm.TextContent{Text: "Hi there"}},
		},
		{
			Role: llm.RoleAssistant,
			Content: []llm.Content{
				llm.FunctionCall{
					ID:        "call_123",
					Name:      "add",
					Arguments: json.RawMessage(`{"a":2,"b":2}`),
				},
			},
		},
		{
			Role: llm.RoleTool,
			Content: []llm.Content{
				llm.FunctionResult{
					CallID: "call_123",
					Name:   "", // Name might be empty in conversion if not provided in core.Message
					Result: json.RawMessage(`{"result":4}`),
				},
			},
		},
	}

	// Execute
	result := mapFromCoreToLLMMessages(input)

	// Verify
	if len(result) != len(expected) {
		t.Fatalf("Expected %d messages, got %d", len(expected), len(result))
	}

	for i, msg := range result {
		// Check basic fields
		if msg.Role != expected[i].Role {
			t.Errorf("Message %d: expected role %s, got %s", i, expected[i].Role, msg.Role)
		}
		if msg.Name != expected[i].Name {
			t.Errorf("Message %d: expected name %s, got %s", i, expected[i].Name, msg.Name)
		}

		// Check contents
		if len(msg.Content) != len(expected[i].Content) {
			t.Errorf("Message %d: expected %d contents, got %d", i, len(expected[i].Content), len(msg.Content))
			continue
		}

		for j, content := range msg.Content {
			expectedContent := expected[i].Content[j]
			if !reflect.DeepEqual(content, expectedContent) {
				t.Errorf("Message %d content %d: expected %+v, got %+v", i, j, expectedContent, content)
			}
		}
	}
}

func TestMapFromCoreToLLMMessages_Grouping(t *testing.T) {
	t.Parallel()
	// Setup test data with multiple tool results
	toolCallMsg := core.Message{
		Role: "assistant",
		ToolCalls: []core.ToolCall{
			{ID: "call_1", Name: "add", Arguments: `{"a":1,"b":1}`},
			{ID: "call_2", Name: "add", Arguments: `{"a":2,"b":2}`},
		},
	}

	toolResult1 := core.Message{
		Role:       "tool",
		ToolCallID: "call_1",
		Content:    `2`,
	}
	toolResult2 := core.Message{
		Role:       "tool",
		ToolCallID: "call_2",
		Content:    `4`,
	}

	input := []core.Message{toolCallMsg, toolResult1, toolResult2}

	// Execute
	result := mapFromCoreToLLMMessages(input)

	// Verify - should have 2 messages: 1 assistant with 2 calls, 1 tool with 2 results
	if len(result) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(result))
	}

	if result[0].Role != llm.RoleAssistant {
		t.Errorf("Expected first message role %s, got %s", llm.RoleAssistant, result[0].Role)
	}
	if len(result[0].Content) != 2 {
		t.Errorf("Expected 2 contents in first message, got %d", len(result[0].Content))
	}

	if result[1].Role != llm.RoleTool {
		t.Errorf("Expected second message role %s, got %s", llm.RoleTool, result[1].Role)
	}
	if len(result[1].Content) != 2 {
		t.Errorf("Expected 2 contents in second message (grouped results), got %d", len(result[1].Content))
	}

	// Verify content types
	for i, content := range result[1].Content {
		if _, ok := content.(llm.FunctionResult); !ok {
			t.Errorf("Content %d in second message is not FunctionResult, got %T", i, content)
		}
	}
}
