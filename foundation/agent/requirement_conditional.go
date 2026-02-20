package agent

import "context"

// ConditionalRequirement enforces rules for a specific tool based on
// the current state of the conversation (step number, tool history, etc.).
//
// This is the Go port of beeai-framework's ConditionalRequirement.
// It evaluates the conversation history to produce Rules that constrain
// which tools the agent can use on the current turn.
type ConditionalRequirement struct {
	TargetTool         string
	ForceAtStep        int
	MaxInvocations     int
	OnlyAfter          []string
	ForceAfter         []string
	ForceAfterAny      bool // Force after ANY tool call (beeai's force_after=Tool)
	MinInvocations     int
	ConsecutiveAllowed bool // Defaults to true
}

// Ensure it implements the interface
var _ Requirement = (*ConditionalRequirement)(nil)

// NewConditionalRequirement creates a new conditional requirement.
func NewConditionalRequirement(targetTool string, opts ...func(*ConditionalRequirement)) *ConditionalRequirement {
	req := &ConditionalRequirement{
		TargetTool:         targetTool,
		ForceAtStep:        0,  // 0 means no forced step
		MaxInvocations:     -1, // -1 means infinite
		MinInvocations:     0,
		ConsecutiveAllowed: true,
	}
	for _, opt := range opts {
		opt(req)
	}
	return req
}

// Options for ConditionalRequirement

func ForceAtStep(step int) func(*ConditionalRequirement) {
	return func(c *ConditionalRequirement) { c.ForceAtStep = step }
}

func MaxInvocations(max int) func(*ConditionalRequirement) {
	return func(c *ConditionalRequirement) { c.MaxInvocations = max }
}

func MinInvocations(min int) func(*ConditionalRequirement) {
	return func(c *ConditionalRequirement) { c.MinInvocations = min }
}

func OnlyAfter(tools ...string) func(*ConditionalRequirement) {
	return func(c *ConditionalRequirement) { c.OnlyAfter = tools }
}

func ForceAfter(tools ...string) func(*ConditionalRequirement) {
	return func(c *ConditionalRequirement) { c.ForceAfter = tools }
}

// ForceAfterAnyTool forces the target tool after ANY other tool call.
// This enables the ReAct (Reason+Act) pattern: Think → Act → Think → Act → ...
// Equivalent to beeai's force_after=Tool.
func ForceAfterAnyTool() func(*ConditionalRequirement) {
	return func(c *ConditionalRequirement) { c.ForceAfterAny = true }
}

func PreventConsecutive() func(*ConditionalRequirement) {
	return func(c *ConditionalRequirement) { c.ConsecutiveAllowed = false }
}

// extractToolHistory scans the conversation history and returns step count,
// target tool invocations, last tool used, and the set of all tools used.
func (c *ConditionalRequirement) extractToolHistory(inv *Invocation) (stepCount int, invocationsOfTarget int, lastToolUsed string, toolsUsed map[string]bool) {
	// Step 1 is the first turn. Each assistant message with a tool call increments the step.
	stepCount = 1
	toolsUsed = make(map[string]bool)

	for _, msg := range inv.History {
		if msg.Role == RoleUser {
			stepCount = 1 // Reset step count for new user turn
		} else if msg.Role == RoleAssistant {
			stepCount++
			for _, content := range msg.Content {
				if content.Type == ContentTypeToolCall && content.ToolCall != nil {
					lastToolUsed = content.ToolCall.Name
					toolsUsed[lastToolUsed] = true
					if lastToolUsed == c.TargetTool {
						invocationsOfTarget++
					}
				}
			}
		}
	}
	return
}

func (c *ConditionalRequirement) Run(ctx context.Context, inv *Invocation) ([]Rule, error) {
	rule := Rule{
		TargetTool: c.TargetTool,
		Allowed:    true,
		Forced:     false,
	}

	stepCount, invocationsOfTarget, lastToolUsed, toolsUsed := c.extractToolHistory(inv)

	// 1. Max Invocations Check — hard limit, disallows tool entirely
	if c.MaxInvocations >= 0 && invocationsOfTarget >= c.MaxInvocations {
		rule.Allowed = false
		return []Rule{rule}, nil
	}

	// 2. OnlyAfter Check — tool is not available until prerequisites are met
	if len(c.OnlyAfter) > 0 {
		for _, requiredTool := range c.OnlyAfter {
			if !toolsUsed[requiredTool] {
				rule.Allowed = false
				return []Rule{rule}, nil
			}
		}
	}

	// 3. Determine if the tool should be forced this turn.
	//    We compute forcing BEFORE the consecutive check so that
	//    consecutive prevention can correctly override forcing within
	//    the same requirement.
	forced := false

	// 3a. ForceAtStep Check
	if c.ForceAtStep > 0 && stepCount == c.ForceAtStep {
		forced = true
	}

	// 3b. ForceAfter Check — force after specific tools
	if len(c.ForceAfter) > 0 && lastToolUsed != "" {
		for _, triggerTool := range c.ForceAfter {
			if lastToolUsed == triggerTool {
				forced = true
				break
			}
		}
	}

	// 3c. ForceAfterAny Check — force after ANY tool (ReAct pattern)
	//     Only fires if the last tool was a different tool (not the target itself,
	//     since consecutive prevention typically accompanies this option).
	if c.ForceAfterAny && lastToolUsed != "" && lastToolUsed != c.TargetTool {
		forced = true
	}

	// 4. Consecutive Check — prevents the same tool from being called twice in a row.
	//    This overrides forcing: if the target was just used, it cannot be forced again.
	if !c.ConsecutiveAllowed && lastToolUsed == c.TargetTool {
		rule.Allowed = false
		rule.Forced = false
		return []Rule{rule}, nil
	}

	// 5. Apply forcing
	if forced {
		rule.Forced = true
		rule.Allowed = true
	}

	// 6. Prevent Stop (if min invocations not yet met)
	if c.MinInvocations > 0 && invocationsOfTarget < c.MinInvocations {
		rule.PreventStop = true
	}

	return []Rule{rule}, nil
}
