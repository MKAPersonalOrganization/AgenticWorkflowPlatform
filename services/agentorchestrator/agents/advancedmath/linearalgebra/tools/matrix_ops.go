package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"school-with-ai/foundation/agentsdk/core"

	"github.com/cpmech/gosl/la"
)

var (
	// ErrMatrix2Required is returned when matrix2 is required but not provided.
	ErrMatrix2Required = errors.New("matrix2 required for operation")

	// ErrMatrixSameDimensions is returned when matrices must have the same dimensions.
	ErrMatrixSameDimensions = errors.New("matrices must have same dimensions")

	// ErrSquareMatrixRequired is returned when a square matrix is required.
	ErrSquareMatrixRequired = errors.New("square matrix required")

	// ErrVectorRequired is returned when a vector is required but not provided.
	ErrVectorRequired = errors.New("vector required for operation")

	// ErrIncompatibleDimensions is returned when matrix/vector dimensions are incompatible.
	ErrIncompatibleDimensions = errors.New("incompatible dimensions")

	// ErrUnknownMatrixOperation is returned when an unknown operation is requested.
	ErrUnknownMatrixOperation = errors.New("unknown operation")
)

// MatrixOperationsTool provides matrix operations.
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

func (t *MatrixOperationsTool) Invoke(ctx context.Context, arguments string) (core.ToolResult, error) {
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
		return core.ToolResult{}, fmt.Errorf("invalid arguments: %w", err)
	}

	// Set defaults
	if args.Alpha == 0 && (args.Operation == "add" || args.Operation == "multiply") {
		args.Alpha = 1.0
	}
	if args.Beta == 0 && args.Operation == "add" {
		args.Beta = 1.0
	}

	var result any

	switch args.Operation {
	case "multiply":
		if len(args.Matrix2) == 0 {
			return core.ToolResult{}, ErrMatrix2Required
		}
		a := la.NewMatrixDeep2(args.Matrix)
		b := la.NewMatrixDeep2(args.Matrix2)
		c := la.NewMatrix(a.M, b.N)
		la.MatMatMul(c, args.Alpha, a, b)
		result = c.GetDeep2()

	case "add":
		if len(args.Matrix2) == 0 {
			return core.ToolResult{}, ErrMatrix2Required
		}
		// gosl doesn't have direct matrix add, so we'll use manual implementation
		a, b := args.Matrix, args.Matrix2
		if len(a) != len(b) || len(a[0]) != len(b[0]) {
			return core.ToolResult{}, ErrMatrixSameDimensions
		}
		m, n := len(a), len(a[0])
		c := make([][]float64, m)
		for i := range m {
			c[i] = make([]float64, n)
			for j := range n {
				c[i][j] = args.Alpha*a[i][j] + args.Beta*b[i][j]
			}
		}
		result = c

	case "subtract":
		if len(args.Matrix2) == 0 {
			return core.ToolResult{}, ErrMatrix2Required
		}
		a, b := args.Matrix, args.Matrix2
		if len(a) != len(b) || len(a[0]) != len(b[0]) {
			return core.ToolResult{}, ErrMatrixSameDimensions
		}
		m, n := len(a), len(a[0])
		c := make([][]float64, m)
		for i := range m {
			c[i] = make([]float64, n)
			for j := range n {
				c[i][j] = a[i][j] - b[i][j]
			}
		}
		result = c

	case "transpose":
		a := la.NewMatrixDeep2(args.Matrix)
		result = a.GetTranspose().GetDeep2()

	case "inverse":
		a := la.NewMatrixDeep2(args.Matrix)
		if a.M != a.N {
			return core.ToolResult{}, ErrSquareMatrixRequired
		}
		ai := la.NewMatrix(a.N, a.M)
		la.MatInv(ai, a, false)
		result = ai.GetDeep2()

	case "determinant":
		a := la.NewMatrixDeep2(args.Matrix)
		if a.M != a.N {
			return core.ToolResult{}, ErrSquareMatrixRequired
		}
		result = a.Det()

	case "trace":
		if len(args.Matrix) != len(args.Matrix[0]) {
			return core.ToolResult{}, ErrSquareMatrixRequired
		}
		var trace float64
		for i := range len(args.Matrix) {
			trace += args.Matrix[i][i]
		}
		result = trace

	case "norm_frobenius":
		a := la.NewMatrixDeep2(args.Matrix)
		result = a.NormFrob()

	case "scale":
		a := la.NewMatrixDeep2(args.Matrix)
		scaled := la.NewMatrix(a.M, a.N)
		scaled.Apply(args.Scalar, a)
		result = scaled.GetDeep2()

	case "matrix_vector_multiply":
		if len(args.Vector) == 0 {
			return core.ToolResult{}, ErrVectorRequired
		}
		a := la.NewMatrixDeep2(args.Matrix)
		u := la.Vector(args.Vector)
		if a.N != len(u) {
			return core.ToolResult{}, fmt.Errorf("%w: matrix columns (%d) != vector length (%d)", ErrIncompatibleDimensions, a.N, len(u))
		}
		v := la.NewVector(a.M)
		la.MatVecMul(v, args.Alpha, a, u)
		result = []float64(v)

	case "vector_matrix_multiply":
		if len(args.Vector) == 0 {
			return core.ToolResult{}, ErrVectorRequired
		}
		a := la.NewMatrixDeep2(args.Matrix)
		u := la.Vector(args.Vector)
		if len(u) != a.M {
			return core.ToolResult{}, fmt.Errorf("%w: vector length (%d) != matrix rows (%d)", ErrIncompatibleDimensions, len(u), a.M)
		}
		v := la.NewVector(a.N)
		la.MatTrVecMul(v, args.Alpha, a, u)
		result = []float64(v)

	default:
		return core.ToolResult{}, fmt.Errorf("%w: %s", ErrUnknownMatrixOperation, args.Operation)
	}

	response := map[string]any{
		"operation": args.Operation,
		"result":    result,
	}

	data, err := json.Marshal(response)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("failed to marshal response: %w", err)
	}

	return core.NewToolResult(string(data)), nil
}
