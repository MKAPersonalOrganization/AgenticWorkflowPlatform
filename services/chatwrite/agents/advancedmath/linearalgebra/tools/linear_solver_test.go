package tools

import (
	"context"
	"encoding/json"
	"math"
	"testing"
)

func TestLinearSolverTool_GaussianElimination(t *testing.T) {
	tool := NewLinearSolverTool()

	tests := []struct {
		name     string
		matrix   [][]float64
		vector   []float64
		expected []float64
	}{
		{
			name:     "2x2 system",
			matrix:   [][]float64{{2, 1}, {1, 3}},
			vector:   []float64{8, 13},
			expected: []float64{2.2, 3.6},
		},
		{
			name:     "3x3 system",
			matrix:   [][]float64{{2, 1, -1}, {-3, -1, 2}, {-2, 1, 2}},
			vector:   []float64{8, -11, -3},
			expected: []float64{2, 3, -1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{
				"method": "gauss",
				"A":      tt.matrix,
				"b":      tt.vector,
			}

			argsJSON, _ := json.Marshal(args)
			result, err := tool.Invoke(context.Background(), string(argsJSON))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var response map[string]any
			if err := json.Unmarshal([]byte(result), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			resultVec := response["solution"].([]any)
			for i, exp := range tt.expected {
				actual := resultVec[i].(float64)
				if math.Abs(actual-exp) > 1e-9 {
					t.Errorf("index %d: expected %v, got %v", i, exp, actual)
				}
			}
		})
	}
}

func TestLinearSolverTool_LU(t *testing.T) {
	tool := NewLinearSolverTool()

	args := map[string]any{
		"method": "lu",
		"A":      [][]float64{{4, 3}, {6, 3}},
		"b":      []float64{10, 12},
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	expected := []float64{1, 2}
	resultVec := response["solution"].([]any)
	for i, exp := range expected {
		actual := resultVec[i].(float64)
		if math.Abs(actual-exp) > 1e-9 {
			t.Errorf("index %d: expected %v, got %v", i, exp, actual)
		}
	}
}

func TestLinearSolverTool_Cholesky(t *testing.T) {
	tool := NewLinearSolverTool()

	args := map[string]any{
		"method": "cholesky",
		"A":      [][]float64{{4, 2}, {2, 3}},
		"b":      []float64{10, 7},
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]any
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	expected := []float64{2, 1}
	resultVec := response["solution"].([]any)
	for i, exp := range expected {
		actual := resultVec[i].(float64)
		if math.Abs(actual-exp) > 1e-9 {
			t.Errorf("index %d: expected %v, got %v", i, exp, actual)
		}
	}
}
