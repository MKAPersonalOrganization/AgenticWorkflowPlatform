package core_test

import (
	"encoding/json"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
	"school-with-ai/foundation/agentsdk/internal/llm"
)

func TestToolCallAccumulator_NewToolCall(t *testing.T) {
	t.Parallel()
	acc := core.NewToolCallAccumulator()

	// First update starts a new tool call
	update := &llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			ID:   "call_123",
			Name: "get_weather",
		}},
		Extensions: llm.Metadata{"index": 0},
	}

	updates, ok := acc.ProcessUpdate(update)
	if !ok {
		t.Fatal("expected update to be processed")
	}

	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	result := updates[0]

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
	t.Parallel()
	acc := core.NewToolCallAccumulator()

	// Start tool call
	acc.ProcessUpdate(&llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			ID:   "call_1",
			Name: "get_weather",
		}},
		Extensions: llm.Metadata{"index": 0},
	})

	// Stream arguments in chunks
	updates1, ok := acc.ProcessUpdate(&llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			Arguments: json.RawMessage(`{"city":`),
		}},
		Extensions: llm.Metadata{"index": 0},
	})
	if !ok {
		t.Fatal("expected update to be processed")
	}
	if len(updates1) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates1))
	}
	update1 := updates1[0]

	if update1.IsNew {
		t.Error("expected IsNew to be false for continuation")
	}
	if update1.ArgsDelta != `{"city":` {
		t.Errorf("unexpected args delta: %s", update1.ArgsDelta)
	}

	updates2, _ := acc.ProcessUpdate(&llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			Arguments: json.RawMessage(`"London"}`),
		}},
		Extensions: llm.Metadata{"index": 0},
	})
	if len(updates2) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates2))
	}
	update2 := updates2[0]

	if update2.ArgsDelta != `"London"}` {
		t.Errorf("unexpected args delta: %s", update2.ArgsDelta)
	}

	// Verify final result
	calls := acc.GetAll()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Arguments != `{"city":"London"}` {
		t.Errorf("unexpected accumulated arguments: %s", calls[0].Arguments)
	}
}

func TestToolCallAccumulator_MultipleToolCalls(t *testing.T) {
	t.Parallel()
	acc := core.NewToolCallAccumulator()

	// First tool call
	acc.ProcessUpdate(&llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			ID:   "call_1",
			Name: "get_weather",
		}},
		Extensions: llm.Metadata{"index": 0},
	})
	acc.ProcessUpdate(&llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			Arguments: json.RawMessage(`{"city":"London"}`),
		}},
		Extensions: llm.Metadata{"index": 0},
	})

	// Second tool call
	acc.ProcessUpdate(&llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			ID:   "call_2",
			Name: "calculate",
		}},
		Extensions: llm.Metadata{"index": 1},
	})
	acc.ProcessUpdate(&llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			Arguments: json.RawMessage(`{"expr":"2+2"}`),
		}},
		Extensions: llm.Metadata{"index": 1},
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
	t.Parallel()
	acc := core.NewToolCallAccumulator()

	if acc.HasToolCalls() {
		t.Error("expected HasToolCalls to be false initially")
	}

	acc.ProcessUpdate(&llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			ID:   "call_1",
			Name: "get_weather",
		}},
		Extensions: llm.Metadata{"index": 0},
	})

	if !acc.HasToolCalls() {
		t.Error("expected HasToolCalls to be true after adding a call")
	}
}

func TestToolCallAccumulator_UnknownIndex(t *testing.T) {
	t.Parallel()
	acc := core.NewToolCallAccumulator()

	// Update for unknown index should not be processed
	_, ok := acc.ProcessUpdate(&llm.Response{
		Content: []llm.Content{llm.FunctionCall{
			Arguments: json.RawMessage(`{"foo":"bar"}`),
		}},
		Extensions: llm.Metadata{"index": 5}, // No tool call at this index
	})

	if ok {
		t.Error("expected update for unknown index to not be processed")
	}
}
