package tools

import (
	"errors"
	"fmt"
	"math"

	"school-with-ai/foundation/agentsdk/core"
)

var (
	// ErrDivisionByZero is returned when dividing by zero.
	ErrDivisionByZero = errors.New("division by zero")

	// ErrSqrtNegative is returned when taking square root of negative number.
	ErrSqrtNegative = errors.New("square root of negative number")
)

// BinaryOperationArgs represents arguments for binary operations (two numbers).
type BinaryOperationArgs struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
}

// UnaryOperationArgs represents arguments for unary operations (one number).
type UnaryOperationArgs struct {
	Value float64 `json:"value"`
}

// AddTool performs addition.
type AddTool struct{}

func (t *AddTool) Name() string { return "add" }
func (t *AddTool) Description() string {
	return "Add two numbers (a + b)"
}

func (t *AddTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *AddTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params BinaryOperationArgs
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse add args: %w", err)
	}
	return map[string]any{"result": params.A + params.B}, nil
}

// SubtractTool performs subtraction.
type SubtractTool struct{}

func (t *SubtractTool) Name() string { return "subtract" }
func (t *SubtractTool) Description() string {
	return "Subtract two numbers (a - b)"
}

func (t *SubtractTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *SubtractTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params BinaryOperationArgs
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse subtract args: %w", err)
	}
	return map[string]any{"result": params.A - params.B}, nil
}

// MultiplyTool performs multiplication.
type MultiplyTool struct{}

func (t *MultiplyTool) Name() string { return "multiply" }
func (t *MultiplyTool) Description() string {
	return "Multiply two numbers (a * b)"
}

func (t *MultiplyTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *MultiplyTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params BinaryOperationArgs
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse multiply args: %w", err)
	}
	return map[string]any{"result": params.A * params.B}, nil
}

// DivideTool performs division.
type DivideTool struct{}

func (t *DivideTool) Name() string { return "divide" }
func (t *DivideTool) Description() string {
	return "Divide two numbers (a / b)"
}

func (t *DivideTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *DivideTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params BinaryOperationArgs
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse divide args: %w", err)
	}
	if params.B == 0 {
		return nil, fmt.Errorf("failed to divide %v by %v: %w", params.A, params.B, ErrDivisionByZero)
	}
	return map[string]any{"result": params.A / params.B}, nil
}

// PowerTool performs exponentiation.
type PowerTool struct{}

func (t *PowerTool) Name() string { return "power" }
func (t *PowerTool) Description() string {
	return "Raise a to the power of b (a^b)"
}

func (t *PowerTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *PowerTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params BinaryOperationArgs
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse power args: %w", err)
	}
	return map[string]any{"result": math.Pow(params.A, params.B)}, nil
}

// SqrtTool performs square root.
type SqrtTool struct{}

func (t *SqrtTool) Name() string { return "sqrt" }
func (t *SqrtTool) Description() string {
	return "Calculate the square root of a number"
}

func (t *SqrtTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{"type": "number"},
		},
		"required": []string{"value"},
	}
}

func (t *SqrtTool) Invoke(ctx core.ToolContext, args any) (any, error) {
	var params UnaryOperationArgs
	if err := core.UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse sqrt args: %w", err)
	}
	if params.Value < 0 {
		return nil, fmt.Errorf("failed to calculate sqrt of %v: %w", params.Value, ErrSqrtNegative)
	}
	return map[string]any{"result": math.Sqrt(params.Value)}, nil
}
