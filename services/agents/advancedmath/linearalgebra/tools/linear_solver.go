package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

// LinearSolverTool solves linear systems
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

func (t *LinearSolverTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args struct {
		A      [][]float64 `json:"A"`
		B      []float64   `json:"b"`
		Method string      `json:"method,omitempty"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Method == "" {
		args.Method = "gauss"
	}

	// Validate dimensions
	n := len(args.A)
	if n == 0 || len(args.A[0]) != n {
		return "", fmt.Errorf("A must be a square matrix")
	}
	if len(args.B) != n {
		return "", fmt.Errorf("b must have same length as A rows")
	}

	var x []float64
	var err error

	switch args.Method {
	case "gauss":
		x, err = solveGaussianElimination(args.A, args.B)
	case "lu":
		x, err = solveLU(args.A, args.B)
	case "cholesky":
		x, err = solveCholesky(args.A, args.B)
	default:
		return "", fmt.Errorf("unknown method: %s", args.Method)
	}

	if err != nil {
		return "", fmt.Errorf("solver failed: %w", err)
	}

	response := map[string]any{
		"operation": "linear_solver",
		"method":    args.Method,
		"solution":  x,
	}

	data, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	return string(data), nil
}

// Gaussian elimination with partial pivoting
func solveGaussianElimination(A [][]float64, b []float64) ([]float64, error) {
	n := len(A)

	// Create augmented matrix [A|b]
	aug := make([][]float64, n)
	for i := 0; i < n; i++ {
		aug[i] = make([]float64, n+1)
		for j := 0; j < n; j++ {
			aug[i][j] = A[i][j]
		}
		aug[i][n] = b[i]
	}

	// Forward elimination
	for i := 0; i < n; i++ {
		// Partial pivoting
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

		// Eliminate column
		for k := i + 1; k < n; k++ {
			factor := aug[k][i] / aug[i][i]
			for j := i; j <= n; j++ {
				aug[k][j] -= factor * aug[i][j]
			}
		}
	}

	// Back substitution
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		x[i] = aug[i][n]
		for j := i + 1; j < n; j++ {
			x[i] -= aug[i][j] * x[j]
		}
		x[i] /= aug[i][i]
	}

	return x, nil
}

// LU decomposition solver
func solveLU(A [][]float64, b []float64) ([]float64, error) {
	n := len(A)

	// Perform LU decomposition
	L, U, P, err := luDecompose(A)
	if err != nil {
		return nil, err
	}

	// Solve Ly = Pb using forward substitution
	Pb := make([]float64, n)
	for i := 0; i < n; i++ {
		Pb[i] = b[P[i]]
	}

	y := make([]float64, n)
	for i := 0; i < n; i++ {
		y[i] = Pb[i]
		for j := 0; j < i; j++ {
			y[i] -= L[i][j] * y[j]
		}
	}

	// Solve Ux = y using back substitution
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		x[i] = y[i]
		for j := i + 1; j < n; j++ {
			x[i] -= U[i][j] * x[j]
		}
		x[i] /= U[i][i]
	}

	return x, nil
}

func luDecompose(A [][]float64) (L, U [][]float64, P []int, err error) {
	n := len(A)

	// Initialize L as identity, U as copy of A
	L = make([][]float64, n)
	U = make([][]float64, n)
	P = make([]int, n)

	for i := 0; i < n; i++ {
		L[i] = make([]float64, n)
		U[i] = make([]float64, n)
		L[i][i] = 1.0
		copy(U[i], A[i])
		P[i] = i
	}

	// Perform LU decomposition with partial pivoting
	for i := 0; i < n; i++ {
		// Find pivot
		maxRow := i
		for k := i + 1; k < n; k++ {
			if math.Abs(U[k][i]) > math.Abs(U[maxRow][i]) {
				maxRow = k
			}
		}

		// Swap rows in U and P
		if maxRow != i {
			U[i], U[maxRow] = U[maxRow], U[i]
			P[i], P[maxRow] = P[maxRow], P[i]
			// Swap already computed parts of L
			for j := 0; j < i; j++ {
				L[i][j], L[maxRow][j] = L[maxRow][j], L[i][j]
			}
		}

		if math.Abs(U[i][i]) < 1e-10 {
			return nil, nil, nil, fmt.Errorf("matrix is singular or nearly singular")
		}

		// Eliminate
		for k := i + 1; k < n; k++ {
			factor := U[k][i] / U[i][i]
			L[k][i] = factor
			for j := i; j < n; j++ {
				U[k][j] -= factor * U[i][j]
			}
		}
	}

	return L, U, P, nil
}

// Cholesky decomposition solver (for symmetric positive-definite matrices)
func solveCholesky(A [][]float64, b []float64) ([]float64, error) {
	n := len(A)

	// Check if matrix is symmetric
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			if math.Abs(A[i][j]-A[j][i]) > 1e-10 {
				return nil, fmt.Errorf("Cholesky decomposition requires symmetric matrix")
			}
		}
	}

	// Perform Cholesky decomposition: A = L * L^T
	L := make([][]float64, n)
	for i := 0; i < n; i++ {
		L[i] = make([]float64, n)
	}

	for j := 0; j < n; j++ {
		for i := j; i < n; i++ {
			sum := A[i][j]
			for k := 0; k < j; k++ {
				sum -= L[i][k] * L[j][k]
			}
			if i == j {
				if sum <= 0.0 {
					return nil, fmt.Errorf("matrix is not positive-definite")
				}
				L[i][j] = math.Sqrt(sum)
			} else {
				L[i][j] = sum / L[j][j]
			}
		}
	}

	// Solve L*y = b (forward substitution)
	y := make([]float64, n)
	for i := 0; i < n; i++ {
		y[i] = b[i]
		for j := 0; j < i; j++ {
			y[i] -= L[i][j] * y[j]
		}
		y[i] /= L[i][i]
	}

	// Solve L^T*x = y (back substitution)
	x := make([]float64, n)
	for i := n - 1; i >= 0; i-- {
		x[i] = y[i]
		for j := i + 1; j < n; j++ {
			x[i] -= L[j][i] * x[j]
		}
		x[i] /= L[i][i]
	}

	return x, nil
}
