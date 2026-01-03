package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/cmplx"
)

// EigenvalueTool computes eigenvalues and eigenvectors
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
		return "", fmt.Errorf("matrix must be square")
	}

	if args.MaxIterations == 0 {
		args.MaxIterations = 1000
	}

	// For symmetric matrices, use simpler algorithm
	isSymmetric := true
	for i := 0; i < n && isSymmetric; i++ {
		for j := i + 1; j < n; j++ {
			if math.Abs(args.Matrix[i][j]-args.Matrix[j][i]) > 1e-10 {
				isSymmetric = false
				break
			}
		}
	}

	var eigenvalues []complex128
	var eigenvectors [][][]float64
	var err error

	if isSymmetric {
		// Use Jacobi method for symmetric matrices
		realEigenvalues, realEigenvectors, err2 := jacobiEigenSymmetric(args.Matrix, args.ComputeVectors, args.MaxIterations)
		if err2 != nil {
			return "", err2
		}
		// Convert to complex
		eigenvalues = make([]complex128, len(realEigenvalues))
		for i, v := range realEigenvalues {
			eigenvalues[i] = complex(v, 0)
		}
		eigenvectors = realEigenvectors
	} else {
		// Use QR algorithm for general matrices
		eigenvalues, eigenvectors, err = qrEigenvalues(args.Matrix, args.ComputeVectors, args.MaxIterations)
		if err != nil {
			return "", err
		}
	}

	// Format eigenvalues for JSON
	eigenvaluesJSON := make([]map[string]float64, len(eigenvalues))
	for i, λ := range eigenvalues {
		eigenvaluesJSON[i] = map[string]float64{
			"real": real(λ),
			"imag": imag(λ),
		}
	}

	response := map[string]any{
		"operation":    "eigenvalue_solver",
		"eigenvalues":  eigenvaluesJSON,
		"is_symmetric": isSymmetric,
	}

	if args.ComputeVectors {
		response["eigenvectors"] = eigenvectors
	}

	data, err := json.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}
	return string(data), nil
}

// Jacobi method for symmetric matrices
func jacobiEigenSymmetric(A [][]float64, computeVectors bool, maxIter int) ([]float64, [][][]float64, error) {
	n := len(A)

	// Copy matrix
	a := make([][]float64, n)
	for i := 0; i < n; i++ {
		a[i] = make([]float64, n)
		copy(a[i], A[i])
	}

	// Initialize eigenvector matrix as identity
	var v [][]float64
	if computeVectors {
		v = make([][]float64, n)
		for i := 0; i < n; i++ {
			v[i] = make([]float64, n)
			v[i][i] = 1.0
		}
	}

	// Jacobi rotation
	for iter := 0; iter < maxIter; iter++ {
		// Find largest off-diagonal element
		var p, q int
		max := 0.0
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				if math.Abs(a[i][j]) > max {
					max = math.Abs(a[i][j])
					p, q = i, j
				}
			}
		}

		// Check convergence
		if max < 1e-10 {
			break
		}

		// Compute rotation angle
		theta := 0.5 * math.Atan2(2*a[p][q], a[q][q]-a[p][p])
		c := math.Cos(theta)
		s := math.Sin(theta)

		// Apply rotation to a
		for i := 0; i < n; i++ {
			if i != p && i != q {
				temp1 := c*a[i][p] - s*a[i][q]
				temp2 := s*a[i][p] + c*a[i][q]
				a[i][p] = temp1
				a[p][i] = temp1
				a[i][q] = temp2
				a[q][i] = temp2
			}
		}

		temp1 := c*a[p][p] - s*a[p][q]
		temp2 := s*a[p][p] + c*a[p][q]
		a[p][q] = 0
		a[q][p] = 0
		a[p][p] = c*temp1 - s*temp2
		a[q][q] = s*temp1 + c*temp2

		// Apply rotation to eigenvectors
		if computeVectors {
			for i := 0; i < n; i++ {
				temp1 := c*v[i][p] - s*v[i][q]
				temp2 := s*v[i][p] + c*v[i][q]
				v[i][p] = temp1
				v[i][q] = temp2
			}
		}
	}

	// Extract eigenvalues from diagonal
	eigenvalues := make([]float64, n)
	for i := 0; i < n; i++ {
		eigenvalues[i] = a[i][i]
	}

	// Extract eigenvectors
	var eigenvectors [][][]float64
	if computeVectors {
		eigenvectors = make([][][]float64, n)
		for i := 0; i < n; i++ {
			eigenvectors[i] = make([][]float64, n)
			for j := 0; j < n; j++ {
				eigenvectors[i][j] = []float64{v[j][i]}
			}
		}
	}

	return eigenvalues, eigenvectors, nil
}

// QR algorithm for general matrices (simplified version)
func qrEigenvalues(A [][]float64, computeVectors bool, maxIter int) ([]complex128, [][][]float64, error) {
	n := len(A)

	// First, reduce to upper Hessenberg form (simplified - just use the matrix as-is for now)
	a := make([][]float64, n)
	for i := 0; i < n; i++ {
		a[i] = make([]float64, n)
		copy(a[i], A[i])
	}

	// QR iteration
	for iter := 0; iter < maxIter; iter++ {
		// Check if already upper triangular
		converged := true
		for i := 1; i < n; i++ {
			for j := 0; j < i; j++ {
				if math.Abs(a[i][j]) > 1e-10 {
					converged = false
					break
				}
			}
			if !converged {
				break
			}
		}

		if converged {
			break
		}

		// QR decomposition
		Q, R := qrDecomposition(a)

		// a = R * Q
		a, _ = matMul(R, Q, 1.0)
	}

	// Extract eigenvalues from diagonal
	eigenvalues := make([]complex128, n)
	for i := 0; i < n-1; i++ {
		// Check for 2x2 block (complex eigenvalues)
		if math.Abs(a[i+1][i]) > 1e-10 {
			// Solve 2x2 eigenvalue problem
			subMat := [][]float64{
				{a[i][i], a[i][i+1]},
				{a[i+1][i], a[i+1][i+1]},
			}
			λ1, λ2 := eigenvalues2x2(subMat)
			eigenvalues[i] = λ1
			eigenvalues[i+1] = λ2
			i++ // Skip next diagonal element
		} else {
			eigenvalues[i] = complex(a[i][i], 0)
		}
	}
	if n > 0 && math.Abs(a[n-1][n-2]) <= 1e-10 {
		eigenvalues[n-1] = complex(a[n-1][n-1], 0)
	}

	return eigenvalues, nil, nil
}

// QR decomposition using Gram-Schmidt
func qrDecomposition(A [][]float64) (Q, R [][]float64) {
	m, n := len(A), len(A[0])
	Q = make([][]float64, m)
	R = make([][]float64, n)

	for i := 0; i < m; i++ {
		Q[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		R[i] = make([]float64, n)
	}

	for j := 0; j < n; j++ {
		// Get column j of A
		v := make([]float64, m)
		for i := 0; i < m; i++ {
			v[i] = A[i][j]
		}

		// Orthogonalize against previous columns
		for k := 0; k < j; k++ {
			qk := make([]float64, m)
			for i := 0; i < m; i++ {
				qk[i] = Q[i][k]
			}
			R[k][j] = vecDot(qk, v)
			for i := 0; i < m; i++ {
				v[i] -= R[k][j] * qk[i]
			}
		}

		// Normalize
		norm := vecNorm(v)
		R[j][j] = norm
		if norm > 1e-10 {
			for i := 0; i < m; i++ {
				Q[i][j] = v[i] / norm
			}
		}
	}

	return Q, R
}

// Solve 2x2 eigenvalue problem
func eigenvalues2x2(A [][]float64) (complex128, complex128) {
	a, b := A[0][0], A[0][1]
	c, d := A[1][0], A[1][1]

	trace := a + d
	det := a*d - b*c
	discriminant := trace*trace - 4*det

	if discriminant >= 0 {
		sqrt := math.Sqrt(discriminant)
		λ1 := complex((trace+sqrt)/2, 0)
		λ2 := complex((trace-sqrt)/2, 0)
		return λ1, λ2
	} else {
		real := trace / 2
		imag := math.Sqrt(-discriminant) / 2
		λ1 := complex(real, imag)
		λ2 := cmplx.Conj(λ1)
		return λ1, λ2
	}
}
