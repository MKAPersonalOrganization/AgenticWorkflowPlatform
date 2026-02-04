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

// SimpleFunctionTool creates a simple function-based tool.
// This is a convenience constructor similar to core.NewFunctionTool.
// It wraps the simpler function signature into the Tool interface.
func SimpleFunctionTool(name, description string, inputSchema map[string]any, fn func(context.Context, map[string]any) (any, error)) Tool {
	return NewFunctionTool(name, description, inputSchema, func(ctx context.Context, tctx *ToolContext) (*ToolResult, error) {
		// Parse arguments from JSON string
		var args map[string]any
		if tctx.Arguments != "" {
			if err := json.Unmarshal([]byte(tctx.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to parse tool arguments: %w", err)
			}
		}

		// Execute function
		result, err := fn(ctx, args)
		if err != nil {
			return &ToolResult{
				ToolCallID: tctx.ToolCallID,
				Output:     err.Error(),
				IsError:    true,
			}, nil
		}

		// Convert result to string
		var output string
		switch v := result.(type) {
		case string:
			output = v
		case fmt.Stringer:
			output = v.String()
		default:
			data, err := json.Marshal(result)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool result: %w", err)
			}
			output = string(data)
		}

		return &ToolResult{
			ToolCallID: tctx.ToolCallID,
			Output:     output,
			IsError:    false,
		}, nil
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
