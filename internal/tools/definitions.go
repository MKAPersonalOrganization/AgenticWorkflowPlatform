package tools

import (
	"github.com/openai/openai-go/v3"
)

// GetWeatherTool returns the OpenAI tool definition for weather lookup
func GetWeatherTool() openai.ChatCompletionToolUnionParam {
	return openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
		Name:        "get_weather",
		Description: openai.String("Get the current weather for a location"),
		Parameters: openai.FunctionParameters{
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
	})
}

// GetAllTools returns all available tools
func GetAllTools() []openai.ChatCompletionToolUnionParam {
	return []openai.ChatCompletionToolUnionParam{
		GetWeatherTool(),
	}
}
