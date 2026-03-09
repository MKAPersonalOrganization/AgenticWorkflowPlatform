package custom

import (
	"context"
	"fmt"
	"school-with-ai/foundation/agent"
)

// ExecutableToolAdapter adapts agent.Tool to agent.ExecutableTool.
// This is used when a tool only implements the metadata interface (agent.Tool)
// but needs to be used where execution is required (agent.ExecutableTool).
type ExecutableToolAdapter struct {
	tool     agent.Tool
	executor agent.Executor
}

// NewExecutableToolAdapter creates an adapter from agent.Tool and agent.Executor.
// If the tool already implements agent.ExecutableTool, it returns it directly.
func NewExecutableToolAdapter(tool agent.Tool) agent.ExecutableTool {
	// Check if tool already implements ExecutableTool
	if et, ok := tool.(agent.ExecutableTool); ok {
		return et
	}

	// Check if tool implements Executor separately
	var executor agent.Executor
	if ex, ok := tool.(agent.Executor); ok {
		executor = ex
	}

	return &ExecutableToolAdapter{
		tool:     tool,
		executor: executor,
	}
}

func (a *ExecutableToolAdapter) Name() string {
	return a.tool.Name()
}

func (a *ExecutableToolAdapter) Description() string {
	return a.tool.Description()
}

func (a *ExecutableToolAdapter) Schema() *agent.Schema {
	return a.tool.Schema()
}

func (a *ExecutableToolAdapter) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	if a.executor != nil {
		return a.executor.Execute(ctx, args)
	}
	// If no executor, return error
	return agent.Result{}, fmt.Errorf("tool %s does not implement Execute method", a.tool.Name())
}
