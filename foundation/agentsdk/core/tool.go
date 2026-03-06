package core

import (
	"encoding/json"
	"fmt"
	"school-with-ai/foundation/agent"
)

// UnmarshalArgs is a helper function to unmarshal typed arguments into a struct.
// This reduces boilerplate in tool implementations.
//
// Example usage:
//
//	var params struct {
//	    Expression string `json:"expression"`
//	}
//	if err := core.UnmarshalArgs(args, &params); err != nil {
//	    return nil, err
//	}
func UnmarshalArgs(args any, target any) error {
	jsonBytes, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal args: %w", err)
	}
	if err := json.Unmarshal(jsonBytes, target); err != nil {
		return fmt.Errorf("failed to unmarshal args: %w", err)
	}
	return nil
}

// ToolRegistry holds a collection of tools by name.
// Updated to use spec-compliant agent.ExecutableTool interface.
type ToolRegistry struct {
	tools map[string]agent.ExecutableTool
}

// NewToolRegistry creates a new empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]agent.ExecutableTool),
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool agent.ExecutableTool) {
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) (agent.ExecutableTool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// All returns all registered tools.
func (r *ToolRegistry) All() []agent.ExecutableTool {
	result := make([]agent.ExecutableTool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// JSONSchema helpers for building parameter schemas

// ObjectSchema creates a JSON schema for an object type.
func ObjectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// StringProperty creates a string property schema.
func StringProperty(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}

// IntegerProperty creates an integer property schema.
func IntegerProperty(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"description": description,
	}
}

// NumberProperty creates a number property schema.
func NumberProperty(description string) map[string]any {
	return map[string]any{
		"type":        "number",
		"description": description,
	}
}

// BooleanProperty creates a boolean property schema.
func BooleanProperty(description string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": description,
	}
}

// EnumProperty creates a string enum property schema.
func EnumProperty(description string, values []string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"enum":        values,
	}
}

// ParseArguments is a helper to parse JSON arguments into a struct.
func ParseArguments[T any](arguments string) (T, error) {
	var result T
	err := json.Unmarshal([]byte(arguments), &result)
	if err != nil {
		return result, fmt.Errorf("failed to parse arguments: %w", err)
	}

	return result, nil
}
