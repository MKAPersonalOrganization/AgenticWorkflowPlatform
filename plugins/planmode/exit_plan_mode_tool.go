package planmode

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"aicompass/foundation/agent"
)

var exitPlanModeSchema = &agent.Schema{
	Type: agent.TypeObject,
	Description: `Exit Plan Mode by presenting your completed research plan for user review. The plan should be a well-structured Markdown document that describes what you intend to do, step by step, based on your research.

The user will see your plan and can:
• Approve it — you proceed to execute the plan in normal mode
• Reject it with feedback — you stay in plan mode to refine your approach

Only call this tool when you have completed your research and are confident in your plan.`,
	Properties: map[string]*agent.Schema{
		"title": {
			Type:        agent.TypeString,
			Description: "A concise title for the plan (e.g., 'Essay Research Plan', 'Code Refactoring Strategy').",
		},
		"plan": {
			Type:        agent.TypeString,
			Description: "The complete plan in Markdown format. Include: objective, key findings from research, step-by-step execution plan, expected outcomes, and any risks or considerations.",
		},
	},
	Required: []string{"plan"},
}

// ExitPlanModeTool presents the agent's research plan for user approval.
type ExitPlanModeTool struct{}

func (t *ExitPlanModeTool) Name() string          { return "exit_plan_mode" }
func (t *ExitPlanModeTool) Description() string   { return exitPlanModeSchema.Description }
func (t *ExitPlanModeTool) Schema() *agent.Schema { return exitPlanModeSchema }

func (t *ExitPlanModeTool) Execute(_ context.Context, args agent.Arguments) (agent.Result, error) {
	var parsed struct {
		Title string `json:"title"`
		Plan  string `json:"plan"`
	}
	if err := args.Decode(&parsed); err != nil {
		return agent.ErrorResult(fmt.Errorf("invalid arguments: %w", err)), nil
	}

	// Validate plan content
	if err := validatePlan(parsed.Plan); err != nil {
		return agent.ErrorResult(err), nil
	}

	title := parsed.Title
	if title == "" {
		title = "Execution Plan"
	}

	// Count plan steps (lines starting with number or bullet)
	steps := countPlanSteps(parsed.Plan)

	result := map[string]any{
		"status":    "plan_submitted",
		"message":   fmt.Sprintf("Plan '%s' submitted for review. Exiting plan mode.", title),
		"title":     title,
		"plan":      parsed.Plan,
		"plan_mode": false,
		"steps":     steps,
		"instructions": "Plan mode ended. The plan has been presented to the user. " +
			"Wait for the user to approve or provide feedback before proceeding. " +
			"If approved, execute the plan step by step. " +
			"If rejected, incorporate the feedback and either re-enter plan mode or adjust your approach.",
	}

	resultJSON, _ := json.Marshal(result)
	return agent.Result{
		Content: []any{string(resultJSON)},
	}, nil
}

// validatePlan checks the plan content for structural correctness.
func validatePlan(plan string) error {
	trimmed := strings.TrimSpace(plan)
	if trimmed == "" {
		return fmt.Errorf("plan must not be empty — include your research findings and step-by-step execution plan")
	}
	if len(trimmed) < 50 {
		return fmt.Errorf("plan is too brief (%d chars) — a proper plan should include objectives, steps, and expected outcomes", len(trimmed))
	}
	if len(trimmed) > 50000 {
		return fmt.Errorf("plan exceeds maximum length (50,000 chars) — summarize or split into phases")
	}
	return nil
}

// countPlanSteps counts lines that look like plan steps (numbered or bulleted).
func countPlanSteps(plan string) int {
	count := 0
	for _, line := range strings.Split(plan, "\n") {
		trimmed := strings.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		// Match numbered steps: "1.", "2.", etc.
		if len(trimmed) > 1 && trimmed[0] >= '1' && trimmed[0] <= '9' && strings.Contains(trimmed[:min(4, len(trimmed))], ".") {
			count++
			continue
		}
		// Match bullet points: "- ", "* ", "• "
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "• ") {
			count++
		}
	}
	return count
}

var _ agent.ExecutableTool = (*ExitPlanModeTool)(nil)
