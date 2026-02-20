package cognitive

import (
	"context"
	"fmt"

	"school-with-ai/foundation/agent"
)

// ThinkArgs represents the arguments for the think tool.
type ThinkArgs struct {
	Thoughts string `json:"thoughts"`
}

var thinkSchema = &agent.Schema{
	Type: agent.TypeObject,
	Properties: map[string]*agent.Schema{
		"thoughts": {
			Type:        agent.TypeString,
			Description: "Precisely describe what you are thinking about.",
		},
	},
	Required: []string{"thoughts"},
}

// ThinkTool allows the agent to output its thoughts before acting.
type ThinkTool struct{}

func (t *ThinkTool) Name() string { return "think" }

func (t *ThinkTool) Description() string {
	return "Use when you want to think through a problem, clarify your assumptions, or break down complex steps before acting or responding."
}

func (t *ThinkTool) Schema() *agent.Schema { return thinkSchema }

func (t *ThinkTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	var params ThinkArgs
	if err := args.Decode(&params); err != nil {
		return agent.ErrorResult(fmt.Errorf("failed to parse think args: %w", err)), nil
	}

	// We simply return OK, as the purpose of the tool is just to let the model generate text in the arguments.
	return agent.JSONResult(map[string]any{"status": "OK"}), nil
}
