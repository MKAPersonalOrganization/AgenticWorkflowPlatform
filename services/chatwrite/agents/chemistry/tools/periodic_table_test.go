package tools

import (
	"context"
	"encoding/json"
	"testing"
)

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
