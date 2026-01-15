package core

import (
	"fmt"
)

// SubagentTool is a tool that delegates execution to a specialized subagent.
// This implements the "Subagents: Centralized orchestration" pattern.
type SubagentTool struct {
	config AgentConfig
	runner SubagentRunner
}

// NewSubagentTool creates a new tool that wraps a subagent.
func NewSubagentTool(config AgentConfig, runner SubagentRunner) *SubagentTool {
	return &SubagentTool{
		config: config,
		runner: runner,
	}
}

func (t *SubagentTool) Name() string {
	return t.config.Name
}

func (t *SubagentTool) Description() string {
	return fmt.Sprintf("Delegates to subagent %s: %s", t.config.Name, t.config.SystemPrompt)
}

func (t *SubagentTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The specific question or task for this specialized subagent.",
			},
		},
		"required": []string{"question"},
	}
}

func (t *SubagentTool) Invoke(ctx ToolContext, args any) (any, error) {
	var params struct {
		Question string `json:"question"`
	}

	if err := UnmarshalArgs(args, &params); err != nil {
		return nil, fmt.Errorf("failed to parse subagent arguments: %w", err)
	}

	if t.runner == nil {
		return nil, fmt.Errorf("no subagent runner configured for tool %s", t.config.Name)
	}

	// Run the subagent and return the result
	result, err := t.runner.RunSubagent(ctx, t.config, ctx.ThreadID, params.Question)
	if err != nil {
		return nil, fmt.Errorf("subagent %s failed: %w", t.config.Name, err)
	}

	return result, nil
}

// Verify SubagentTool implements Tool.
var _ Tool = (*SubagentTool)(nil)
