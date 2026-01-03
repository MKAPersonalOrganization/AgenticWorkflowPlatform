package tools

import (
	"context"
	"encoding/json"
	"math"
	"testing"
)

func TestMatrixOperationsTool_Multiply(t *testing.T) {
	tool := NewMatrixOperationsTool()

	matrix1 := [][]float64{{1, 2}, {3, 4}}
	matrix2 := [][]float64{{5, 6}, {7, 8}}

	args := map[string]any{
		"operation": "multiply",
		"matrix":    matrix1,
		"matrix2":   matrix2,
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]any
	json.Unmarshal([]byte(result), &response)

	expected := [][]float64{{19, 22}, {43, 50}}
	resultMatrix := response["result"].([]interface{})
	for i := range expected {
		row := resultMatrix[i].([]interface{})
		for j, exp := range expected[i] {
			actual := row[j].(float64)
			if math.Abs(actual-exp) > 1e-10 {
				t.Errorf("at [%d][%d]: expected %v, got %v", i, j, exp, actual)
			}
		}
	}
}

func TestMatrixOperationsTool_Transpose(t *testing.T) {
	tool := NewMatrixOperationsTool()

	args := map[string]any{
		"operation": "transpose",
		"matrix":    [][]float64{{1, 2, 3}, {4, 5, 6}},
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]any
	json.Unmarshal([]byte(result), &response)

	expected := [][]float64{{1, 4}, {2, 5}, {3, 6}}
	resultMatrix := response["result"].([]interface{})
	for i := range expected {
		row := resultMatrix[i].([]interface{})
		for j, exp := range expected[i] {
			actual := row[j].(float64)
			if math.Abs(actual-exp) > 1e-10 {
				t.Errorf("at [%d][%d]: expected %v, got %v", i, j, exp, actual)
			}
		}
	}
}

func TestMatrixOperationsTool_Determinant(t *testing.T) {
	tool := NewMatrixOperationsTool()

	tests := []struct {
		name     string
		matrix   [][]float64
		expected float64
	}{
		{
			name:     "2x2 matrix",
			matrix:   [][]float64{{4, 7}, {2, 6}},
			expected: 10.0,
		},
		{
			name:     "identity matrix",
			matrix:   [][]float64{{1, 0}, {0, 1}},
			expected: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := map[string]any{
				"operation": "determinant",
				"matrix":    tt.matrix,
			}

			argsJSON, _ := json.Marshal(args)
			result, err := tool.Invoke(context.Background(), string(argsJSON))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var response map[string]any
			json.Unmarshal([]byte(result), &response)

			actual := response["result"].(float64)
			if math.Abs(actual-tt.expected) > 1e-9 {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestMatrixOperationsTool_Inverse(t *testing.T) {
	tool := NewMatrixOperationsTool()

	args := map[string]any{
		"operation": "inverse",
		"matrix":    [][]float64{{4, 7}, {2, 6}},
	}

	argsJSON, _ := json.Marshal(args)
	result, err := tool.Invoke(context.Background(), string(argsJSON))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response map[string]any
	json.Unmarshal([]byte(result), &response)

	expected := [][]float64{{0.6, -0.7}, {-0.2, 0.4}}
	resultMatrix := response["result"].([]interface{})
	for i := range expected {
		row := resultMatrix[i].([]interface{})
		for j, exp := range expected[i] {
			actual := row[j].(float64)
			if math.Abs(actual-exp) > 1e-9 {
				t.Errorf("at [%d][%d]: expected %v, got %v", i, j, exp, actual)
			}
		}
	}
}
