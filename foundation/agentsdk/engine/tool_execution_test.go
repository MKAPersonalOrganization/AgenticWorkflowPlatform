package engine

import (
	"errors"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
)

func TestToolExecutionService_ExecuteAll(t *testing.T) {
	service := NewToolExecutionService(nil)

	mockExecutor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecResult, error) {
			switch tc.Name {
			case "tool1":
				return core.ToolExecResult{Result: "result1"}, nil
			case "tool2":
				return core.ToolExecResult{Result: "result2"}, nil
			case "tool_elicitation":
				return core.ToolExecResult{
					Command: &core.Command{
						Elicitation:   &core.ElicitationSpec{Question: "more info?"},
						StopIteration: true,
					},
					StopIteration: true,
				}, nil
			case "fail_tool":
				return core.ToolExecResult{}, errors.New("something went wrong")
			default:
				return core.ToolExecResult{Result: "default"}, nil
			}
		},
	}

	t.Run("All Successful", func(t *testing.T) {
		toolCalls := []core.ToolCall{
			{ID: "call1", Name: "tool1"},
			{ID: "call2", Name: "tool2"},
		}

		results := service.ExecuteAll(toolCalls, mockExecutor)

		if results.ShouldStop {
			t.Error("Expected ShouldStop to be false")
		}

		if len(results.Results) != 2 {
			t.Fatalf("Expected 2 results, got %d", len(results.Results))
		}

		if results.Results[0].Result.Result != "result1" || results.Results[1].Result.Result != "result2" {
			t.Errorf("Unexpected results: %v", results.Results)
		}
	})

	t.Run("One Elicitation Stops All", func(t *testing.T) {
		toolCalls := []core.ToolCall{
			{ID: "call1", Name: "tool1"},
			{ID: "call_elicit", Name: "tool_elicitation"},
			{ID: "call2", Name: "tool2"},
		}

		results := service.ExecuteAll(toolCalls, mockExecutor)

		if !results.ShouldStop {
			t.Error("Expected ShouldStop to be true")
		}

		if len(results.Results) != 3 {
			t.Fatalf("Expected 3 results, got %d", len(results.Results))
		}

		if !results.Results[1].StopIteration {
			t.Error("Expected tool_elicitation result to have StopIteration true")
		}
	})

	t.Run("Error Handling", func(t *testing.T) {
		toolCalls := []core.ToolCall{
			{ID: "ok", Name: "tool1"},
			{ID: "fail", Name: "fail_tool"},
		}

		results := service.ExecuteAll(toolCalls, mockExecutor)

		if len(results.Results) != 2 {
			t.Fatalf("Expected 2 results, got %d", len(results.Results))
		}

		if results.Results[1].Error == nil {
			t.Error("Expected fail_tool to have an Error")
		}

		if results.Results[1].Result.Result == "" {
			t.Error("Expected fail_tool to have a Result even with error")
		}
	})
}
