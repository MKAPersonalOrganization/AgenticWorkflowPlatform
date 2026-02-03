package agent

import "context"

// FunctionTool is a Tool implementation that wraps a function.
type FunctionTool struct {
	name        string
	description string
	parameters  map[string]any
	handler     func(ctx context.Context, tCtx *ToolContext) (*ToolResult, error)
}

// NewFunctionTool creates a new function tool.
func NewFunctionTool(
	name string,
	description string,
	parameters map[string]any,
	handler func(ctx context.Context, tCtx *ToolContext) (*ToolResult, error),
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

func (f *FunctionTool) InputSchema() map[string]any {
	return f.parameters
}

func (f *FunctionTool) Execute(ctx context.Context, tCtx *ToolContext) (*ToolResult, error) {
	return f.handler(ctx, tCtx)
}
