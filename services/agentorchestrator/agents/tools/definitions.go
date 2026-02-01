package tools

import (
	"school-with-ai/foundation/agentsdk/llm"
)

// GetAddTool returns the tool definition for addition.
func GetAddTool() llm.AITool {
	return llm.NewAIFunctionTool(
		"add",
		"Add two numbers (a + b)",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	)
}

// GetSubtractTool returns the tool definition for subtraction.
func GetSubtractTool() llm.AITool {
	return llm.NewAIFunctionTool(
		"subtract",
		"Subtract two numbers (a - b)",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	)
}

// GetMultiplyTool returns the tool definition for multiplication.
func GetMultiplyTool() llm.AITool {
	return llm.NewAIFunctionTool(
		"multiply",
		"Multiply two numbers (a * b)",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	)
}

// GetDivideTool returns the tool definition for division.
func GetDivideTool() llm.AITool {
	return llm.NewAIFunctionTool(
		"divide",
		"Divide two numbers (a / b)",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	)
}

// GetPowerTool returns the tool definition for exponentiation.
func GetPowerTool() llm.AITool {
	return llm.NewAIFunctionTool(
		"power",
		"Raise a to the power of b (a^b)",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"a": map[string]any{"type": "number"},
				"b": map[string]any{"type": "number"},
			},
			"required": []string{"a", "b"},
		},
	)
}

// GetSqrtTool returns the tool definition for square root.
func GetSqrtTool() llm.AITool {
	return llm.NewAIFunctionTool(
		"sqrt",
		"Calculate the square root of a number",
		map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{"type": "number"},
			},
			"required": []string{"value"},
		},
	)
}

// GetAllTools returns all available tools.
func GetAllTools() []llm.AITool {
	return []llm.AITool{
		GetAddTool(),
		GetSubtractTool(),
		GetMultiplyTool(),
		GetDivideTool(),
		GetPowerTool(),
		GetSqrtTool(),
	}
}
