package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// UnmarshalArgs is a helper to parse JSON string arguments into a struct.
func UnmarshalArgs(args string, target any) error {
	if args == "" {
		return nil
	}
	return json.Unmarshal([]byte(args), target)
}

// UnmarshalToolArgs is a helper to unmarshal tool arguments from various formats.
// Similar to core.UnmarshalArgs for compatibility.
func UnmarshalToolArgs(args any, target any) error {
	// If args is already a string, unmarshal it
	if str, ok := args.(string); ok {
		return UnmarshalArgs(str, target)
	}

	// If args is a map or other type, marshal then unmarshal
	data, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("failed to marshal args: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal args: %w", err)
	}

	return nil
}

// SchemaToMap converts a Schema to a map[string]any representation.
func SchemaToMap(s *Schema) map[string]any {
	if s == nil {
		return nil
	}

	m := make(map[string]any)
	if s.Type != "" {
		m["type"] = string(s.Type)
	}
	if s.Description != "" {
		m["description"] = s.Description
	}
	if len(s.Required) > 0 {
		m["required"] = s.Required
	}
	if len(s.Properties) > 0 {
		props := make(map[string]any)
		for k, v := range s.Properties {
			props[k] = SchemaToMap(v)
		}
		m["properties"] = props
	}
	if s.Items != nil {
		m["items"] = SchemaToMap(s.Items)
	}
	if len(s.Enum) > 0 {
		m["enum"] = s.Enum
	}
	if s.Format != "" {
		m["format"] = s.Format
	}
	if s.Default != nil {
		m["default"] = s.Default
	}
	if s.Nullable {
		m["nullable"] = s.Nullable
	}

	return m
}

// SchemaFromMap creates a Schema from a map[string]any representation.
func SchemaFromMap(m map[string]any) *Schema {
	if m == nil {
		return nil
	}

	schema := &Schema{}

	if t, ok := m["type"].(string); ok {
		schema.Type = SchemaType(t)
	}
	if d, ok := m["description"].(string); ok {
		schema.Description = d
	}
	if req, ok := m["required"].([]string); ok {
		schema.Required = req
	}
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok {
				schema.Required = append(schema.Required, s)
			}
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		schema.Properties = make(map[string]*Schema)
		for k, v := range props {
			if propMap, ok := v.(map[string]any); ok {
				schema.Properties[k] = SchemaFromMap(propMap)
			}
		}
	}
	if items, ok := m["items"].(map[string]any); ok {
		schema.Items = SchemaFromMap(items)
	}
	if enum, ok := m["enum"].([]string); ok {
		schema.Enum = enum
	}
	if enum, ok := m["enum"].([]any); ok {
		for _, e := range enum {
			if s, ok := e.(string); ok {
				schema.Enum = append(schema.Enum, s)
			}
		}
	}
	if f, ok := m["format"].(string); ok {
		schema.Format = f
	}
	schema.Default = m["default"]
	if n, ok := m["nullable"].(bool); ok {
		schema.Nullable = n
	}

	return schema
}

// SimpleFunctionTool creates a simple function-based tool.
// This is a convenience constructor for creating tools with simpler function signatures.
func SimpleFunctionTool(name, description string, inputSchema map[string]any, fn func(context.Context, map[string]any) (any, error)) ExecutableTool {
	schema := SchemaFromMap(inputSchema)
	return NewFunctionTool(name, description, schema, func(ctx context.Context, args Arguments) (Result, error) {
		// Parse arguments from JSON
		var argsMap map[string]any
		if args.Raw != nil {
			if err := json.Unmarshal(args.Raw, &argsMap); err != nil {
				return ErrorResult(fmt.Errorf("failed to parse tool arguments: %w", err)), nil
			}
		}

		// Execute function
		result, err := fn(ctx, argsMap)
		if err != nil {
			return ErrorResult(err), nil
		}

		// Convert result to string
		switch v := result.(type) {
		case string:
			return TextResult(v), nil
		case fmt.Stringer:
			return TextResult(v.String()), nil
		default:
			return JSONResult(result), nil
		}
	})
}

// ToolSet acts as a registry for collecting tools.
type ToolSet struct {
	tools []Tool
}

// NewToolSet creates a new tool set.
func NewToolSet() *ToolSet {
	return &ToolSet{
		tools: make([]Tool, 0),
	}
}

// Register adds a tool to the set.
func (ts *ToolSet) Register(t Tool) {
	ts.tools = append(ts.tools, t)
}

// AddAll adds multiple tools to the set.
func (ts *ToolSet) AddAll(tools []Tool) {
	ts.tools = append(ts.tools, tools...)
}

// All returns all registered tools.
func (ts *ToolSet) All() []Tool {
	return ts.tools
}
