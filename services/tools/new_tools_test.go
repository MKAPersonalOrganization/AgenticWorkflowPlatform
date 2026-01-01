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

func TestUnitConversionTool_Interface(t *testing.T) {
	tool := NewUnitConversionTool()

	if tool.Name() != "convert_units" {
		t.Errorf("Expected name 'convert_units', got '%s'", tool.Name())
	}
}

func TestUnitConversionTool_Invoke(t *testing.T) {
	tool := NewUnitConversionTool()

	tests := []struct {
		name     string
		args     string
		expected float64
	}{
		{
			name:     "meters to feet",
			args:     `{"value": 10, "from_unit": "meters", "to_unit": "feet"}`,
			expected: 32.8084,
		},
		{
			name:     "celsius to fahrenheit",
			args:     `{"value": 0, "from_unit": "celsius", "to_unit": "fahrenheit"}`,
			expected: 32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Invoke(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var response map[string]any
			if err := json.Unmarshal([]byte(result), &response); err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			converted := response["converted_value"].(float64)
			if converted < tt.expected-0.1 || converted > tt.expected+0.1 {
				t.Errorf("Expected ~%.2f, got %.2f", tt.expected, converted)
			}
		})
	}
}

func TestPeriodicTableTool_Interface(t *testing.T) {
	tool := NewPeriodicTableTool()

	if tool.Name() != "lookup_element" {
		t.Errorf("Expected name 'lookup_element', got '%s'", tool.Name())
	}
}

func TestPeriodicTableTool_Invoke(t *testing.T) {
	tool := NewPeriodicTableTool()

	tests := []struct {
		element        string
		expectedSymbol string
		expectedNumber int
	}{
		{"H", "H", 1},
		{"O", "O", 8},
		{"Fe", "Fe", 26},
	}

	for _, tt := range tests {
		t.Run(tt.element, func(t *testing.T) {
			args := `{"element": "` + tt.element + `"}`
			result, err := tool.Invoke(context.Background(), args)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			var response map[string]any
			if err := json.Unmarshal([]byte(result), &response); err != nil {
				t.Fatalf("Failed to parse result: %v", err)
			}

			if response["symbol"] != tt.expectedSymbol {
				t.Errorf("Expected symbol '%s', got '%v'", tt.expectedSymbol, response["symbol"])
			}

			if int(response["atomic_number"].(float64)) != tt.expectedNumber {
				t.Errorf("Expected atomic number %d, got %v", tt.expectedNumber, response["atomic_number"])
			}
		})
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
