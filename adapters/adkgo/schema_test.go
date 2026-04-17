package adkgo

import (
	"testing"

	"google.golang.org/genai"

	fagent "aicompass/foundation/agent"
)

// TestFoundationSchemaToGenAI_UppercasesTypes guards against the bug where the
// converter returned genai schemas with JSON-Schema lowercase types ("object",
// "string", …) instead of OpenAPI uppercase ("OBJECT", "STRING", …). genai
// serialises an unknown type as invalid, which manifested as LLMs silently
// ignoring the OutputSchema and not following it.
func TestFoundationSchemaToGenAI_UppercasesTypes(t *testing.T) {
	t.Parallel()

	in := &fagent.Schema{
		Type:        fagent.TypeObject,
		Description: "a person",
		Properties: map[string]*fagent.Schema{
			"name":    {Type: fagent.TypeString, Description: "full name"},
			"age":     {Type: fagent.TypeInteger},
			"height":  {Type: fagent.TypeNumber},
			"admin":   {Type: fagent.TypeBoolean},
			"tags":    {Type: fagent.TypeArray, Items: &fagent.Schema{Type: fagent.TypeString}},
			"address": {Type: fagent.TypeObject, Properties: map[string]*fagent.Schema{"city": {Type: fagent.TypeString}}},
		},
		Required: []string{"name"},
	}

	got := foundationSchemaToGenAI(in)
	if got == nil {
		t.Fatal("conversion returned nil")
	}
	if got.Type != genai.TypeObject {
		t.Errorf("root type = %q, want %q", got.Type, genai.TypeObject)
	}
	want := map[string]genai.Type{
		"name":    genai.TypeString,
		"age":     genai.TypeInteger,
		"height":  genai.TypeNumber,
		"admin":   genai.TypeBoolean,
		"tags":    genai.TypeArray,
		"address": genai.TypeObject,
	}
	for k, expected := range want {
		p, ok := got.Properties[k]
		if !ok {
			t.Errorf("property %q missing", k)
			continue
		}
		if p.Type != expected {
			t.Errorf("property %q: type = %q, want %q", k, p.Type, expected)
		}
	}
	// Array items must also uppercase.
	if got.Properties["tags"].Items == nil || got.Properties["tags"].Items.Type != genai.TypeString {
		t.Errorf("tags.items type mismatch: got %+v", got.Properties["tags"].Items)
	}
	// Nested object properties recurse correctly.
	if got.Properties["address"].Properties["city"].Type != genai.TypeString {
		t.Errorf("address.city type = %q, want %q", got.Properties["address"].Properties["city"].Type, genai.TypeString)
	}
}

// TestFoundationSchemaToGenAI_UnknownType ensures unmapped type tokens become
// TypeUnspecified rather than leaking through as an invalid literal.
func TestFoundationSchemaToGenAI_UnknownType(t *testing.T) {
	t.Parallel()

	in := &fagent.Schema{Type: fagent.SchemaType("bogus")}
	got := foundationSchemaToGenAI(in)
	if got.Type != genai.TypeUnspecified {
		t.Errorf("unknown type = %q, want %q", got.Type, genai.TypeUnspecified)
	}
}

// TestBuildOpenAIResponseFormat guards the OpenAI structured-output path:
// every object node must carry additionalProperties:false and list every
// property in required. Originally-optional properties must have their
// type widened to a [T, "null"] union so strict mode accepts an empty
// value without violating the required-list invariant.
func TestBuildOpenAIResponseFormat(t *testing.T) {
	t.Parallel()

	// Start from what agent_builder sends us: an already-uppercased genai.Schema.
	schema := foundationSchemaToGenAI(&fagent.Schema{
		Type: fagent.TypeObject,
		Properties: map[string]*fagent.Schema{
			"name":     {Type: fagent.TypeString},
			"age":      {Type: fagent.TypeInteger},
			"nickname": {Type: fagent.TypeString}, // will be optional (not in required)
		},
		Required: []string{"name", "age"},
	})

	format := buildOpenAIResponseFormat(schema)
	if format.OfJSONSchema == nil {
		t.Fatal("expected OfJSONSchema to be set")
	}
	if !format.OfJSONSchema.JSONSchema.Strict.Value {
		t.Error("strict mode must be on by default")
	}
	if format.OfJSONSchema.JSONSchema.Name == "" {
		t.Error("name must be non-empty")
	}

	m, ok := format.OfJSONSchema.JSONSchema.Schema.(map[string]any)
	if !ok {
		t.Fatalf("schema payload is not map[string]any, got %T", format.OfJSONSchema.JSONSchema.Schema)
	}
	if m["additionalProperties"] != false {
		t.Errorf("additionalProperties should be false, got %v", m["additionalProperties"])
	}
	req, _ := m["required"].([]string)
	if len(req) != 3 {
		t.Errorf("strict mode requires every property in required; got %v", req)
	}
	props, _ := m["properties"].(map[string]any)
	nickname, _ := props["nickname"].(map[string]any)
	nickType := nickname["type"]
	arr, ok := nickType.([]any)
	if !ok || len(arr) != 2 {
		t.Errorf("optional nickname should have nullable union type, got %v", nickType)
	}
	// Required properties keep their scalar type — not widened to a union.
	name, _ := props["name"].(map[string]any)
	if _, isArr := name["type"].([]any); isArr {
		t.Errorf("required name should not be widened to nullable, got %v", name["type"])
	}
}
