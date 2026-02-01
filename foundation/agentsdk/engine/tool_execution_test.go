package engine

import (
	"errors"
	"fmt"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
)

func TestToolExecutionService_ExecuteAll(t *testing.T) {
	service := NewToolExecutionService(nil)

	mockExecutor := &MockToolExecutor{
		executeFunc: func(tc core.ToolCall) (core.ToolExecutionResult, error) {
			switch tc.Name {
			case "tool1":
				return core.ToolExecutionResult{Result: "result1"}, nil
			case "tool2":
				return core.ToolExecutionResult{Result: "result2"}, nil
			case "tool_elicitation":
				return core.ToolExecutionResult{
					Command: &core.Command{
						Elicitation:   &core.ElicitationSpec{Question: "more info?"},
						StopIteration: true,
					},
					StopIteration: true,
				}, nil
			case "fail_tool":
				return core.ToolExecutionResult{}, errors.New("something went wrong")
			default:
				return core.ToolExecutionResult{Result: "default"}, nil
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

		if results.Results[0].Result != "result1" || results.Results[1].Result != "result2" {
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

		if !results.Results[1].IsError {
			t.Error("Expected fail_tool to have IsError true")
		}

		expectedError := fmt.Sprintf(`{"error": "%s"}`, "something went wrong")
		if results.Results[1].Result != expectedError {
			t.Errorf("Expected error JSON, got %s", results.Results[1].Result)
		}
	})
}
