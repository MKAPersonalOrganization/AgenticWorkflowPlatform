package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// WeatherArgs represents the arguments for get_weather tool
type WeatherArgs struct {
	Location string `json:"location"`
	Unit     string `json:"unit,omitempty"`
}

// WeatherTool implements the Tool interface for weather lookups
type WeatherTool struct{}

// NewWeatherTool creates a new weather tool
func NewWeatherTool() *WeatherTool {
	return &WeatherTool{}
}

func (w *WeatherTool) Name() string {
	return "get_weather"
}

func (w *WeatherTool) Description() string {
	return "Get current weather information for a location"
}

func (w *WeatherTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{
				"type":        "string",
				"description": "The city and state, e.g. San Francisco, CA",
			},
			"unit": map[string]any{
				"type":        "string",
				"description": "Temperature unit (fahrenheit or celsius)",
				"enum":        []string{"fahrenheit", "celsius"},
			},
		},
		"required": []string{"location"},
	}
}

func (w *WeatherTool) Invoke(ctx context.Context, arguments string) (string, error) {
	return ExecuteWeatherTool(arguments)
}

// ExecuteWeatherTool simulates weather lookup
// In production, this would call a real weather API
func ExecuteWeatherTool(argsJSON string) (string, error) {
	var args WeatherArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("failed to parse weather args: %w", err)
	}

	// Simulate weather data (replace with real API in production)
	unit := args.Unit
	if unit == "" {
		unit = "fahrenheit"
	}

	temp := 72
	if unit == "celsius" {
		temp = 22
	}

	result := map[string]any{
		"location":    args.Location,
		"temperature": temp,
		"unit":        unit,
		"condition":   "sunny",
		"humidity":    65,
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal weather result: %w", err)
	}

	return string(resultJSON), nil
}
