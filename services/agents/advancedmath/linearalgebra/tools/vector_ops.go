package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
)

// VectorOperationsTool provides vector operations
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
			return "", fmt.Errorf("vector2 required for add operation")
		}
		if len(args.Vector1) != len(args.Vector2) {
			return "", fmt.Errorf("vectors must have same length")
		}
		result = vecAdd(args.Vector1, args.Vector2, args.Alpha, args.Beta)

	case "subtract":
		if len(args.Vector2) == 0 {
			return "", fmt.Errorf("vector2 required for subtract operation")
		}
		if len(args.Vector1) != len(args.Vector2) {
			return "", fmt.Errorf("vectors must have same length")
		}
		result = vecAdd(args.Vector1, args.Vector2, 1.0, -1.0)

	case "scale":
		result = vecScale(args.Vector1, args.Scalar)

	case "dot":
		if len(args.Vector2) == 0 {
			return "", fmt.Errorf("vector2 required for dot product")
		}
		if len(args.Vector1) != len(args.Vector2) {
			return "", fmt.Errorf("vectors must have same length")
		}
		result = vecDot(args.Vector1, args.Vector2)

	case "cross":
		if len(args.Vector2) == 0 {
			return "", fmt.Errorf("vector2 required for cross product")
		}
		if len(args.Vector1) != 3 || len(args.Vector2) != 3 {
			return "", fmt.Errorf("cross product requires 3D vectors")
		}
		result = vecCross(args.Vector1, args.Vector2)

	case "norm":
		result = vecNorm(args.Vector1)

	case "unit":
		result = vecUnit(args.Vector1)

	case "max":
		result = vecMax(args.Vector1)

	case "min":
		result = vecMin(args.Vector1)

	case "accum":
		result = vecAccum(args.Vector1)

	case "rms":
		result = vecRms(args.Vector1)

	case "norm_diff":
		if len(args.Vector2) == 0 {
			return "", fmt.Errorf("vector2 required for norm_diff operation")
		}
		if len(args.Vector1) != len(args.Vector2) {
			return "", fmt.Errorf("vectors must have same length")
		}
		result = vecNormDiff(args.Vector1, args.Vector2)

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

// Vector operation implementations (based on gosl/la)

func vecAdd(u, v []float64, alpha, beta float64) []float64 {
	res := make([]float64, len(u))
	for i := 0; i < len(u); i++ {
		res[i] = alpha*u[i] + beta*v[i]
	}
	return res
}

func vecScale(v []float64, scalar float64) []float64 {
	res := make([]float64, len(v))
	for i := 0; i < len(v); i++ {
		res[i] = scalar * v[i]
	}
	return res
}

func vecDot(u, v []float64) float64 {
	var res float64
	for i := 0; i < len(u); i++ {
		res += u[i] * v[i]
	}
	return res
}

func vecCross(u, v []float64) []float64 {
	return []float64{
		u[1]*v[2] - u[2]*v[1],
		u[2]*v[0] - u[0]*v[2],
		u[0]*v[1] - u[1]*v[0],
	}
}

func vecNorm(v []float64) float64 {
	return math.Sqrt(vecDot(v, v))
}

func vecUnit(v []float64) []float64 {
	norm := vecNorm(v)
	if norm == 0 {
		return v
	}
	return vecScale(v, 1.0/norm)
}

func vecMax(v []float64) float64 {
	max := v[0]
	for i := 1; i < len(v); i++ {
		if v[i] > max {
			max = v[i]
		}
	}
	return max
}

func vecMin(v []float64) float64 {
	min := v[0]
	for i := 1; i < len(v); i++ {
		if v[i] < min {
			min = v[i]
		}
	}
	return min
}

func vecAccum(v []float64) float64 {
	var sum float64
	for i := 0; i < len(v); i++ {
		sum += v[i]
	}
	return sum
}

func vecRms(v []float64) float64 {
	var rms float64
	for i := 0; i < len(v); i++ {
		rms += v[i] * v[i]
	}
	return math.Sqrt(rms / float64(len(v)))
}

func vecNormDiff(u, v []float64) float64 {
	var diff float64
	for i := 0; i < len(u); i++ {
		d := u[i] - v[i]
		diff += d * d
	}
	return math.Sqrt(diff)
}
