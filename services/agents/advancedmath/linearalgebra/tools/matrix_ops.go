package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

// MatrixOperationsTool provides matrix operations
type MatrixOperationsTool struct{}

func NewMatrixOperationsTool() *MatrixOperationsTool {
	return &MatrixOperationsTool{}
}

func (t *MatrixOperationsTool) Name() string {
	return "matrix_operations"
}

func (t *MatrixOperationsTool) Description() string {
	return "Perform matrix operations: multiply, add, transpose, inverse, determinant, trace, norm"
}

func (t *MatrixOperationsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type": "string",
				"enum": []string{
					"multiply", "add", "subtract", "transpose", "inverse",
					"determinant", "trace", "norm_frobenius", "scale",
					"matrix_vector_multiply", "vector_matrix_multiply",
				},
				"description": "Matrix operation to perform",
			},
			"matrix": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "array", "items": map[string]string{"type": "number"}},
				"description": "Matrix (as array of rows)",
			},
			"matrix2": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "array", "items": map[string]string{"type": "number"}},
				"description": "Second matrix (for binary operations)",
			},
			"vector": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Vector (for matrix-vector operations)",
			},
			"alpha": map[string]any{
				"type":        "number",
				"description": "Alpha scalar coefficient",
				"default":     1.0,
			},
			"beta": map[string]any{
				"type":        "number",
				"description": "Beta scalar coefficient",
				"default":     1.0,
			},
			"scalar": map[string]any{
				"type":        "number",
				"description": "Scalar value (for scale operation)",
			},
		},
		"required": []string{"operation", "matrix"},
	}
}

func (t *MatrixOperationsTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Operation string      `json:"operation"`
		Matrix    [][]float64 `json:"matrix"`
		Matrix2   [][]float64 `json:"matrix2,omitempty"`
		Vector    []float64   `json:"vector,omitempty"`
		Alpha     float64     `json:"alpha,omitempty"`
		Beta      float64     `json:"beta,omitempty"`
		Scalar    float64     `json:"scalar,omitempty"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Set defaults
	if args.Alpha == 0 && (args.Operation == "add" || args.Operation == "multiply") {
		args.Alpha = 1.0
	}
	if args.Beta == 0 && args.Operation == "add" {
		args.Beta = 1.0
	}

	var result any
	var err error

	switch args.Operation {
	case "multiply":
		if len(args.Matrix2) == 0 {
			return "", fmt.Errorf("matrix2 required for multiply operation")
		}
		result, err = matMul(args.Matrix, args.Matrix2, args.Alpha)

	case "add":
		if len(args.Matrix2) == 0 {
			return "", fmt.Errorf("matrix2 required for add operation")
		}
		result, err = matAdd(args.Matrix, args.Matrix2, args.Alpha, args.Beta)

	case "subtract":
		if len(args.Matrix2) == 0 {
			return "", fmt.Errorf("matrix2 required for subtract operation")
		}
		result, err = matAdd(args.Matrix, args.Matrix2, 1.0, -1.0)

	case "transpose":
		result = matTranspose(args.Matrix)

	case "inverse":
		result, err = matInverse(args.Matrix)

	case "determinant":
		result, err = matDeterminant(args.Matrix)

	case "trace":
		result, err = matTrace(args.Matrix)

	case "norm_frobenius":
		result = matNormFrobenius(args.Matrix)

	case "scale":
		result = matScale(args.Matrix, args.Scalar)

	case "matrix_vector_multiply":
		if len(args.Vector) == 0 {
			return "", fmt.Errorf("vector required for matrix_vector_multiply")
		}
		result, err = matVecMul(args.Matrix, args.Vector, args.Alpha)

	case "vector_matrix_multiply":
		if len(args.Vector) == 0 {
			return "", fmt.Errorf("vector required for vector_matrix_multiply")
		}
		result, err = vecMatMul(args.Vector, args.Matrix, args.Alpha)

	default:
		return "", fmt.Errorf("unknown operation: %s", args.Operation)
	}

	if err != nil {
		return "", fmt.Errorf("operation failed: %w", err)
	}

	response := map[string]any{
		"operation": args.Operation,
		"result":    result,
	}

	data, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	return string(data), nil
}

// Matrix operation implementations

func matMul(a, b [][]float64, alpha float64) ([][]float64, error) {
	m, n, p := len(a), len(a[0]), len(b[0])
	if len(b) != n {
		return nil, fmt.Errorf("incompatible dimensions for multiplication: (%dx%d) x (%dx%d)", m, n, len(b), p)
	}

	c := make([][]float64, m)
	for i := 0; i < m; i++ {
		c[i] = make([]float64, p)
		for j := 0; j < p; j++ {
			for k := 0; k < n; k++ {
				c[i][j] += alpha * a[i][k] * b[k][j]
			}
		}
	}
	return c, nil
}

func matAdd(a, b [][]float64, alpha, beta float64) ([][]float64, error) {
	if len(a) != len(b) || len(a[0]) != len(b[0]) {
		return nil, fmt.Errorf("matrices must have same dimensions")
	}

	m, n := len(a), len(a[0])
	c := make([][]float64, m)
	for i := 0; i < m; i++ {
		c[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			c[i][j] = alpha*a[i][j] + beta*b[i][j]
		}
	}
	return c, nil
}

func matTranspose(a [][]float64) [][]float64 {
	m, n := len(a), len(a[0])
	t := make([][]float64, n)
	for i := 0; i < n; i++ {
		t[i] = make([]float64, m)
		for j := 0; j < m; j++ {
			t[i][j] = a[j][i]
		}
	}
	return t
}

func matScale(a [][]float64, scalar float64) [][]float64 {
	m, n := len(a), len(a[0])
	c := make([][]float64, m)
	for i := 0; i < m; i++ {
		c[i] = make([]float64, n)
		for j := 0; j < n; j++ {
			c[i][j] = scalar * a[i][j]
		}
	}
	return c
}

func matTrace(a [][]float64) (float64, error) {
	if len(a) != len(a[0]) {
		return 0, fmt.Errorf("trace requires square matrix")
	}
	var trace float64
	for i := 0; i < len(a); i++ {
		trace += a[i][i]
	}
	return trace, nil
}

func matNormFrobenius(a [][]float64) float64 {
	var sum float64
	for i := 0; i < len(a); i++ {
		for j := 0; j < len(a[0]); j++ {
			sum += a[i][j] * a[i][j]
		}
	}
	return math.Sqrt(sum)
}

func matVecMul(a [][]float64, v []float64, alpha float64) ([]float64, error) {
	if len(a[0]) != len(v) {
		return nil, fmt.Errorf("incompatible dimensions: matrix columns (%d) != vector length (%d)", len(a[0]), len(v))
	}

	result := make([]float64, len(a))
	for i := 0; i < len(a); i++ {
		for j := 0; j < len(v); j++ {
			result[i] += alpha * a[i][j] * v[j]
		}
	}
	return result, nil
}

func vecMatMul(v []float64, a [][]float64, alpha float64) ([]float64, error) {
	if len(v) != len(a) {
		return nil, fmt.Errorf("incompatible dimensions: vector length (%d) != matrix rows (%d)", len(v), len(a))
	}

	result := make([]float64, len(a[0]))
	for j := 0; j < len(a[0]); j++ {
		for i := 0; i < len(v); i++ {
			result[j] += alpha * v[i] * a[i][j]
		}
	}
	return result, nil
}

func matInverse(a [][]float64) ([][]float64, error) {
	n := len(a)
	if n != len(a[0]) {
		return nil, fmt.Errorf("inverse requires square matrix")
	}

	// Create augmented matrix [A|I]
	aug := make([][]float64, n)
	for i := 0; i < n; i++ {
		aug[i] = make([]float64, 2*n)
		for j := 0; j < n; j++ {
			aug[i][j] = a[i][j]
		}
		aug[i][n+i] = 1.0
	}

	// Gauss-Jordan elimination
	for i := 0; i < n; i++ {
		// Find pivot
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(aug[k][i]) > math.Abs(aug[maxRow][i]) {
				maxRow = k
			}
		}

		// Swap rows
		aug[i], aug[maxRow] = aug[maxRow], aug[i]

		// Check for singular matrix
		if math.Abs(aug[i][i]) < 1e-10 {
			return nil, fmt.Errorf("matrix is singular or nearly singular")
		}

		// Scale pivot row
		pivot := aug[i][i]
		for j := 0; j < 2*n; j++ {
			aug[i][j] /= pivot
		}

		// Eliminate column
		for k := 0; k < n; k++ {
			if k != i {
				factor := aug[k][i]
				for j := 0; j < 2*n; j++ {
					aug[k][j] -= factor * aug[i][j]
				}
			}
		}
	}

	// Extract inverse from augmented matrix
	inv := make([][]float64, n)
	for i := 0; i < n; i++ {
		inv[i] = aug[i][n:]
	}

	return inv, nil
}

func matDeterminant(a [][]float64) (float64, error) {
	n := len(a)
	if n != len(a[0]) {
		return 0, fmt.Errorf("determinant requires square matrix")
	}

	// Make a copy
	m := make([][]float64, n)
	for i := 0; i < n; i++ {
		m[i] = make([]float64, n)
		copy(m[i], a[i])
	}

	det := 1.0
	for i := 0; i < n; i++ {
		// Find pivot
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(m[k][i]) > math.Abs(m[maxRow][i]) {
				maxRow = k
			}
		}

		if maxRow != i {
			m[i], m[maxRow] = m[maxRow], m[i]
			det = -det
		}

		if math.Abs(m[i][i]) < 1e-10 {
			return 0, nil
		}

		det *= m[i][i]

		// Eliminate column
		for k := i + 1; k < n; k++ {
			factor := m[k][i] / m[i][i]
			for j := i; j < n; j++ {
				m[k][j] -= factor * m[i][j]
			}
		}
	}

	return det, nil
}
