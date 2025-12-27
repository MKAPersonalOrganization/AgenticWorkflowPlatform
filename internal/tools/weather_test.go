package tools

import (
	"encoding/json"
	"testing"
)

// TestExecuteWeatherTool verifies weather tool execution
func TestExecuteWeatherTool(t *testing.T) {
	tests := []struct {
		name        string
		argsJSON    string
		wantErr     bool
		checkResult func(t *testing.T, result string)
	}{
		{
			name:     "valid weather request with fahrenheit",
			argsJSON: `{"location":"San Francisco, CA","unit":"fahrenheit"}`,
			wantErr:  false,
			checkResult: func(t *testing.T, result string) {
				var weather map[string]any
				if err := json.Unmarshal([]byte(result), &weather); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}

				if weather["location"] != "San Francisco, CA" {
					t.Errorf("Expected location 'San Francisco, CA', got %v", weather["location"])
				}
				if weather["unit"] != "fahrenheit" {
					t.Errorf("Expected unit 'fahrenheit', got %v", weather["unit"])
				}
				if weather["temperature"] != float64(72) {
					t.Errorf("Expected temperature 72, got %v", weather["temperature"])
				}
				if weather["condition"] != "sunny" {
					t.Errorf("Expected condition 'sunny', got %v", weather["condition"])
				}
			},
		},
		{
			name:     "valid weather request with celsius",
			argsJSON: `{"location":"Tokyo","unit":"celsius"}`,
			wantErr:  false,
			checkResult: func(t *testing.T, result string) {
				var weather map[string]any
				if err := json.Unmarshal([]byte(result), &weather); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}

				if weather["location"] != "Tokyo" {
					t.Errorf("Expected location 'Tokyo', got %v", weather["location"])
				}
				if weather["unit"] != "celsius" {
					t.Errorf("Expected unit 'celsius', got %v", weather["unit"])
				}
				if weather["temperature"] != float64(22) {
					t.Errorf("Expected temperature 22, got %v", weather["temperature"])
				}
			},
		},
		{
			name:     "valid weather request without unit (defaults to fahrenheit)",
			argsJSON: `{"location":"New York"}`,
			wantErr:  false,
			checkResult: func(t *testing.T, result string) {
				var weather map[string]any
				if err := json.Unmarshal([]byte(result), &weather); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}

				if weather["unit"] != "fahrenheit" {
					t.Errorf("Expected default unit 'fahrenheit', got %v", weather["unit"])
				}
				if weather["temperature"] != float64(72) {
					t.Errorf("Expected default temperature 72, got %v", weather["temperature"])
				}
			},
		},
		{
			name:     "invalid JSON",
			argsJSON: `{"location":`,
			wantErr:  true,
		},
		{
			name:     "missing required location field",
			argsJSON: `{"unit":"celsius"}`,
			wantErr:  false, // JSON is valid, just missing location
			checkResult: func(t *testing.T, result string) {
				var weather map[string]any
				if err := json.Unmarshal([]byte(result), &weather); err != nil {
					t.Fatalf("Failed to unmarshal result: %v", err)
				}

				// Location will be empty string
				if weather["location"] != "" {
					t.Errorf("Expected empty location, got %v", weather["location"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExecuteWeatherTool(tt.argsJSON)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

// TestWeatherArgsUnmarshal verifies WeatherArgs struct unmarshaling
func TestWeatherArgsUnmarshal(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		expected WeatherArgs
	}{
		{
			name: "full args",
			json: `{"location":"Paris","unit":"celsius"}`,
			expected: WeatherArgs{
				Location: "Paris",
				Unit:     "celsius",
			},
		},
		{
			name: "location only",
			json: `{"location":"London"}`,
			expected: WeatherArgs{
				Location: "London",
				Unit:     "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var args WeatherArgs
			if err := json.Unmarshal([]byte(tt.json), &args); err != nil {
				t.Fatalf("Failed to unmarshal: %v", err)
			}

			if args.Location != tt.expected.Location {
				t.Errorf("Expected location %q, got %q", tt.expected.Location, args.Location)
			}
			if args.Unit != tt.expected.Unit {
				t.Errorf("Expected unit %q, got %q", tt.expected.Unit, args.Unit)
			}
		})
	}
}

// TestWeatherToolResultFormat verifies result is valid JSON
func TestWeatherToolResultFormat(t *testing.T) {
	argsJSON := `{"location":"Berlin","unit":"celsius"}`
	result, err := ExecuteWeatherTool(argsJSON)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify result is valid JSON
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Errorf("Result is not valid JSON: %v\nResult: %s", err, result)
	}

	// Verify all expected fields are present
	requiredFields := []string{"location", "temperature", "unit", "condition", "humidity"}
	for _, field := range requiredFields {
		if _, exists := parsed[field]; !exists {
			t.Errorf("Result missing required field: %s", field)
		}
	}
}
