package llm

import "maps"

// AITool is the base interface for all AI tools.
// This mirrors Microsoft.Extensions.AI.AITool from the .NET agent framework.
//
// Tools represent capabilities that can be called by AI models during generation.
type AITool interface {
	// Name returns the unique identifier for this tool.
	Name() string

	// Description returns a human-readable description of what the tool does.
	Description() string

	// Parameters returns the JSON Schema describing the tool's parameters.
	Parameters() map[string]any
}

// AIFunctionTool represents a function-based tool with metadata.
// This is the concrete implementation of AITool.
type AIFunctionTool struct {
	name        string
	description string
	parameters  map[string]any
}

// Name implements AITool.
func (t AIFunctionTool) Name() string {
	return t.name
}

// Description implements AITool.
func (t AIFunctionTool) Description() string {
	return t.description
}

// Parameters implements AITool.
func (t AIFunctionTool) Parameters() map[string]any {
	return t.parameters
}

// Clone creates a deep copy of AIFunctionTool.
func (t AIFunctionTool) Clone() AIFunctionTool {
	var params map[string]any
	if t.parameters != nil {
		params = make(map[string]any, len(t.parameters))
		maps.Copy(params, t.parameters)
	}

	return AIFunctionTool{
		name:        t.name,
		description: t.description,
		parameters:  params,
	}
}

// Verify AIFunctionTool implements AITool.
var _ AITool = AIFunctionTool{}

// NewAIFunctionTool creates a new function tool with the given metadata.
func NewAIFunctionTool(name, description string, parameters map[string]any) AIFunctionTool {
	return AIFunctionTool{
		name:        name,
		description: description,
		parameters:  parameters,
	}
}

// CloneTools creates a deep copy of a slice of AITools.
func CloneTools(tools []AITool) []AITool {
	if tools == nil {
		return nil
	}

	cloned := make([]AITool, len(tools))
	for i, tool := range tools {
		if ft, ok := tool.(AIFunctionTool); ok {
			cloned[i] = ft.Clone()
		} else {
			cloned[i] = tool
		}
	}

	return cloned
}
