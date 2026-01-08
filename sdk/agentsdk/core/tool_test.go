package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"school-with-ai/sdk/agentsdk/core"
	"testing"
)

func TestFunctionTool(t *testing.T) {
	t.Parallel()
	tool := core.NewFunctionTool(
		"get_weather",
		"Get the current weather for a location",
		core.ObjectSchema(
			map[string]any{
				"location": core.StringProperty("The city name"),
				"unit":     core.EnumProperty("Temperature unit", []string{"celsius", "fahrenheit"}),
			},
			[]string{"location"},
		),
		func(ctx context.Context, arguments string) (string, error) {
			var args struct {
				Location string `json:"location"`
				Unit     string `json:"unit"`
			}
			err := json.Unmarshal([]byte(arguments), &args)
			if err != nil {
				return "", fmt.Errorf("failed to unmarshal arguments: %w", err)
			}
			if args.Unit == "" {
				args.Unit = "celsius"
			}

			return `{"temperature": 22, "unit": "` + args.Unit + `", "location": "` + args.Location + `"}`, nil
		},
	)

	if tool.Name() != "get_weather" {
		t.Errorf("expected name 'get_weather', got '%s'", tool.Name())
	}

	if tool.Description() != "Get the current weather for a location" {
		t.Errorf("unexpected description: %s", tool.Description())
	}

	params := tool.Parameters()
	if params["type"] != "object" {
		t.Errorf("expected type 'object', got '%v'", params["type"])
	}

	// Test invocation
	result, err := tool.Invoke(context.Background(), `{"location": "London", "unit": "celsius"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var response struct {
		Temperature int    `json:"temperature"`
		Unit        string `json:"unit"`
		Location    string `json:"location"`
	}
	if err := json.Unmarshal([]byte(result), &response); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if response.Location != "London" {
		t.Errorf("expected location 'London', got '%s'", response.Location)
	}
	if response.Temperature != 22 {
		t.Errorf("expected temperature 22, got %d", response.Temperature)
	}
}

func TestToolRegistry(t *testing.T) {
	t.Parallel()
	registry := core.NewToolRegistry()

	// Register some tools
	weatherTool := core.NewFunctionTool(
		"get_weather",
		"Get weather",
		core.ObjectSchema(map[string]any{"location": core.StringProperty("City")}, []string{"location"}),
		func(ctx context.Context, args string) (string, error) { return `{"temp": 20}`, nil },
	)

	calcTool := core.NewFunctionTool(
		"calculate",
		"Calculate expression",
		core.ObjectSchema(map[string]any{"expression": core.StringProperty("Math expression")}, []string{"expression"}),
		func(ctx context.Context, args string) (string, error) { return `{"result": 42}`, nil },
	)

	registry.Register(weatherTool)
	registry.Register(calcTool)

	// Test Get
	tool, ok := registry.Get("get_weather")
	if !ok {
		t.Fatal("expected to find get_weather tool")
	}
	if tool.Name() != "get_weather" {
		t.Errorf("unexpected tool name: %s", tool.Name())
	}

	// Test not found
	_, ok = registry.Get("unknown")
	if ok {
		t.Error("expected unknown tool to not be found")
	}

	// Test All
	all := registry.All()
	if len(all) != 2 {
		t.Errorf("expected 2 tools, got %d", len(all))
	}

	// Test ToToolSpecs
	specs := registry.ToToolSpecs()
	if len(specs) != 2 {
		t.Errorf("expected 2 specs, got %d", len(specs))
	}
	for _, spec := range specs {
		if spec.Type != "function" {
			t.Errorf("expected type 'function', got '%s'", spec.Type)
		}
	}
}

func TestToToolSpec(t *testing.T) {
	t.Parallel()
	tool := core.NewFunctionTool(
		"test_tool",
		"A test tool",
		core.ObjectSchema(
			map[string]any{
				"param1": core.StringProperty("First parameter"),
				"param2": core.IntegerProperty("Second parameter"),
			},
			[]string{"param1"},
		),
		func(ctx context.Context, args string) (string, error) { return "{}", nil },
	)

	spec := core.ToToolSpec(tool)

	if spec.Type != "function" {
		t.Errorf("expected type 'function', got '%s'", spec.Type)
	}
	if spec.Function.Name != "test_tool" {
		t.Errorf("expected name 'test_tool', got '%s'", spec.Function.Name)
	}
	if spec.Function.Description != "A test tool" {
		t.Errorf("unexpected description: %s", spec.Function.Description)
	}

	// Verify parameters are properly structured
	params := spec.Function.Parameters
	if params["type"] != "object" {
		t.Errorf("expected parameters type 'object', got '%v'", params["type"])
	}
	required, ok := params["required"].([]string)
	if !ok || len(required) != 1 || required[0] != "param1" {
		t.Errorf("unexpected required fields: %v", params["required"])
	}
}

func TestParseArguments(t *testing.T) {
	t.Parallel()
	type Args struct {
		Location string `json:"location"`
		Unit     string `json:"unit"`
	}

	args, err := core.ParseArguments[Args](`{"location": "Paris", "unit": "celsius"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if args.Location != "Paris" {
		t.Errorf("expected location 'Paris', got '%s'", args.Location)
	}
	if args.Unit != "celsius" {
		t.Errorf("expected unit 'celsius', got '%s'", args.Unit)
	}
}

func TestSchemaHelpers(t *testing.T) {
	t.Parallel()
	// Test ObjectSchema
	objSchema := core.ObjectSchema(
		map[string]any{
			"name": core.StringProperty("The name"),
		},
		[]string{"name"},
	)
	if objSchema["type"] != "object" {
		t.Errorf("expected type 'object'")
	}

	// Test StringProperty
	strProp := core.StringProperty("A string")
	if strProp["type"] != "string" {
		t.Errorf("expected type 'string'")
	}
	if strProp["description"] != "A string" {
		t.Errorf("unexpected description")
	}

	// Test IntegerProperty
	intProp := core.IntegerProperty("A number")
	if intProp["type"] != "integer" {
		t.Errorf("expected type 'integer'")
	}

	// Test NumberProperty
	numProp := core.NumberProperty("A decimal")
	if numProp["type"] != "number" {
		t.Errorf("expected type 'number'")
	}

	// Test BooleanProperty
	boolProp := core.BooleanProperty("A flag")
	if boolProp["type"] != "boolean" {
		t.Errorf("expected type 'boolean'")
	}

	// Test EnumProperty
	enumProp := core.EnumProperty("A choice", []string{"a", "b", "c"})
	if enumProp["type"] != "string" {
		t.Errorf("expected type 'string'")
	}
	enum, ok := enumProp["enum"].([]string)
	if !ok || len(enum) != 3 {
		t.Errorf("unexpected enum values: %v", enumProp["enum"])
	}
}
