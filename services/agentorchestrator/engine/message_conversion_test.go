package engine

import (
	"reflect"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/llm"
)

func TestToLLMChatMessages(t *testing.T) {
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
					Name:      "get_weather",
					Arguments: `{"city":"London"}`,
				},
			},
		},
	}

	toolResultMsg := core.Message{
		Role:       "tool",
		ToolCallID: "call_123",
		Content:    "Sunny",
	}

	handoffMsg := core.Message{
		Role:     "system",
		Metadata: map[string]any{"type": "agent_handoff"},
	}

	input := []core.Message{userMsg, assistantMsg, toolCallMsg, toolResultMsg, handoffMsg}

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
					Name:         "get_weather",
					RawArguments: `{"city":"London"}`,
				},
			},
		},
		{
			Role: llm.ChatRoleTool,
			Contents: []llm.AIContent{
				llm.FunctionResultContent{
					CallID: "call_123",
					Result: "Sunny",
				},
			},
		},
	}

	// Execute
	result := toLLMChatMessages(input)

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
