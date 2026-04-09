package cognitive

import (
	"aicompass/foundation/agent"
	"context"
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
// It implements StreamingExecutableTool to stream the thoughts content
// in real-time via the TOOL_CONTENT_STREAMED event.
type ThinkTool struct{}

func (t *ThinkTool) Name() string { return "think" }

func (t *ThinkTool) Description() string {
	return "Use when you want to think through a problem, clarify your assumptions, or break down complex steps before acting or responding."
}

func (t *ThinkTool) Schema() *agent.Schema { return thinkSchema }

// Execute implements the Executor interface.
// The think tool's purpose is to let the model generate text in the arguments;
// the actual streaming of "thoughts" happens during LLM argument generation
// (via ToolContentStreamed events emitted by StreamProcessor), not here.
func (t *ThinkTool) Execute(ctx context.Context, args agent.Arguments) (agent.Result, error) {
	// We simply return OK. The thoughts content was already streamed to the
	// frontend in real-time during the LLM response phase.
	return agent.JSONResult(map[string]any{"status": "OK"}), nil
}

// Verify ThinkTool implements ExecutableTool.
var _ agent.ExecutableTool = (*ThinkTool)(nil)
