package core_test

import (
	"context"
	"fmt"
	"testing"

	"school-with-ai/foundation/agentsdk/core"
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
		func(ctx core.ToolContext, args any) (any, error) {
			var params struct {
				Location string `json:"location"`
				Unit     string `json:"unit"`
			}
			if err := core.UnmarshalArgs(args, &params); err != nil {
				return nil, fmt.Errorf("failed to parse arguments: %w", err)
			}
			if params.Unit == "" {
				params.Unit = "celsius"
			}

			return map[string]any{
				"temperature": 22,
				"unit":        params.Unit,
				"location":    params.Location,
			}, nil
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
	result, err := tool.Invoke(core.NewToolContext(context.Background(), "", "", "", nil), map[string]any{"location": "London", "unit": "celsius"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any result, got %T", result)
	}

	if resMap["location"] != "London" {
		t.Errorf("expected location 'London', got '%v'", resMap["location"])
	}
	if resMap["temperature"] != 22 {
		t.Errorf("expected temperature 22, got %v", resMap["temperature"])
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
		func(ctx core.ToolContext, args any) (any, error) {
			return map[string]any{"temp": 20}, nil
		},
	)

	calcTool := core.NewFunctionTool(
		"calculate",
		"Calculate expression",
		core.ObjectSchema(map[string]any{"expression": core.StringProperty("Math expression")}, []string{"expression"}),
		func(ctx core.ToolContext, args any) (any, error) {
			return map[string]any{"result": 42}, nil
		},
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
		func(ctx core.ToolContext, args any) (any, error) { return map[string]any{}, nil },
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
