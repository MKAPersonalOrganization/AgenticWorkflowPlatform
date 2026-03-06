package calculator

import (
	"context"
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

// Common schemas for calculator tools
var binaryOpSchema = &agent.Schema{
	Type: agent.TypeObject,
	Properties: map[string]*agent.Schema{
		"a": {Type: agent.TypeNumber},
		"b": {Type: agent.TypeNumber},
	},
	Required: []string{"a", "b"},
}

var unaryOpSchema = &agent.Schema{
	Type: agent.TypeObject,
	Properties: map[string]*agent.Schema{
		"value": {Type: agent.TypeNumber},
	},
	Required: []string{"value"},
}

// AddTool performs addition.
type AddTool struct{}

func (t *AddTool) Name() string          { return "add" }
func (t *AddTool) Description() string   { return "Add two numbers (a + b)" }
func (t *AddTool) Schema() *agent.Schema { return binaryOpSchema }

func (t *AddTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params BinaryOperationArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse add args: %w", err)), nil
	}
	return agent.JSONResult(map[string]any{"result": params.A + params.B}), nil
}

// SubtractTool performs subtraction.
type SubtractTool struct{}

func (t *SubtractTool) Name() string          { return "subtract" }
func (t *SubtractTool) Description() string   { return "Subtract two numbers (a - b)" }
func (t *SubtractTool) Schema() *agent.Schema { return binaryOpSchema }

func (t *SubtractTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params BinaryOperationArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse subtract args: %w", err)), nil
	}
	return agent.JSONResult(map[string]any{"result": params.A - params.B}), nil
}

// MultiplyTool performs multiplication.
type MultiplyTool struct{}

func (t *MultiplyTool) Name() string          { return "multiply" }
func (t *MultiplyTool) Description() string   { return "Multiply two numbers (a * b)" }
func (t *MultiplyTool) Schema() *agent.Schema { return binaryOpSchema }

func (t *MultiplyTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params BinaryOperationArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse multiply args: %w", err)), nil
	}
	return agent.JSONResult(map[string]any{"result": params.A * params.B}), nil
}

// DivideTool performs division.
type DivideTool struct{}

func (t *DivideTool) Name() string          { return "divide" }
func (t *DivideTool) Description() string   { return "Divide two numbers (a / b)" }
func (t *DivideTool) Schema() *agent.Schema { return binaryOpSchema }

func (t *DivideTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params BinaryOperationArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse divide args: %w", err)), nil
	}
	if params.B == 0 {
		return agent.ErrorResult(fmt.Errorf("failed to divide %v by %v: %w", params.A, params.B, ErrDivisionByZero)), nil
	}
	return agent.JSONResult(map[string]any{"result": params.A / params.B}), nil
}

// PowerTool performs exponentiation.
type PowerTool struct{}

func (t *PowerTool) Name() string          { return "power" }
func (t *PowerTool) Description() string   { return "Raise a to the power of b (a^b)" }
func (t *PowerTool) Schema() *agent.Schema { return binaryOpSchema }

func (t *PowerTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params BinaryOperationArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse power args: %w", err)), nil
	}
	return agent.JSONResult(map[string]any{"result": math.Pow(params.A, params.B)}), nil
}

// SqrtTool performs square root.
type SqrtTool struct{}

func (t *SqrtTool) Name() string          { return "sqrt" }
func (t *SqrtTool) Description() string   { return "Calculate the square root of a number" }
func (t *SqrtTool) Schema() *agent.Schema { return unaryOpSchema }

func (t *SqrtTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params UnaryOperationArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse sqrt args: %w", err)), nil
	}
	if params.Value < 0 {
		return agent.ErrorResult(fmt.Errorf("failed to calculate sqrt of %v: %w", params.Value, ErrSqrtNegative)), nil
	}
	return agent.JSONResult(map[string]any{"result": math.Sqrt(params.Value)}), nil
}
