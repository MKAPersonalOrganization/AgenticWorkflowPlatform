package tools

import (
	"context"

	"school-with-ai/foundation/agent"
)

// simpleTool is a minimal implementation of agent.Tool for simple function definitions.
type simpleTool struct {
	name        string
	description string
	inputSchema map[string]any
}

func (t *simpleTool) Name() string                { return t.name }
func (t *simpleTool) Description() string         { return t.description }
func (t *simpleTool) InputSchema() map[string]any { return t.inputSchema }
func (t *simpleTool) Execute(ctx context.Context, tctx *agent.ToolContext) (*agent.ToolResult, error) {
	// These are tool definitions only - execution happens elsewhere
	return &agent.ToolResult{
		ToolCallID: tctx.ToolCallID,
		Output:     "Tool execution not implemented",
		IsError:    true,
	}, nil
}

// GetAddTool returns the tool definition for addition.
func GetAddTool() agent.Tool {
	return &simpleTool{
		name:        "add",
		description: "Add two numbers (a + b)",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	}
}

// GetSubtractTool returns the tool definition for subtraction.
func GetSubtractTool() agent.Tool {
	return &simpleTool{
		name:        "subtract",
		description: "Subtract two numbers (a - b)",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	}
}

// GetMultiplyTool returns the tool definition for multiplication.
func GetMultiplyTool() agent.Tool {
	return &simpleTool{
		name:        "multiply",
		description: "Multiply two numbers (a * b)",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	}
}

// GetDivideTool returns the tool definition for division.
func GetDivideTool() agent.Tool {
	return &simpleTool{
		name:        "divide",
		description: "Divide two numbers (a / b)",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	}
}

// GetPowerTool returns the tool definition for exponentiation.
func GetPowerTool() agent.Tool {
	return &simpleTool{
		name:        "power",
		description: "Raise a to the power of b (a^b)",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	}
}

// GetSqrtTool returns the tool definition for square root.
func GetSqrtTool() agent.Tool {
	return &simpleTool{
		name:        "sqrt",
		description: "Calculate the square root of a number",
		inputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{"type": "number"},
			},
			"required": []string{"value"},
		},
	}
}

// GetAllTools returns all available tools.
func GetAllTools() []agent.Tool {
	return []agent.Tool{
		GetAddTool(),
		GetSubtractTool(),
		GetMultiplyTool(),
		GetDivideTool(),
		GetPowerTool(),
		GetSqrtTool(),
	}
}
