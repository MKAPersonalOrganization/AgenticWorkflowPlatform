package sdk

import (
	"context"
	"fmt"

	"school-with-ai/domain"
)

// BasicToolExecutor implements ToolExecutor using the agent's tool registry
type BasicToolExecutor struct {
	toolRegistry *ToolRegistry
}

// NewBasicToolExecutor creates a new BasicToolExecutor
func NewBasicToolExecutor(toolRegistry *ToolRegistry) *BasicToolExecutor {
	return &BasicToolExecutor{
		toolRegistry: toolRegistry,
	}
}

// Execute runs a tool call and returns the result
func (e *BasicToolExecutor) Execute(toolCall domain.ToolCall) (string, error) {
	toolName := toolCall.Function.Name

	// Look up the tool in the registry
	tool, ok := e.toolRegistry.Get(toolName)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	// Invoke the tool with the provided arguments
	return tool.Invoke(context.Background(), toolCall.Function.Arguments)
}

// Verify BasicToolExecutor implements ToolExecutor
var _ ToolExecutor = (*BasicToolExecutor)(nil)
