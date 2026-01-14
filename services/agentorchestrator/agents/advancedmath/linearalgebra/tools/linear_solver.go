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
	// ErrLinearSquareMatrix is returned when matrix A must be square.
	ErrLinearSquareMatrix = errors.New("matrix A must be a square matrix")

	// ErrLinearDimensionMismatch is returned when vector b doesn't match A dimensions.
	ErrLinearDimensionMismatch = errors.New("vector b must have same length as A rows")

	// ErrUnknownMethod is returned when an unknown solver method is requested.
	ErrUnknownMethod = errors.New("unknown method")
)

// LinearSolverTool solves linear systems.
type LinearSolverTool struct{}

func NewLinearSolverTool() *LinearSolverTool {
	return &LinearSolverTool{}
}

func (t *LinearSolverTool) Name() string {
	return "linear_solver"
}

func (t *LinearSolverTool) Description() string {
	return "Solve linear systems Ax = b using Gaussian elimination or LU decomposition"
}

func (t *LinearSolverTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"A": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "array", "items": map[string]string{"type": "number"}},
				"description": "Coefficient matrix A",
			},
			"b": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Right-hand side vector b",
			},
			"method": map[string]any{
				"type":        "string",
				"enum":        []string{"gauss", "lu", "cholesky"},
				"description": "Solution method (default: gauss)",
				"default":     "gauss",
			},
		},
		"required": []string{"A", "b"},
	}
}

func (t *LinearSolverTool) Invoke(ctx context.Context, arguments string) (core.ToolResult, error) {
	var args struct {
		A      [][]float64 `json:"A"`
		B      []float64   `json:"b"`
		Method string      `json:"method,omitempty"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return core.ToolResult{}, fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Method == "" {
		args.Method = "gauss"
	}

	// Validate dimensions
	n := len(args.A)
	if n == 0 || len(args.A[0]) != n {
		return core.ToolResult{}, ErrLinearSquareMatrix
	}
	if len(args.B) != n {
		return core.ToolResult{}, ErrLinearDimensionMismatch
	}

	var x []float64

	A_mat := la.NewMatrixDeep2(args.A)
	b_vec := la.Vector(args.B)

	switch args.Method {
	case "gauss", "lu":
		// Use DenSolve which uses LAPACK (more efficient than custom Gaussian elimination)
		x = la.NewVector(len(args.B))
		la.DenSolve(x, A_mat, b_vec, true) // preserveA = true to keep original matrix intact

	case "cholesky":
		// Use Cholesky solver for symmetric positive-definite matrices
		x = la.NewVector(len(args.B))
		la.SolveRealLinSysSPD(x, A_mat, b_vec)

	default:
		return core.ToolResult{}, fmt.Errorf("%w: %s", ErrUnknownMethod, args.Method)
	}

	response := map[string]any{
		"operation": "linear_solver",
		"method":    args.Method,
		"solution":  []float64(x),
	}

	data, err := json.Marshal(response)
	if err != nil {
		return core.ToolResult{}, fmt.Errorf("failed to marshal response: %w", err)
	}

	return core.NewToolResult(string(data)), nil
}
