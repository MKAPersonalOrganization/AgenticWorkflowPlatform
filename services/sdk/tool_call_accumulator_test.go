package sdk_test

import (
	"testing"

	"school-with-ai/services/sdk"
	"school-with-ai/services/llm"
)

func TestToolCallAccumulator_NewToolCall(t *testing.T) {
	acc := sdk.NewToolCallAccumulator()

	// First delta starts a new tool call
	delta := llm.ToolCallDelta{
		Index: 0,
		ID:    "call_123",
		Name:  "get_weather",
	}

	update, ok := acc.ProcessDelta(delta)
	if !ok {
		t.Fatal("expected delta to be processed")
	}

	if !update.IsNew {
		t.Error("expected IsNew to be true")
	}
	if update.ID != "call_123" {
		t.Errorf("expected ID 'call_123', got '%s'", update.ID)
	}
	if update.Name != "get_weather" {
		t.Errorf("expected Name 'get_weather', got '%s'", update.Name)
	}
	if acc.Count() != 1 {
		t.Errorf("expected count 1, got %d", acc.Count())
	}
}

func TestToolCallAccumulator_ArgumentStreaming(t *testing.T) {
	acc := sdk.NewToolCallAccumulator()

	// Start tool call
	acc.ProcessDelta(llm.ToolCallDelta{
		Index: 0,
		ID:    "call_123",
		Name:  "get_weather",
	})

	// Stream arguments in chunks
	update1, ok := acc.ProcessDelta(llm.ToolCallDelta{
		Index:     0,
		Arguments: `{"city":`,
	})
	if !ok {
		t.Fatal("expected delta to be processed")
	}
	if update1.IsNew {
		t.Error("expected IsNew to be false for continuation")
	}
	if update1.ArgsDelta != `{"city":` {
		t.Errorf("unexpected args delta: %s", update1.ArgsDelta)
	}

	update2, _ := acc.ProcessDelta(llm.ToolCallDelta{
		Index:     0,
		Arguments: `"London"}`,
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
	acc := sdk.NewToolCallAccumulator()

	// First tool call
	acc.ProcessDelta(llm.ToolCallDelta{
		Index: 0,
		ID:    "call_1",
		Name:  "get_weather",
	})
	acc.ProcessDelta(llm.ToolCallDelta{
		Index:     0,
		Arguments: `{"city":"London"}`,
	})

	// Second tool call
	acc.ProcessDelta(llm.ToolCallDelta{
		Index: 1,
		ID:    "call_2",
		Name:  "calculate",
	})
	acc.ProcessDelta(llm.ToolCallDelta{
		Index:     1,
		Arguments: `{"expr":"2+2"}`,
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
	acc := sdk.NewToolCallAccumulator()

	if acc.HasToolCalls() {
		t.Error("expected HasToolCalls to be false initially")
	}

	acc.ProcessDelta(llm.ToolCallDelta{
		Index: 0,
		ID:    "call_1",
		Name:  "get_weather",
	})

	if !acc.HasToolCalls() {
		t.Error("expected HasToolCalls to be true after adding a call")
	}
}

func TestToolCallAccumulator_UnknownIndex(t *testing.T) {
	acc := sdk.NewToolCallAccumulator()

	// Delta for unknown index should not be processed
	_, ok := acc.ProcessDelta(llm.ToolCallDelta{
		Index:     5, // No tool call at this index
		Arguments: `{"foo":"bar"}`,
	})

	if ok {
		t.Error("expected delta for unknown index to not be processed")
	}
}
