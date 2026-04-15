package planmode

import (
	"aicompass/foundation/agent"
	"context"
	"encoding/json"
	"fmt"
)

// readOnlyTools is the whitelist of tool categories allowed during plan mode.
// Mirrors Gemini CLI's PLAN_MODE_TOOLS — only research/read-only tools.
var readOnlyTools = []string{
	"read_file", "list_directory", "glob", "grep_search",
	"web_search", "web_fetch",
	"think",
	"query_knowledge_base",
	"write_todos",
}

var enterPlanModeSchema = &agent.Schema{
	Type: agent.TypeObject,
	Description: `Switch the agent into Plan Mode — a research-only phase where you gather information, analyze context, and formulate a structured plan BEFORE taking any actions.

CRITICAL RULES WHILE IN PLAN MODE:
• You may ONLY use read-only tools: search, read, think, query, list, write_todos
• You MUST NOT use any tools that create, modify, or delete content (no create_markdown, no update_markdown, etc.)
• You MUST NOT attempt to produce a final answer — your goal is to research and plan
• When your research is complete, call exit_plan_mode with your structured plan

Use plan mode when the user's request is complex and requires:
1. Understanding the full scope before acting
2. Researching multiple sources or knowledge areas
3. Breaking down a multi-step task into a safe execution plan
4. Avoiding costly mistakes by thinking first`,
	Properties: map[string]*agent.Schema{
		"reason": {
			Type:        agent.TypeString,
			Description: "A brief explanation of why you are entering plan mode and what you intend to research.",
		},
	},
}

// EnterPlanModeTool switches the agent into read-only plan mode.
type EnterPlanModeTool struct{}

func (t *EnterPlanModeTool) Name() string          { return "enter_plan_mode" }
func (t *EnterPlanModeTool) Description() string   { return enterPlanModeSchema.Description }
func (t *EnterPlanModeTool) Schema() *agent.Schema { return enterPlanModeSchema }

func (t *EnterPlanModeTool) Execute(_ context.Context, args agent.Arguments) (agent.Result, error) {
	var parsed struct {
		Reason string `json:"reason"`
	}
	// Reason is optional — ignore decode errors for empty args
	_ = args.Decode(&parsed)

	reason := parsed.Reason
	if reason == "" {
		reason = "Complex task requiring research before execution"
	}

	result := map[string]any{
		"status":        "plan_mode_active",
		"message":       "Switched to Plan Mode. You are now in research-only mode.",
		"reason":        reason,
		"plan_mode":     true,
		"allowed_tools": readOnlyTools,
		"instructions": fmt.Sprintf(
			"PLAN MODE ACTIVE — Research phase started.\n"+
				"Reason: %s\n\n"+
				"ALLOWED ACTIONS:\n"+
				"• Search and read files, knowledge bases, and the web\n"+
				"• Use the think tool to reason about what you find\n"+
				"• Use write_todos to track your research progress\n"+
				"• Ask the user for clarification if needed\n\n"+
				"FORBIDDEN ACTIONS:\n"+
				"• Creating, updating, or deleting any content\n"+
				"• Producing a final answer\n"+
				"• Any tool not in the read-only whitelist\n\n"+
				"When your research is complete, call exit_plan_mode with your structured plan.",
			reason,
		),
	}

	resultJSON, _ := json.Marshal(result)
	return agent.Result{
		Content: []any{string(resultJSON)},
	}, nil
}

var _ agent.ExecutableTool = (*EnterPlanModeTool)(nil)
