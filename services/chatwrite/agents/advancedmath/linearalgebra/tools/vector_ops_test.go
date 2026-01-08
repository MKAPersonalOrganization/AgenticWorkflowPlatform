package tools

import (
	"context"
	"encoding/json"
	"math"
	"testing"
)

func TestVectorOperationsTool_Add(t *testing.T) {
	t.Parallel()
	tool := NewVectorOperationsTool()

	tests := []struct {
		name     string
		args     map[string]any
		expected []float64
		wantErr  bool
	}{
		{
			name: "simple addition",
			args: map[string]any{
				"operation": "add",
				"vector1":   []float64{1, 2, 3},
				"vector2":   []float64{4, 5, 6},
			},
			expected: []float64{5, 7, 9},
		},
		{
			name: "scaled addition",
			args: map[string]any{
				"operation": "add",
				"vector1":   []float64{1, 2, 3},
				"vector2":   []float64{4, 5, 6},
				"alpha":     2.0,
				"beta":      0.5,
			},
			expected: []float64{4, 6.5, 9},
		},
		{
			name: "missing vector2",
			args: map[string]any{
				"operation": "add",
				"vector1":   []float64{1, 2, 3},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			argsJSON, _ := json.Marshal(tt.args)
			result, err := tool.Invoke(context.Background(), string(argsJSON))

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var response map[string]any
			if err := json.Unmarshal([]byte(result), &response); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}

			resultVec := response["result"].([]any)
			for i, expected := range tt.expected {
				actual := resultVec[i].(float64)
				if math.Abs(actual-expected) > 1e-10 {
					t.Errorf("index %d: expected %v, got %v", i, expected, actual)
				}
			}
		})
	}
}

func TestVectorOperationsTool_Dot(t *testing.T) {
	t.Parallel()
	tool := NewVectorOperationsTool()

	tests := []struct {
		name     string
		vector1  []float64
		vector2  []float64
		expected float64
	}{
		{
			name:     "simple dot product",
			vector1:  []float64{1, 2, 3},
			vector2:  []float64{4, 5, 6},
			expected: 32.0, // 1*4 + 2*5 + 3*6 = 32
		},
		{
			name:     "orthogonal vectors",
			vector1:  []float64{1, 0, 0},
			vector2:  []float64{0, 1, 0},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := map[string]any{
				"operation": "dot",
				"vector1":   tt.vector1,
				"vector2":   tt.vector2,
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

			actual := response["result"].(float64)
			if math.Abs(actual-tt.expected) > 1e-10 {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestVectorOperationsTool_Cross(t *testing.T) {
	t.Parallel()
	tool := NewVectorOperationsTool()

	tests := []struct {
		name     string
		vector1  []float64
		vector2  []float64
		expected []float64
	}{
		{
			name:     "standard basis vectors",
			vector1:  []float64{1, 0, 0},
			vector2:  []float64{0, 1, 0},
			expected: []float64{0, 0, 1},
		},
		{
			name:     "general vectors",
			vector1:  []float64{2, 3, 4},
			vector2:  []float64{5, 6, 7},
			expected: []float64{-3, 6, -3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := map[string]any{
				"operation": "cross",
				"vector1":   tt.vector1,
				"vector2":   tt.vector2,
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

			resultVec := response["result"].([]any)
			for i, exp := range tt.expected {
				actual := resultVec[i].(float64)
				if math.Abs(actual-exp) > 1e-10 {
					t.Errorf("index %d: expected %v, got %v", i, exp, actual)
				}
			}
		})
	}
}

func TestVectorOperationsTool_Norm(t *testing.T) {
	t.Parallel()
	tool := NewVectorOperationsTool()

	tests := []struct {
		name     string
		vector   []float64
		expected float64
	}{
		{
			name:     "unit vector",
			vector:   []float64{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "pythagorean triple",
			vector:   []float64{3, 4},
			expected: 5.0,
		},
		{
			name:     "3D vector",
			vector:   []float64{1, 2, 2},
			expected: 3.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := map[string]any{
				"operation": "norm",
				"vector1":   tt.vector,
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

			actual := response["result"].(float64)
			if math.Abs(actual-tt.expected) > 1e-10 {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestVectorOperationsTool_Unit(t *testing.T) {
	t.Parallel()
	tool := NewVectorOperationsTool()

	args := map[string]any{
		"operation": "unit",
		"vector1":   []float64{3, 4},
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

	expected := []float64{0.6, 0.8}
	resultVec := response["result"].([]any)
	for i, exp := range expected {
		actual := resultVec[i].(float64)
		if math.Abs(actual-exp) > 1e-10 {
			t.Errorf("index %d: expected %v, got %v", i, exp, actual)
		}
	}
}
