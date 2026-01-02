package tools

import (
	"context"
	"encoding/json"
	"testing"
)

func TestGraphingTool_Interface(t *testing.T) {
	tool := NewGraphingTool()

	if tool.Name() != "plot_function" {
		t.Errorf("Expected name 'plot_function', got '%s'", tool.Name())
	}

	if tool.Description() == "" {
		t.Error("Description should not be empty")
	}

	params := tool.Parameters()
	if params == nil {
		t.Error("Parameters should not be nil")
	}
}

func TestGraphingTool_Invoke(t *testing.T) {
	tool := NewGraphingTool()

	args := `{"function": "x^2", "x_min": -5, "x_max": 5}`
	result, err := tool.Invoke(context.Background(), args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if response["function"] != "x^2" {
		t.Errorf("Expected function 'x^2', got '%v'", response["function"])
	}
}

func TestEquationSolverTool_Interface(t *testing.T) {
	tool := NewEquationSolverTool()

	if tool.Name() != "solve_equation" {
		t.Errorf("Expected name 'solve_equation', got '%s'", tool.Name())
	}
}

func TestEquationSolverTool_Invoke(t *testing.T) {
	tool := NewEquationSolverTool()

	args := `{"equation": "2x + 5 = 13"}`
	result, err := tool.Invoke(context.Background(), args)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("Failed to parse result: %v", err)
	}

	if response["type"] != "linear" {
		t.Errorf("Expected type 'linear', got '%v'", response["type"])
	}

	if response["solutions"] == nil {
		t.Error("Expected solutions array")
	}
}
