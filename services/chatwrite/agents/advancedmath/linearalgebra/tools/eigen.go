package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cpmech/gosl/la"
)

// ErrEigenSquareMatrix is returned when a square matrix is required for eigenvalue computation.
var ErrEigenSquareMatrix = errors.New("matrix must be square")

// EigenvalueTool computes eigenvalues and eigenvectors.
type EigenvalueTool struct{}

func NewEigenvalueTool() *EigenvalueTool {
	return &EigenvalueTool{}
}

func (t *EigenvalueTool) Name() string {
	return "eigenvalue_solver"
}

func (t *EigenvalueTool) Description() string {
	return "Compute eigenvalues and eigenvectors of matrices using QR algorithm"
}

func (t *EigenvalueTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"matrix": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "array", "items": map[string]string{"type": "number"}},
				"description": "Matrix to analyze (must be square)",
			},
			"compute_vectors": map[string]any{
				"type":        "boolean",
				"description": "Whether to compute eigenvectors (default: false)",
				"default":     false,
			},
			"max_iterations": map[string]any{
				"type":        "integer",
				"description": "Maximum iterations for QR algorithm (default: 1000)",
				"default":     1000,
			},
		},
		"required": []string{"matrix"},
	}
}

func (t *EigenvalueTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Matrix         [][]float64 `json:"matrix"`
		ComputeVectors bool        `json:"compute_vectors,omitempty"`
		MaxIterations  int         `json:"max_iterations,omitempty"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	n := len(args.Matrix)
	if n == 0 || len(args.Matrix[0]) != n {
		return "", ErrEigenSquareMatrix
	}

	A := la.NewMatrixDeep2(args.Matrix)

	// Compute eigenvalues using gosl
	var eigenvaluesComplex la.VectorC
	var eigenvectors *la.MatrixC

	if args.ComputeVectors {
		// Compute eigenvalues and eigenvectors
		eigenvaluesComplex = la.NewVectorC(n)
		eigenvectors = la.NewMatrixC(n, n)
		la.EigenVecR(eigenvectors, eigenvaluesComplex, A, true) // preserveA = true
	} else {
		// Compute only eigenvalues
		eigenvaluesComplex = la.NewVectorC(n)
		la.EigenVal(eigenvaluesComplex, A, true) // preserveA = true
	}

	// Format eigenvalues for JSON
	eigenvaluesJSON := make([]map[string]float64, len(eigenvaluesComplex))
	for i, λ := range eigenvaluesComplex {
		eigenvaluesJSON[i] = map[string]float64{
			"real": real(λ),
			"imag": imag(λ),
		}
	}

	response := map[string]any{
		"operation":   "eigenvalue_solver",
		"eigenvalues": eigenvaluesJSON,
	}

	if args.ComputeVectors {
		// Convert complex eigenvectors to real/imag pairs for JSON
		eigenvectorsJSON := make([][][]float64, n)
		for i := range n {
			eigenvectorsJSON[i] = make([][]float64, 2)
			eigenvectorsJSON[i][0] = make([]float64, n) // real part
			eigenvectorsJSON[i][1] = make([]float64, n) // imag part
			for j := range n {
				val := eigenvectors.Get(j, i) // column i is eigenvector i
				eigenvectorsJSON[i][0][j] = real(val)
				eigenvectorsJSON[i][1][j] = imag(val)
			}
		}
		response["eigenvectors"] = eigenvectorsJSON
	}

	data, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(data), nil
}
