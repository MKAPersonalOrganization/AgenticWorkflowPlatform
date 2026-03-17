package adkgo

import (
	"context"
	"fmt"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/genai"

	fagent "school-with-ai/foundation/agent"
)

// ──────────────────────────────────────────────────────────────────────────────
// Requirement Evaluation
//
// Requirements are evaluated once per agent invocation to produce Rules that
// control tool visibility, force tool use, and inject LLM guidance.
// Evaluating once and reusing the results avoids double-evaluation of
// History-dependent requirements (which inspect the full conversation).
// ──────────────────────────────────────────────────────────────────────────────

// evaluateRequirements runs all requirements in agentCfg and returns the
// aggregated rules. Disabled requirements (Enableable.Enabled() == false)
// are skipped. Errors from individual requirements are silently ignored to
// avoid blocking agent execution due to a single failing requirement.
func evaluateRequirements(ctx context.Context, agentCfg *fagent.Config, inv *fagent.Invocation) []fagent.Rule {
	if agentCfg == nil {
		return nil
	}
	var rules []fagent.Rule
	for _, req := range agentCfg.Requirements {
		if e, ok := req.(fagent.Enableable); ok && !e.Enabled() {
			continue
		}
		r, rErr := req.Run(ctx, inv)
		if rErr != nil {
			continue
		}
		rules = append(rules, r...)
	}
	return rules
}

// ──────────────────────────────────────────────────────────────────────────────
// Requirement Guidance Injection
//
// Pre-computed rules are injected into the LLM turn as a synthetic user-role
// Content message via a BeforeAgentCallback. ADK-Go surfaces this content to
// the model before each agent run, making constraints visible to the LLM.
// ──────────────────────────────────────────────────────────────────────────────

// buildBeforeRunHooks creates ADK-Go BeforeAgentCallbacks that inject
// pre-computed requirement guidance into the LLM turn as a user-role Content
// message. Requirements are evaluated once in the caller and the resulting
// rules are passed in — no double evaluation.
//
// Returns nil if no rules produce guidance text (no callbacks needed).
func buildBeforeRunHooks(rules []fagent.Rule) []agent.BeforeAgentCallback {
	if len(rules) == 0 {
		return nil
	}

	var guidance strings.Builder
	for _, rule := range rules {
		if rule.Blocked && rule.Reason != "" {
			fmt.Fprintf(&guidance, "IMPORTANT: The tool '%s' is currently disabled: %s\n", rule.TargetTool, rule.Reason)
		}
		if rule.Forced && rule.Reason != "" {
			fmt.Fprintf(&guidance, "IMPORTANT: You MUST use the tool '%s': %s\n", rule.TargetTool, rule.Reason)
		}
		if rule.PreventStop && rule.Reason != "" {
			fmt.Fprintf(&guidance, "IMPORTANT: You must not stop yet: %s\n", rule.Reason)
		}
	}

	if guidance.Len() == 0 {
		return nil
	}

	guidanceText := guidance.String()
	return []agent.BeforeAgentCallback{
		func(_ agent.CallbackContext) (*genai.Content, error) {
			return &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: guidanceText}},
			}, nil
		},
	}
}
