package core_test

import (
	"context"
	"encoding/json"
	"fmt"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/core"
	"testing"
)

func TestFunctionTool(t *testing.T) {
	t.Parallel()
	tool := agent.NewFunctionTool(
		"get_weather",
		"Get the current weather for a location",
		&agent.Schema{
			Type: agent.TypeObject,
			Properties: map[string]*agent.Schema{
				"location": {Type: agent.TypeString, Description: "The city name"},
				"unit":     {Type: agent.TypeString, Enum: []string{"celsius", "fahrenheit"}},
			},
			Required: []string{"location"},
		},
		func(ctx context.Context, args agent.Arguments) (agent.Result, error) {
			var params struct {
				Location string `json:"location"`
				Unit     string `json:"unit"`
			}
			if err := args.Decode(&params); err != nil {
				return agent.NewResultError(fmt.Errorf("failed to parse arguments: %w", err)), nil
			}
			if params.Unit == "" {
				params.Unit = "celsius"
			}

			return agent.JSONResult(map[string]any{
				"temperature": 22,
				"unit":        params.Unit,
				"location":    params.Location,
			}), nil
		},
	)

	if tool.Name() != "get_weather" {
		t.Errorf("expected name 'get_weather', got '%s'", tool.Name())
	}

	if tool.Description() != "Get the current weather for a location" {
		t.Errorf("unexpected description: %s", tool.Description())
	}

	// Test execution
	rawArgs := []byte(`{"location": "London", "unit": "celsius"}`)
	result, err := tool.Execute(context.Background(), agent.Arguments{Raw: rawArgs})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}

	resMap := make(map[string]any)
	err = json.Unmarshal([]byte(result.Content[0].(string)), &resMap)
	if err != nil {
		t.Fatalf("failed to unmarshal content: %v", err)
	}

	if resMap["location"] != "London" {
		t.Errorf("expected location 'London', got '%v'", resMap["location"])
	}
	if resMap["temperature"].(float64) != 22 {
		t.Errorf("expected temperature 22, got %v", resMap["temperature"])
	}
}

func TestToolRegistry(t *testing.T) {
	t.Parallel()
	registry := core.NewToolRegistry()

	// Register some tools
	weatherTool := agent.NewFunctionTool(
		"get_weather",
		"Get weather",
		nil,
		func(ctx context.Context, args agent.Arguments) (agent.Result, error) {
			return agent.JSONResult(map[string]any{"temp": 20}), nil
		},
	)

	calcTool := agent.NewFunctionTool(
		"calculate",
		"Calculate expression",
		nil,
		func(ctx context.Context, args agent.Arguments) (agent.Result, error) {
			return agent.JSONResult(map[string]any{"result": 42}), nil
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
