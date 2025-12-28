package tools

import (
	"encoding/json"
	"testing"
)

func TestExecuteCalculatorTool(t *testing.T) {
	tests := []struct {
		name           string
		argsJSON       string
		wantExpression string
		wantResult     float64
		wantErr        bool
	}{
		{
			name:           "simple addition",
			argsJSON:       `{"expression": "2 + 2"}`,
			wantExpression: "2 + 2",
			wantResult:     4,
			wantErr:        false,
		},
		{
			name:           "simple subtraction",
			argsJSON:       `{"expression": "10 - 3"}`,
			wantExpression: "10 - 3",
			wantResult:     7,
			wantErr:        false,
		},
		{
			name:           "simple multiplication",
			argsJSON:       `{"expression": "5 * 6"}`,
			wantExpression: "5 * 6",
			wantResult:     30,
			wantErr:        false,
		},
		{
			name:           "simple division",
			argsJSON:       `{"expression": "20 / 4"}`,
			wantExpression: "20 / 4",
			wantResult:     5,
			wantErr:        false,
		},
		{
			name:           "complex expression",
			argsJSON:       `{"expression": "2 + 3 * 4"}`,
			wantExpression: "2 + 3 * 4",
			wantResult:     14, // 2 + (3 * 4) = 2 + 12 = 14
			wantErr:        false,
		},
		{
			name:           "parentheses",
			argsJSON:       `{"expression": "(2 + 3) * 4"}`,
			wantExpression: "(2 + 3) * 4",
			wantResult:     20, // (2 + 3) * 4 = 5 * 4 = 20
			wantErr:        false,
		},
		{
			name:           "exponentiation",
			argsJSON:       `{"expression": "2^8"}`,
			wantExpression: "2^8",
			wantResult:     256,
			wantErr:        false,
		},
		{
			name:           "square root",
			argsJSON:       `{"expression": "sqrt(16)"}`,
			wantExpression: "sqrt(16)",
			wantResult:     4,
			wantErr:        false,
		},
		{
			name:           "negative numbers",
			argsJSON:       `{"expression": "-5 + 10"}`,
			wantExpression: "-5 + 10",
			wantResult:     5,
			wantErr:        false,
		},
		{
			name:           "decimal numbers",
			argsJSON:       `{"expression": "3.5 * 2"}`,
			wantExpression: "3.5 * 2",
			wantResult:     7,
			wantErr:        false,
		},
		{
			name:     "invalid JSON",
			argsJSON: `{invalid}`,
			wantErr:  true,
		},
		{
			name:     "division by zero",
			argsJSON: `{"expression": "10 / 0"}`,
			wantErr:  true,
		},
		{
			name:     "empty expression",
			argsJSON: `{"expression": ""}`,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExecuteCalculatorTool(tt.argsJSON)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ExecuteCalculatorTool() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("ExecuteCalculatorTool() unexpected error: %v", err)
				return
			}

			// Parse the result JSON
			var resultData map[string]any
			if err := json.Unmarshal([]byte(result), &resultData); err != nil {
				t.Errorf("Failed to parse result JSON: %v", err)
				return
			}

			// Check expression
			if expr, ok := resultData["expression"].(string); ok {
				if expr != tt.wantExpression {
					t.Errorf("ExecuteCalculatorTool() expression = %v, want %v", expr, tt.wantExpression)
				}
			} else {
				t.Errorf("ExecuteCalculatorTool() result missing expression field")
			}

			// Check result
			if res, ok := resultData["result"].(float64); ok {
				if res != tt.wantResult {
					t.Errorf("ExecuteCalculatorTool() result = %v, want %v", res, tt.wantResult)
				}
			} else {
				t.Errorf("ExecuteCalculatorTool() result missing result field")
			}
		})
	}
}

func TestEvaluateExpression(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		want       float64
		wantErr    bool
	}{
		{"addition", "1 + 1", 2, false},
		{"subtraction", "5 - 3", 2, false},
		{"multiplication", "4 * 5", 20, false},
		{"division", "10 / 2", 5, false},
		{"operator precedence", "2 + 3 * 4", 14, false},
		{"parentheses", "(2 + 3) * 4", 20, false},
		{"nested parentheses", "((2 + 3) * 4) - 5", 15, false},
		{"exponentiation", "3^2", 9, false},
		{"square root", "sqrt(25)", 5, false},
		{"complex", "sqrt(16) + 2^3", 12, false}, // sqrt(16) = 4, 2^3 = 8, 4 + 8 = 12
		{"negative", "-10 + 5", -5, false},
		{"decimal", "3.14 * 2", 6.28, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := evaluateExpression(tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("evaluateExpression() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("evaluateExpression() = %v, want %v", got, tt.want)
			}
		})
	}
}
