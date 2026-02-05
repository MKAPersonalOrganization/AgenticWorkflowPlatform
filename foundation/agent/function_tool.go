package agent

import (
	"context"
)

// FunctionTool is an ExecutableTool implementation that wraps a function.
type FunctionTool struct {
	name        string
	description string
	schema      *Schema
	handler     func(ctx context.Context, args Arguments) (Result, error)
}

// NewFunctionTool creates a new function tool with a Schema.
func NewFunctionTool(
	name string,
	description string,
	schema *Schema,
	handler func(ctx context.Context, args Arguments) (Result, error),
) *FunctionTool {
	return &FunctionTool{
		name:        name,
		description: description,
		schema:      schema,
		handler:     handler,
	}
}

func (f *FunctionTool) Name() string {
	return f.name
}

func (f *FunctionTool) Description() string {
	return f.description
}

func (f *FunctionTool) Schema() *Schema {
	return f.schema
}

// Execute implements the Executor interface.
func (f *FunctionTool) Execute(ctx context.Context, args Arguments) (Result, error) {
	return f.handler(ctx, args)
}
