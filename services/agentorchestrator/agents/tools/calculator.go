package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"school-with-ai/foundation/agent"
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

func jsonResult(v any) (*agent.ToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return &agent.ToolResult{Output: string(b)}, nil
}

// AddTool performs addition.
type AddTool struct{}

func (t *AddTool) Name() string { return "add" }
func (t *AddTool) Description() string {
	return "Add two numbers (a + b)"
}

func (t *AddTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *AddTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params BinaryOperationArgs
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("failed to parse add args: %w", err)
	}
	return jsonResult(map[string]any{"result": params.A + params.B})
}

// SubtractTool performs subtraction.
type SubtractTool struct{}

func (t *SubtractTool) Name() string { return "subtract" }
func (t *SubtractTool) Description() string {
	return "Subtract two numbers (a - b)"
}

func (t *SubtractTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *SubtractTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params BinaryOperationArgs
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("failed to parse subtract args: %w", err)
	}
	return jsonResult(map[string]any{"result": params.A - params.B})
}

// MultiplyTool performs multiplication.
type MultiplyTool struct{}

func (t *MultiplyTool) Name() string { return "multiply" }
func (t *MultiplyTool) Description() string {
	return "Multiply two numbers (a * b)"
}

func (t *MultiplyTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *MultiplyTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params BinaryOperationArgs
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("failed to parse multiply args: %w", err)
	}
	return jsonResult(map[string]any{"result": params.A * params.B})
}

// DivideTool performs division.
type DivideTool struct{}

func (t *DivideTool) Name() string { return "divide" }
func (t *DivideTool) Description() string {
	return "Divide two numbers (a / b)"
}

func (t *DivideTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *DivideTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params BinaryOperationArgs
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("failed to parse divide args: %w", err)
	}
	if params.B == 0 {
		return nil, fmt.Errorf("failed to divide %v by %v: %w", params.A, params.B, ErrDivisionByZero)
	}
	return jsonResult(map[string]any{"result": params.A / params.B})
}

// PowerTool performs exponentiation.
type PowerTool struct{}

func (t *PowerTool) Name() string { return "power" }
func (t *PowerTool) Description() string {
	return "Raise a to the power of b (a^b)"
}

func (t *PowerTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"a": map[string]any{"type": "number"},
			"b": map[string]any{"type": "number"},
		},
		"required": []string{"a", "b"},
	}
}

func (t *PowerTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params BinaryOperationArgs
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("failed to parse power args: %w", err)
	}
	return jsonResult(map[string]any{"result": math.Pow(params.A, params.B)})
}

// SqrtTool performs square root.
type SqrtTool struct{}

func (t *SqrtTool) Name() string { return "sqrt" }
func (t *SqrtTool) Description() string {
	return "Calculate the square root of a number"
}

func (t *SqrtTool) InputSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"value": map[string]any{"type": "number"},
		},
		"required": []string{"value"},
	}
}

func (t *SqrtTool) Execute(ctx context.Context, tCtx *agent.ToolContext) (*agent.ToolResult, error) {
	var params UnaryOperationArgs
	if err := agent.UnmarshalArgs(tCtx.Arguments, &params); err != nil {
		return nil, fmt.Errorf("failed to parse sqrt args: %w", err)
	}
	if params.Value < 0 {
		return nil, fmt.Errorf("failed to calculate sqrt of %v: %w", params.Value, ErrSqrtNegative)
	}
	return jsonResult(map[string]any{"result": math.Sqrt(params.Value)})
}
