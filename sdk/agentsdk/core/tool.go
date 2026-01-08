package core

import (
	"context"
	"encoding/json"
	"fmt"
)

// Tool represents a tool that can be called by an LLM.
// This interface follows the agent-framework pattern where tools are
// self-describing and invokable.
type Tool interface {
	// Name returns the unique name of the tool
	Name() string

	// Description returns a human-readable description of what the tool does
	Description() string

	// Parameters returns the JSON schema for the tool's parameters
	Parameters() map[string]any

	// Invoke executes the tool with the given arguments
	// The arguments are passed as a JSON string
	Invoke(ctx context.Context, arguments string) (string, error)
}

// ToolSpec represents the JSON schema specification of a tool
// This is used to send tool definitions to the LLM.
type ToolSpec struct {
	Type     string       `json:"type"`
	Function FunctionSpec `json:"function"`
}

// FunctionSpec describes a function tool.
type FunctionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ToToolSpec converts a Tool to a ToolSpec for LLM consumption.
func ToToolSpec(t Tool) ToolSpec {
	return ToolSpec{
		Type: "function",
		Function: FunctionSpec{
			Name:        t.Name(),
			Description: t.Description(),
			Parameters:  t.Parameters(),
		},
	}
}

// FunctionTool is a Tool implementation that wraps a function.
type FunctionTool struct {
	name        string
	description string
	parameters  map[string]any
	handler     func(ctx context.Context, arguments string) (string, error)
}

// NewFunctionTool creates a new function tool.
func NewFunctionTool(
	name string,
	description string,
	parameters map[string]any,
	handler func(ctx context.Context, arguments string) (string, error),
) *FunctionTool {
	return &FunctionTool{
		name:        name,
		description: description,
		parameters:  parameters,
		handler:     handler,
	}
}

func (f *FunctionTool) Name() string {
	return f.name
}

func (f *FunctionTool) Description() string {
	return f.description
}

func (f *FunctionTool) Parameters() map[string]any {
	return f.parameters
}

func (f *FunctionTool) Invoke(ctx context.Context, arguments string) (string, error) {
	return f.handler(ctx, arguments)
}

// Verify FunctionTool implements Tool.
var _ Tool = (*FunctionTool)(nil)

// ToolRegistry holds a collection of tools by name.
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new empty tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]

	return tool, ok
}

// All returns all registered tools.
func (r *ToolRegistry) All() []Tool {
	result := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}

	return result
}

// ToToolSpecs converts all tools to ToolSpecs for LLM consumption.
func (r *ToolRegistry) ToToolSpecs() []ToolSpec {
	result := make([]ToolSpec, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, ToToolSpec(tool))
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
