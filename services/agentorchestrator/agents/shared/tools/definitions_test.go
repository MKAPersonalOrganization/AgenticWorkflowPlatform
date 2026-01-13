package tools

import (
	"testing"
)

// TestGetWeatherTool verifies the weather tool definition structure.
func TestGetWeatherTool(t *testing.T) {
	t.Parallel()
	tool := GetWeatherTool()

	// Check tool name
	if tool.Name() != "get_weather" {
		t.Errorf("Expected name 'get_weather', got %v", tool.Name())
	}

	// Check tool has description
	if tool.Description() == "" {
		t.Error("Tool missing description")
	}

	// Check tool has parameters
	if tool.Parameters() == nil {
		t.Fatal("Tool missing parameters")
	}

	// Parameters are already map[string]any
	params := tool.Parameters()

	// Check parameters type
	if params["type"] != "object" {
		t.Errorf("Expected parameters type 'object', got %v", params["type"])
	}

	// Check properties exist
	properties, ok := params["properties"].(map[string]any)
	if !ok {
		t.Fatalf("Expected properties object, got %T", params["properties"])
	}

	// Check location property
	if _, hasLocation := properties["location"]; !hasLocation {
		t.Error("Properties missing 'location' field")
	}

	// Check unit property
	if _, hasUnit := properties["unit"]; !hasUnit {
		t.Error("Properties missing 'unit' field")
	}

	// Check required fields
	required, ok := params["required"].([]string)
	if !ok {
		t.Fatalf("Expected required array, got %T", params["required"])
	}

	if len(required) != 1 || required[0] != "location" {
		t.Errorf("Expected required=['location'], got %v", required)
	}
}

// TestGetCalculatorTool verifies the calculator tool definition structure.
func TestGetCalculatorTool(t *testing.T) {
	t.Parallel()
	tool := GetCalculatorTool()

	// Check tool name
	if tool.Name() != "calculate" {
		t.Errorf("Expected name 'calculate', got %v", tool.Name())
	}

	// Check tool has description
	if tool.Description() == "" {
		t.Error("Tool missing description")
	}

	// Check tool has parameters
	if tool.Parameters() == nil {
		t.Fatal("Tool missing parameters")
	}
}

// TestGetAllTools verifies all tools are returned.
func TestGetAllTools(t *testing.T) {
	t.Parallel()
	tools := GetAllTools()

	if len(tools) < 2 {
		t.Fatalf("Expected at least 2 tools, got %d", len(tools))
	}

	// Verify we have the weather tool
	foundWeather := false
	foundCalculator := false
	for _, tool := range tools {
		if tool.Name() == "get_weather" {
			foundWeather = true
		}
		if tool.Name() == "calculate" {
			foundCalculator = true
		}
	}

	if !foundWeather {
		t.Error("Weather tool not found in GetAllTools()")
	}
	if !foundCalculator {
		t.Error("Calculator tool not found in GetAllTools()")
	}
}

// TestWeatherToolParametersSchema verifies the JSON schema is valid.
func TestWeatherToolParametersSchema(t *testing.T) {
	t.Parallel()
	tool := GetWeatherTool()

	// Parameters are already map[string]any
	params := tool.Parameters()
	properties := params["properties"].(map[string]any)

	// Check location property schema (it's map[string]string)
	location, ok := properties["location"].(map[string]string)
	if !ok {
		t.Fatalf("Expected location to be map[string]string, got %T", properties["location"])
	}
	if location["type"] != "string" {
		t.Errorf("Expected location type 'string', got %v", location["type"])
	}
	if _, hasDesc := location["description"]; !hasDesc {
		t.Error("Location property missing description")
	}

	// Check unit property schema (it's map[string]any because it has enum)
	unit, ok := properties["unit"].(map[string]any)
	if !ok {
		t.Fatalf("Expected unit to be map[string]any, got %T", properties["unit"])
	}
	if unit["type"] != "string" {
		t.Errorf("Expected unit type 'string', got %v", unit["type"])
	}

	// Check unit has enum values
	enum, ok := unit["enum"].([]string)
	if !ok {
		t.Fatalf("Unit property missing or invalid enum values, got %T", unit["enum"])
	}

	expectedEnums := map[string]bool{"celsius": false, "fahrenheit": false}
	for _, e := range enum {
		if _, exists := expectedEnums[e]; exists {
			expectedEnums[e] = true
		}
	}

	for enumVal, found := range expectedEnums {
		if !found {
			t.Errorf("Expected enum value %q not found", enumVal)
		}
	}
}
