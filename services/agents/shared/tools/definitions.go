package tools

import (
	"school-with-ai/services/llm"
)

// GetWeatherTool returns the tool definition for weather lookup
func GetWeatherTool() llm.Tool {
	return llm.Tool{
		Type:        "function",
		Name:        "get_weather",
		Description: "Get the current weather for a location",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"location": map[string]string{
					"type":        "string",
					"description": "City name or location (e.g., 'San Francisco, CA')",
				},
				"unit": map[string]any{
					"type":        "string",
					"enum":        []string{"celsius", "fahrenheit"},
					"description": "Temperature unit",
				},
			},
			"required": []string{"location"},
		},
	}
}

// GetCalculatorTool returns the tool definition for calculator
func GetCalculatorTool() llm.Tool {
	return llm.Tool{
		Type:        "function",
		Name:        "calculate",
		Description: "Perform mathematical calculations. Supports basic arithmetic (+, -, *, /), exponentiation (^), square root (sqrt), and parentheses.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"expression": map[string]string{
					"type":        "string",
					"description": "Mathematical expression to evaluate (e.g., '2 + 2', '15 * 3', 'sqrt(16)', '2^8')",
				},
			},
			"required": []string{"expression"},
		},
	}
}

// GetAllTools returns all available tools
func GetAllTools() []llm.Tool {
	return []llm.Tool{
		GetWeatherTool(),
		GetCalculatorTool(),
	}
}
