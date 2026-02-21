package agent

import (
	"fmt"
	"sort"
)

// RequirementMiddleware creates a Chat-level Middleware that acts as a
// declarative Reasoner — evaluating all Requirements, resolving conflicts
// by priority, and enforcing the result on the LLM call.
//
// This is the Go equivalent of BeeAI's RequirementsReasoner.
//
// Resolution algorithm (per tool):
//  1. Collect all Rules from all enabled Requirements
//  2. Group rules by TargetTool
//  3. Allowed=false is a hard veto — always wins regardless of priority
//  4. Hidden=true is sticky — any rule can hide a tool
//  5. Forced=true resolved by highest priority when conflicting
//  6. PreventStop accumulates — any rule can prevent stopping
//  7. Reasons are collected from highest-priority rule per tool
func RequirementMiddleware(reqs ...Requirement) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if !c.IsChat() {
				return next.Handle(c)
			}

			// Build an Invocation from the current Chat context.
			inv := &Invocation{
				History: make([]*Message, len(c.Chat.Messages)),
				Tools:   c.Chat.Tools,
			}
			for i, m := range c.Chat.Messages {
				inv.History[i] = m.Clone()
			}

			// 1. Evaluate all enabled Requirements → collect Rules.
			var allRules []Rule
			for _, req := range reqs {
				// Skip disabled requirements.
				if e, ok := req.(Enableable); ok && !e.Enabled() {
					continue
				}
				rules, err := req.Run(c, inv)
				if err != nil {
					return fmt.Errorf("requirement evaluation failed: %w", err)
				}
				allRules = append(allRules, rules...)
			}

			if len(allRules) == 0 {
				return next.Handle(c)
			}

			// 2. Resolve rules into a RequirementRequest.
			request := resolveRules(allRules, c.Chat.Tools)

			// 3. Apply the resolved request to the Chat context.
			applyRequest(c, request)

			return next.Handle(c)
		})
	}
}

// resolveRules aggregates rules per tool and resolves conflicts by priority.
// This is the core "Reasoner" algorithm inspired by BeeAI's RequirementsReasoner.
func resolveRules(rules []Rule, tools []Tool) RequirementRequest {
	// Group rules by target tool.
	type toolRules struct {
		rules []Rule
	}
	grouped := make(map[string]*toolRules)
	for _, r := range rules {
		if _, ok := grouped[r.TargetTool]; !ok {
			grouped[r.TargetTool] = &toolRules{}
		}
		grouped[r.TargetTool].rules = append(grouped[r.TargetTool].rules, r)
	}

	// Resolve per-tool state.
	type resolvedTool struct {
		allowed     bool
		forced      bool
		hidden      bool
		preventStop bool
		reason      string
	}
	resolved := make(map[string]*resolvedTool)

	for toolName, tr := range grouped {
		rt := &resolvedTool{allowed: true}

		// Sort rules by priority descending for forced/reason resolution.
		sorted := make([]Rule, len(tr.rules))
		copy(sorted, tr.rules)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Priority > sorted[j].Priority
		})

		for _, r := range sorted {
			// Blocked=true is a hard veto — always wins.
			if r.Blocked {
				rt.allowed = false
			}
			// Hidden is sticky — any rule can hide.
			if r.Hidden {
				rt.hidden = true
			}
			// PreventStop accumulates.
			if r.PreventStop {
				rt.preventStop = true
			}
		}

		// Forced and Reason come from the highest-priority rule.
		if len(sorted) > 0 && sorted[0].Forced && rt.allowed {
			rt.forced = true
		}
		if len(sorted) > 0 && sorted[0].Reason != "" {
			rt.reason = sorted[0].Reason
		}

		resolved[toolName] = rt
	}

	// Build the RequirementRequest.
	req := RequirementRequest{
		CanFinish: true,
		Reasons:   make(map[string]string),
	}

	// Collect global PreventStop.
	for _, rt := range resolved {
		if rt.preventStop {
			req.CanFinish = false
			break
		}
	}

	// Collect forced tools.
	for toolName, rt := range resolved {
		if rt.forced {
			req.ForcedTools = append(req.ForcedTools, toolName)
		}
		if rt.reason != "" {
			req.Reasons[toolName] = rt.reason
		}
	}

	// Filter tools.
	for _, t := range tools {
		rt, hasRules := resolved[t.Name()]
		if !hasRules {
			// No rules for this tool — allowed by default.
			req.AllowedTools = append(req.AllowedTools, t)
			continue
		}
		if rt.hidden {
			req.HiddenTools = append(req.HiddenTools, t.Name())
			continue
		}
		if !rt.allowed {
			continue // blocked
		}
		req.AllowedTools = append(req.AllowedTools, t)
	}

	// Determine ToolChoice.
	switch len(req.ForcedTools) {
	case 0:
		if !req.CanFinish {
			req.ToolChoice = "required"
		}
	case 1:
		req.ToolChoice = req.ForcedTools[0]
	default:
		req.ToolChoice = "required"
	}

	return req
}

// applyRequest enforces the RequirementRequest on the Chat context.
func applyRequest(c *Context, req RequirementRequest) {
	// Ensure ChatOptions exists.
	if c.Chat.Options == nil {
		c.Chat.Options = &ChatOptions{}
	}

	// Apply tool filtering.
	if len(req.ForcedTools) > 0 {
		// When tools are forced, restrict to only forced tools.
		forced := make(map[string]bool, len(req.ForcedTools))
		for _, name := range req.ForcedTools {
			forced[name] = true
		}
		var forcedTools []Tool
		for _, t := range req.AllowedTools {
			if forced[t.Name()] {
				forcedTools = append(forcedTools, t)
			}
		}
		if len(forcedTools) > 0 {
			c.Chat.Tools = forcedTools
		} else {
			c.Chat.Tools = req.AllowedTools
		}

		// Build instruction with reasons.
		instr := "CRITICAL SYSTEM INSTRUCTION: You MUST use one of the following tools immediately: "
		for i, name := range req.ForcedTools {
			if i > 0 {
				instr += ", "
			}
			instr += "'" + name + "'"
			if reason, ok := req.Reasons[name]; ok {
				instr += " (" + reason + ")"
			}
		}
		c.Chat.Messages = append([]Message{*NewSystemMessage(instr)}, c.Chat.Messages...)
	} else {
		c.Chat.Tools = req.AllowedTools
	}

	// Apply ToolChoice.
	if req.ToolChoice != "" {
		c.Chat.Options.ToolChoice = req.ToolChoice
	}

	// Apply PreventStop.
	if !req.CanFinish && len(req.ForcedTools) == 0 {
		c.Chat.Messages = append([]Message{*NewSystemMessage(
			"CRITICAL SYSTEM INSTRUCTION: You cannot provide a final answer yet. You must continue using tools.",
		)}, c.Chat.Messages...)
	}
}
