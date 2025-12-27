package tools

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go/v3"
)

// TestGetWeatherTool verifies the weather tool definition structure
func TestGetWeatherTool(t *testing.T) {
	tool := GetWeatherTool()

	// Verify tool can be marshaled to JSON
	toolJSON, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal tool: %v", err)
	}

	// Verify JSON structure
	var toolMap map[string]any
	if err := json.Unmarshal(toolJSON, &toolMap); err != nil {
		t.Fatalf("Failed to unmarshal tool JSON: %v", err)
	}

	// Check tool type
	if toolMap["type"] != "function" {
		t.Errorf("Expected type 'function', got %v", toolMap["type"])
	}

	// Check function definition exists
	function, ok := toolMap["function"].(map[string]any)
	if !ok {
		t.Fatalf("Expected function definition, got %T", toolMap["function"])
	}

	// Check function name
	if function["name"] != "get_weather" {
		t.Errorf("Expected name 'get_weather', got %v", function["name"])
	}

	// Check function has description
	if _, hasDesc := function["description"]; !hasDesc {
		t.Error("Function missing description")
	}

	// Check function has parameters
	params, ok := function["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("Expected parameters object, got %T", function["parameters"])
	}

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
	required, ok := params["required"].([]any)
	if !ok {
		t.Fatalf("Expected required array, got %T", params["required"])
	}

	if len(required) != 1 || required[0] != "location" {
		t.Errorf("Expected required=['location'], got %v", required)
	}
}

// TestGetAllTools verifies all tools are returned
func TestGetAllTools(t *testing.T) {
	tools := GetAllTools()

	if len(tools) == 0 {
		t.Fatal("Expected at least one tool, got none")
	}

	// Verify we have the weather tool
	foundWeather := false
	for _, tool := range tools {
		toolJSON, err := json.Marshal(tool)
		if err != nil {
			t.Errorf("Failed to marshal tool: %v", err)
			continue
		}

		var toolMap map[string]any
		if err := json.Unmarshal(toolJSON, &toolMap); err != nil {
			t.Errorf("Failed to unmarshal tool JSON: %v", err)
			continue
		}

		if function, ok := toolMap["function"].(map[string]any); ok {
			if function["name"] == "get_weather" {
				foundWeather = true
				break
			}
		}
	}

	if !foundWeather {
		t.Error("Weather tool not found in GetAllTools()")
	}
}

// TestToolDefinitionCanBeUsedWithOpenAI verifies tool can be used in OpenAI params
func TestToolDefinitionCanBeUsedWithOpenAI(t *testing.T) {
	tools := GetAllTools()

	// Verify tools can be used in ChatCompletionNewParams
	params := openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("What's the weather?"),
		},
		Tools: tools, // Should compile without errors
		Model: openai.ChatModelGPT4oMini,
	}

	// Verify params can be marshaled
	_, err := json.Marshal(params)
	if err != nil {
		t.Errorf("Failed to marshal params with tools: %v", err)
	}
}

// TestWeatherToolParametersSchema verifies the JSON schema is valid
func TestWeatherToolParametersSchema(t *testing.T) {
	tool := GetWeatherTool()

	toolJSON, err := json.Marshal(tool)
	if err != nil {
		t.Fatalf("Failed to marshal tool: %v", err)
	}

	var toolMap map[string]any
	if err := json.Unmarshal(toolJSON, &toolMap); err != nil {
		t.Fatalf("Failed to unmarshal tool JSON: %v", err)
	}

	function := toolMap["function"].(map[string]any)
	params := function["parameters"].(map[string]any)
	properties := params["properties"].(map[string]any)

	// Check location property schema
	location := properties["location"].(map[string]any)
	if location["type"] != "string" {
		t.Errorf("Expected location type 'string', got %v", location["type"])
	}
	if _, hasDesc := location["description"]; !hasDesc {
		t.Error("Location property missing description")
	}

	// Check unit property schema
	unit := properties["unit"].(map[string]any)
	if unit["type"] != "string" {
		t.Errorf("Expected unit type 'string', got %v", unit["type"])
	}

	// Check unit has enum values
	enum, ok := unit["enum"].([]any)
	if !ok {
		t.Fatal("Unit property missing enum values")
	}

	expectedEnums := map[string]bool{"celsius": false, "fahrenheit": false}
	for _, e := range enum {
		if str, ok := e.(string); ok {
			if _, exists := expectedEnums[str]; exists {
				expectedEnums[str] = true
			}
		}
	}

	for enumVal, found := range expectedEnums {
		if !found {
			t.Errorf("Expected enum value %q not found", enumVal)
		}
	}
}
