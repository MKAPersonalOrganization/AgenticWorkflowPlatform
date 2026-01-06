package core_test

import (
	"testing"

	"school-with-ai/sdk/agentsdk/llm"
	"school-with-ai/sdk/agentsdk/core"
)

func TestToolCallAccumulator_NewToolCall(t *testing.T) {
	acc := core.NewToolCallAccumulator()

	// First update starts a new tool call
	update := llm.ChatResponseUpdate{
		ToolCallIndex: 0,
		ToolCallID:    "call_123",
		Contents: []llm.AIContent{llm.FunctionCallContent{
			CallID: "call_123",
			Name:   "get_weather",
		}},
	}

	result, ok := acc.ProcessUpdate(update)
	if !ok {
		t.Fatal("expected update to be processed")
	}

	if !result.IsNew {
		t.Error("expected IsNew to be true")
	}
	if result.ID != "call_123" {
		t.Errorf("expected ID 'call_123', got '%s'", result.ID)
	}
	if result.Name != "get_weather" {
		t.Errorf("expected Name 'get_weather', got '%s'", result.Name)
	}
	if acc.Count() != 1 {
		t.Errorf("expected count 1, got %d", acc.Count())
	}
}

func TestToolCallAccumulator_ArgumentStreaming(t *testing.T) {
	acc := core.NewToolCallAccumulator()

	// Start tool call
	acc.ProcessUpdate(llm.ChatResponseUpdate{
		ToolCallIndex: 0,
		ToolCallID:    "call_123",
		Contents: []llm.AIContent{llm.FunctionCallContent{
			CallID: "call_123",
			Name:   "get_weather",
		}},
	})

	// Stream arguments in chunks
	update1, ok := acc.ProcessUpdate(llm.ChatResponseUpdate{
		ToolCallIndex: 0,
		Contents: []llm.AIContent{llm.FunctionCallContent{
			RawArguments: `{"city":`,
		}},
	})
	if !ok {
		t.Fatal("expected update to be processed")
	}
	if update1.IsNew {
		t.Error("expected IsNew to be false for continuation")
	}
	if update1.ArgsDelta != `{"city":` {
		t.Errorf("unexpected args delta: %s", update1.ArgsDelta)
	}

	update2, _ := acc.ProcessUpdate(llm.ChatResponseUpdate{
		ToolCallIndex: 0,
		Contents: []llm.AIContent{llm.FunctionCallContent{
			RawArguments: `"London"}`,
		}},
	})
	if update2.ArgsDelta != `"London"}` {
		t.Errorf("unexpected args delta: %s", update2.ArgsDelta)
	}

	// Verify final result
	calls := acc.GetAll()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Arguments != `{"city":"London"}` {
		t.Errorf("unexpected accumulated arguments: %s", calls[0].Function.Arguments)
	}
}

func TestToolCallAccumulator_MultipleToolCalls(t *testing.T) {
	acc := core.NewToolCallAccumulator()

	// First tool call
	acc.ProcessUpdate(llm.ChatResponseUpdate{
		ToolCallIndex: 0,
		ToolCallID:    "call_1",
		Contents: []llm.AIContent{llm.FunctionCallContent{
			CallID: "call_1",
			Name:   "get_weather",
		}},
	})
	acc.ProcessUpdate(llm.ChatResponseUpdate{
		ToolCallIndex: 0,
		Contents: []llm.AIContent{llm.FunctionCallContent{
			RawArguments: `{"city":"London"}`,
		}},
	})

	// Second tool call
	acc.ProcessUpdate(llm.ChatResponseUpdate{
		ToolCallIndex: 1,
		ToolCallID:    "call_2",
		Contents: []llm.AIContent{llm.FunctionCallContent{
			CallID: "call_2",
			Name:   "calculate",
		}},
	})
	acc.ProcessUpdate(llm.ChatResponseUpdate{
		ToolCallIndex: 1,
		Contents: []llm.AIContent{llm.FunctionCallContent{
			RawArguments: `{"expr":"2+2"}`,
		}},
	})

	if acc.Count() != 2 {
		t.Errorf("expected 2 calls, got %d", acc.Count())
	}

	calls := acc.GetAll()
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
}

func TestToolCallAccumulator_HasToolCalls(t *testing.T) {
	acc := core.NewToolCallAccumulator()

	if acc.HasToolCalls() {
		t.Error("expected HasToolCalls to be false initially")
	}

	acc.ProcessUpdate(llm.ChatResponseUpdate{
		ToolCallIndex: 0,
		ToolCallID:    "call_1",
		Contents: []llm.AIContent{llm.FunctionCallContent{
			CallID: "call_1",
			Name:   "get_weather",
		}},
	})

	if !acc.HasToolCalls() {
		t.Error("expected HasToolCalls to be true after adding a call")
	}
}

func TestToolCallAccumulator_UnknownIndex(t *testing.T) {
	acc := core.NewToolCallAccumulator()

	// Update for unknown index should not be processed
	_, ok := acc.ProcessUpdate(llm.ChatResponseUpdate{
		ToolCallIndex: 5, // No tool call at this index
		Contents: []llm.AIContent{llm.FunctionCallContent{
			RawArguments: `{"foo":"bar"}`,
		}},
	})

	if ok {
		t.Error("expected update for unknown index to not be processed")
	}
}
