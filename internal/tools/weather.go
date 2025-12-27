package tools

import (
	"encoding/json"
	"fmt"
)

// WeatherArgs represents the arguments for get_weather tool
type WeatherArgs struct {
	Location string `json:"location"`
	Unit     string `json:"unit,omitempty"`
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
