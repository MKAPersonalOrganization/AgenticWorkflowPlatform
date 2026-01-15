package tools

import (
	"context"
	"school-with-ai/foundation/agentsdk/core"
	"testing"
)

func TestExecuteCalculatorTool(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name           string
		args           any
		wantExpression string
		wantResult     float64
		wantErr        bool
	}{
		{
			name:           "simple addition",
			args:           map[string]any{"expression": "2 + 2"},
			wantExpression: "2 + 2",
			wantResult:     4,
			wantErr:        false,
		},
		{
			name:           "simple subtraction",
			args:           map[string]any{"expression": "10 - 3"},
			wantExpression: "10 - 3",
			wantResult:     7,
			wantErr:        false,
		},
		{
			name:           "simple multiplication",
			args:           map[string]any{"expression": "5 * 6"},
			wantExpression: "5 * 6",
			wantResult:     30,
			wantErr:        false,
		},
		{
			name:           "simple division",
			args:           map[string]any{"expression": "20 / 4"},
			wantExpression: "20 / 4",
			wantResult:     5,
			wantErr:        false,
		},
		{
			name:           "complex expression",
			args:           map[string]any{"expression": "2 + 3 * 4"},
			wantExpression: "2 + 3 * 4",
			wantResult:     14,
			wantErr:        false,
		},
		{
			name:           "parentheses",
			args:           map[string]any{"expression": "(2 + 3) * 4"},
			wantExpression: "(2 + 3) * 4",
			wantResult:     20,
			wantErr:        false,
		},
		{
			name:           "exponentiation",
			args:           map[string]any{"expression": "2^8"},
			wantExpression: "2^8",
			wantResult:     256,
			wantErr:        false,
		},
		{
			name:           "square root",
			args:           map[string]any{"expression": "sqrt(16)"},
			wantExpression: "sqrt(16)",
			wantResult:     4,
			wantErr:        false,
		},
		{
			name:           "negative numbers",
			args:           map[string]any{"expression": "-5 + 10"},
			wantExpression: "-5 + 10",
			wantResult:     5,
			wantErr:        false,
		},
		{
			name:           "decimal numbers",
			args:           map[string]any{"expression": "3.5 * 2"},
			wantExpression: "3.5 * 2",
			wantResult:     7,
			wantErr:        false,
		},
		{
			name:    "invalid arguments type",
			args:    "not a map or struct",
			wantErr: true,
		},
		{
			name:    "division by zero",
			args:    map[string]any{"expression": "10 / 0"},
			wantErr: true,
		},
		{
			name:    "empty expression",
			args:    map[string]any{"expression": ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := ExecuteCalculatorTool(tt.args)

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

			resultData, ok := result.(map[string]any)
			if !ok {
				t.Fatalf("Expected map[string]any result, got %T", result)
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
	t.Parallel()
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
		{"complex", "sqrt(16) + 2^3", 12, false},
		{"negative", "-10 + 5", -5, false},
		{"decimal", "3.14 * 2", 6.28, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

func TestCalculatorTool_Interface(t *testing.T) {
	t.Parallel()
	calc := NewCalculatorTool()

	if calc.Name() != "calculate" {
		t.Errorf("Name() = %s, want 'calculate'", calc.Name())
	}

	if calc.Description() == "" {
		t.Error("Description() should not be empty")
	}

	params := calc.Parameters()
	if params["type"] != "object" {
		t.Errorf("Parameters type = %v, want 'object'", params["type"])
	}

	props := params["properties"].(map[string]any)
	if _, ok := props["expression"]; !ok {
		t.Error("Parameters should have 'expression' property")
	}

	required := params["required"].([]string)
	if len(required) != 1 || required[0] != "expression" {
		t.Errorf("Parameters required = %v, want ['expression']", params["required"])
	}
}

func TestCalculatorTool_Invoke(t *testing.T) {
	t.Parallel()
	calc := NewCalculatorTool()
	ctx := context.Background()

	// Test successful invocation
	result, err := calc.Invoke(core.NewToolContext(ctx, "", "", "", nil), map[string]any{"expression": "2 + 2"})
	if err != nil {
		t.Fatalf("Invoke() unexpected error: %v", err)
	}

	response, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map[string]any result, got %T", result)
	}

	if response["result"] != 4.0 {
		t.Errorf("Invoke() result = %v, want 4", response["result"])
	}
}
