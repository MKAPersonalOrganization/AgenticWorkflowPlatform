package llm

// AITool is the base interface for all AI tools.
// This mirrors Microsoft.Extensions.AI.AITool from the .NET agent framework.
//
// Tools represent capabilities that can be called by AI models during generation.
// They can be function declarations (metadata only) or executable functions (with implementation).
type AITool interface {
	// GetName returns the unique identifier for this tool.
	GetName() string

	// GetDescription returns a human-readable description of what the tool does.
	// This helps the model decide when to use the tool.
	GetDescription() string

	// GetParameters returns the JSON Schema describing the tool's parameters.
	// Typically includes "type": "object", "properties", and "required" fields.
	GetParameters() map[string]any
}

// AIFunctionTool represents a function-based tool with metadata.
// This is the concrete implementation of AITool, similar to Microsoft.Extensions.AI.AIFunction.
//
// Unlike the old concrete AITool struct, this allows for polymorphism and
// future extension (e.g., built-in tools like code_interpreter, file_search).
type AIFunctionTool struct {
	// Name is the unique identifier for this tool.
	Name string `json:"name"`

	// Description explains what the tool does.
	Description string `json:"description"`

	// Parameters is the JSON Schema describing the tool's parameters.
	Parameters map[string]any `json:"parameters,omitempty"`
}

// GetName implements AITool.
func (t AIFunctionTool) GetName() string {
	return t.Name
}

// GetDescription implements AITool.
func (t AIFunctionTool) GetDescription() string {
	return t.Description
}

// GetParameters implements AITool.
func (t AIFunctionTool) GetParameters() map[string]any {
	return t.Parameters
}

// Clone creates a deep copy of AIFunctionTool.
func (t AIFunctionTool) Clone() AIFunctionTool {
	clone := t

	if t.Parameters != nil {
		clone.Parameters = make(map[string]any, len(t.Parameters))
		for k, v := range t.Parameters {
			clone.Parameters[k] = v
		}
	}

	return clone
}

// Verify AIFunctionTool implements AITool
var (
	_ AITool = AIFunctionTool{}
	_ AITool = (*AIFunctionTool)(nil)
)

// NewAIFunctionTool creates a new function tool with the given metadata.
// This is similar to AIFunctionFactory.CreateDeclaration in .NET.
//
// Example:
//
//	tool := llm.NewAIFunctionTool(
//	    "get_weather",
//	    "Get the current weather for a location",
//	    map[string]any{
//	        "type": "object",
//	        "properties": map[string]any{
//	            "location": map[string]string{
//	                "type": "string",
//	                "description": "City name",
//	            },
//	        },
//	        "required": []string{"location"},
//	    },
//	)
func NewAIFunctionTool(name, description string, parameters map[string]any) AIFunctionTool {
	return AIFunctionTool{
		Name:        name,
		Description: description,
		Parameters:  parameters,
	}
}

// CloneTools creates a deep copy of a slice of AITools.
func CloneTools(tools []AITool) []AITool {
	if tools == nil {
		return nil
	}

	cloned := make([]AITool, len(tools))
	for i, tool := range tools {
		// Use type assertion to clone if the concrete type supports it
		if ft, ok := tool.(AIFunctionTool); ok {
			cloned[i] = ft.Clone()
		} else if ft, ok := tool.(*AIFunctionTool); ok {
			c := ft.Clone()
			cloned[i] = &c
		} else {
			// For other types, just copy the interface value
			cloned[i] = tool
		}
	}

	return cloned
}
