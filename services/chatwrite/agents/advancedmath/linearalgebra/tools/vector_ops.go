package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cpmech/gosl/la"
)

// VectorOperationsTool provides vector operations.
type VectorOperationsTool struct{}

func NewVectorOperationsTool() *VectorOperationsTool {
	return &VectorOperationsTool{}
}

func (t *VectorOperationsTool) Name() string {
	return "vector_operations"
}

func (t *VectorOperationsTool) Description() string {
	return "Perform vector operations: add, subtract, scale, dot product, cross product, norm, unit vector, max, min"
}

func (t *VectorOperationsTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"operation": map[string]any{
				"type": "string",
				"enum": []string{
					"add", "subtract", "scale", "dot", "cross", "norm",
					"unit", "max", "min", "accum", "rms", "norm_diff",
				},
				"description": "Vector operation to perform",
			},
			"vector1": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "First vector",
			},
			"vector2": map[string]any{
				"type":        "array",
				"items":       map[string]string{"type": "number"},
				"description": "Second vector (for binary operations)",
			},
			"alpha": map[string]any{
				"type":        "number",
				"description": "Alpha scalar coefficient (for add operation: alpha*v1 + beta*v2)",
				"default":     1.0,
			},
			"beta": map[string]any{
				"type":        "number",
				"description": "Beta scalar coefficient (for add operation)",
				"default":     1.0,
			},
			"scalar": map[string]any{
				"type":        "number",
				"description": "Scalar value (for scale operation)",
			},
		},
		"required": []string{"operation", "vector1"},
	}
}

func (t *VectorOperationsTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Operation string    `json:"operation"`
		Vector1   []float64 `json:"vector1"`
		Vector2   []float64 `json:"vector2,omitempty"`
		Alpha     float64   `json:"alpha,omitempty"`
		Beta      float64   `json:"beta,omitempty"`
		Scalar    float64   `json:"scalar,omitempty"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Set default values
	if args.Alpha == 0 && args.Operation == "add" {
		args.Alpha = 1.0
	}
	if args.Beta == 0 && args.Operation == "add" {
		args.Beta = 1.0
	}

	var result any
	var err error

	switch args.Operation {
	case "add":
		if len(args.Vector2) == 0 {
			return "", errors.New("vector2 required for add operation")
		}
		if len(args.Vector1) != len(args.Vector2) {
			return "", errors.New("vectors must have same length")
		}
		res := la.NewVector(len(args.Vector1))
		la.VecAdd(res, args.Alpha, la.Vector(args.Vector1), args.Beta, la.Vector(args.Vector2))
		result = []float64(res)

	case "subtract":
		if len(args.Vector2) == 0 {
			return "", errors.New("vector2 required for subtract operation")
		}
		if len(args.Vector1) != len(args.Vector2) {
			return "", errors.New("vectors must have same length")
		}
		res := la.NewVector(len(args.Vector1))
		la.VecAdd(res, 1.0, la.Vector(args.Vector1), -1.0, la.Vector(args.Vector2))
		result = []float64(res)

	case "scale":
		v := la.Vector(args.Vector1)
		res := la.NewVector(len(v))
		res.Apply(args.Scalar, v)
		result = []float64(res)

	case "dot":
		if len(args.Vector2) == 0 {
			return "", errors.New("vector2 required for dot product")
		}
		if len(args.Vector1) != len(args.Vector2) {
			return "", errors.New("vectors must have same length")
		}
		result = la.VecDot(la.Vector(args.Vector1), la.Vector(args.Vector2))

	case "cross":
		if len(args.Vector2) == 0 {
			return "", errors.New("vector2 required for cross product")
		}
		if len(args.Vector1) != 3 || len(args.Vector2) != 3 {
			return "", errors.New("cross product requires 3D vectors")
		}
		// gosl doesn't have cross product, use standard formula
		u, v := args.Vector1, args.Vector2
		result = []float64{
			u[1]*v[2] - u[2]*v[1],
			u[2]*v[0] - u[0]*v[2],
			u[0]*v[1] - u[1]*v[0],
		}

	case "norm":
		result = la.Vector(args.Vector1).Norm()

	case "unit":
		v := la.Vector(args.Vector1)
		result = []float64(v.GetUnit())

	case "max":
		result = la.Vector(args.Vector1).Max()

	case "min":
		result = la.Vector(args.Vector1).Min()

	case "accum":
		result = la.Vector(args.Vector1).Accum()

	case "rms":
		result = la.Vector(args.Vector1).Rms()

	case "norm_diff":
		if len(args.Vector2) == 0 {
			return "", errors.New("vector2 required for norm_diff operation")
		}
		if len(args.Vector1) != len(args.Vector2) {
			return "", errors.New("vectors must have same length")
		}
		result = la.Vector(args.Vector1).NormDiff(la.Vector(args.Vector2))

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
