package engine

import (
	"reflect"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
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
				ID: "call_123",
				Function: core.ToolFunction{
					Name:      "add",
					Arguments: `{"a":2,"b":2}`,
				},
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
	expected := []llm.ChatMessage{
		{
			Role:     llm.ChatRoleUser,
			Contents: []llm.AIContent{llm.TextContent{Text: "Hello"}},
		},
		{
			Role:       llm.ChatRoleAssistant,
			AuthorName: "TestAgent",
			Contents:   []llm.AIContent{llm.TextContent{Text: "Hi there"}},
		},
		{
			Role: llm.ChatRoleAssistant,
			Contents: []llm.AIContent{
				llm.FunctionCallContent{
					CallID:       "call_123",
					Name:         "add",
					RawArguments: `{"a":2,"b":2}`,
				},
			},
		},
		{
			Role: llm.ChatRoleTool,
			Contents: []llm.AIContent{
				llm.FunctionResultContent{
					CallID: "call_123",
					Result: `{"result":4}`,
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
		if msg.AuthorName != expected[i].AuthorName {
			t.Errorf("Message %d: expected author %s, got %s", i, expected[i].AuthorName, msg.AuthorName)
		}

		// Check contents
		if len(msg.Contents) != len(expected[i].Contents) {
			t.Errorf("Message %d: expected %d contents, got %d", i, len(expected[i].Contents), len(msg.Contents))
			continue
		}

		for j, content := range msg.Contents {
			expectedContent := expected[i].Contents[j]
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
			{ID: "call_1", Function: core.ToolFunction{Name: "add", Arguments: `{"a":1,"b":1}`}},
			{ID: "call_2", Function: core.ToolFunction{Name: "add", Arguments: `{"a":2,"b":2}`}},
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

	if result[0].Role != llm.ChatRoleAssistant {
		t.Errorf("Expected first message role %s, got %s", llm.ChatRoleAssistant, result[0].Role)
	}
	if len(result[0].Contents) != 2 {
		t.Errorf("Expected 2 contents in first message, got %d", len(result[0].Contents))
	}

	if result[1].Role != llm.ChatRoleTool {
		t.Errorf("Expected second message role %s, got %s", llm.ChatRoleTool, result[1].Role)
	}
	if len(result[1].Contents) != 2 {
		t.Errorf("Expected 2 contents in second message (grouped results), got %d", len(result[1].Contents))
	}

	// Verify content types
	for i, content := range result[1].Contents {
		if _, ok := content.(llm.FunctionResultContent); !ok {
			t.Errorf("Content %d in second message is not FunctionResultContent, got %T", i, content)
		}
	}
}
