package tools

import (
	"context"
	"encoding/json"
	"testing"
)

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
