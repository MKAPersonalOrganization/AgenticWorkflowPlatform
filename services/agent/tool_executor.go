package agent

import (
	"context"
	"fmt"

	"school-with-ai/domain"
)

// ToolExecutor executes tool calls and returns results
type ToolExecutor interface {
	// Execute runs a tool call and returns the result
	Execute(toolCall domain.ToolCall) (string, error)
}

// RegistryToolExecutor executes tools using a ToolRegistry
type RegistryToolExecutor struct {
	registry *ToolRegistry
	ctx      context.Context
}

// NewRegistryToolExecutor creates a new executor with the given registry
func NewRegistryToolExecutor(ctx context.Context, registry *ToolRegistry) *RegistryToolExecutor {
	return &RegistryToolExecutor{
		registry: registry,
		ctx:      ctx,
	}
}

// Execute runs a tool call using the registry
func (e *RegistryToolExecutor) Execute(toolCall domain.ToolCall) (string, error) {
	tool, ok := e.registry.Get(toolCall.Function.Name)
	if !ok {
		return "", fmt.Errorf("tool not found: %s", toolCall.Function.Name)
	}

	return tool.Invoke(e.ctx, toolCall.Function.Arguments)
}

// Verify RegistryToolExecutor implements ToolExecutor
var _ ToolExecutor = (*RegistryToolExecutor)(nil)
