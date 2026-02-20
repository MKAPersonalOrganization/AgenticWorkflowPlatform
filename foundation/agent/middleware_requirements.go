package agent

import (
	"fmt"
)

// RequirementMiddleware creates a Middleware that enforces deterministic rules
// before the Invocation reaches the LLM adapter (ChatOp).
//
// The middleware evaluates all Requirements against the current conversation state,
// aggregates the resulting Rules, and enforces them by:
//   - Filtering c.Chat.Tools (removing disallowed tools)
//   - Restricting c.Chat.Tools to only forced tools (when applicable)
//   - Setting c.Chat.Options.ToolChoice to force specific tool usage
//   - Prepending system messages as additional enforcement
//   - Preventing early stopping when requirements are not yet satisfied
func RequirementMiddleware(reqs ...Requirement) Middleware {
	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			if !c.IsChat() {
				return next.Handle(c)
			}

			// Build an Invocation from the current Chat context for requirement evaluation.
			inv := &Invocation{
				History: make([]*Message, len(c.Chat.Messages)),
				Tools:   c.Chat.Tools,
			}
			for i, m := range c.Chat.Messages {
				inv.History[i] = m.Clone()
			}

			// 1. Evaluate all Requirements.
			var allRules []Rule
			for _, req := range reqs {
				rules, err := req.Run(c, inv)
				if err != nil {
					return fmt.Errorf("requirement evaluation failed: %w", err)
				}
				allRules = append(allRules, rules...)
			}

			// 2. Aggregate rules per tool.
			//    Conflict resolution: forbidden overrides forced, PreventStop accumulates.
			type toolState struct {
				Allowed, Forced bool
			}
			states := make(map[string]*toolState)
			for _, t := range c.Chat.Tools {
				states[t.Name()] = &toolState{Allowed: true}
			}

			globalPreventStop := false
			var forcedToolNames []string

			for _, r := range allRules {
				if r.PreventStop {
					globalPreventStop = true
				}
				if r.Forced {
					forcedToolNames = append(forcedToolNames, r.TargetTool)
				}
				if state, ok := states[r.TargetTool]; ok {
					if !r.Allowed {
						state.Allowed = false
					}
					if r.Forced {
						state.Forced = true
					}
				}
			}

			// Remove forced names that are also forbidden.
			forbidden := make(map[string]bool)
			for name, st := range states {
				if !st.Allowed {
					forbidden[name] = true
				}
			}
			{
				clean := forcedToolNames[:0]
				for _, n := range forcedToolNames {
					if !forbidden[n] {
						clean = append(clean, n)
					}
				}
				forcedToolNames = clean
			}

			// 3. Build filtered and forced tool lists.
			var filteredTools, forcedTools []Tool
			for _, t := range c.Chat.Tools {
				st := states[t.Name()]
				if !st.Allowed {
					continue
				}
				filteredTools = append(filteredTools, t)
				if st.Forced {
					forcedTools = append(forcedTools, t)
				}
			}

			// Ensure ChatOptions exists for writing ToolChoice.
			if c.Chat.Options == nil {
				c.Chat.Options = &ChatOptions{}
			}

			// 4. Apply forced tools: restrict available tools + set ToolChoice.
			if len(forcedTools) > 0 {
				c.Chat.Tools = forcedTools

				instr := "CRITICAL SYSTEM INSTRUCTION: You MUST use one of the following tools immediately: "
				for i, ft := range forcedTools {
					if i > 0 {
						instr += ", "
					}
					instr += "'" + ft.Name() + "'"
				}
				c.Chat.Messages = append([]Message{*NewSystemMessage(instr)}, c.Chat.Messages...)

				if len(forcedTools) == 1 {
					c.Chat.Options.ToolChoice = forcedTools[0].Name()
				} else {
					c.Chat.Options.ToolChoice = "required"
				}
			} else if len(forcedToolNames) > 0 {
				// Forced by name but tool not in the tools list — still enforce.
				instr := "CRITICAL SYSTEM INSTRUCTION: You MUST use one of the following tools immediately: "
				for i, name := range forcedToolNames {
					if i > 0 {
						instr += ", "
					}
					instr += "'" + name + "'"
				}
				c.Chat.Messages = append([]Message{*NewSystemMessage(instr)}, c.Chat.Messages...)

				if len(forcedToolNames) == 1 {
					c.Chat.Options.ToolChoice = forcedToolNames[0]
				} else {
					c.Chat.Options.ToolChoice = "required"
				}
			} else if len(filteredTools) < len(c.Chat.Tools) {
				// Some tools filtered out, none forced.
				c.Chat.Tools = filteredTools
			}

			// 5. PreventStop: block the agent from finishing prematurely.
			if globalPreventStop && len(forcedTools) == 0 && len(forcedToolNames) == 0 {
				c.Chat.Messages = append([]Message{*NewSystemMessage(
					"CRITICAL SYSTEM INSTRUCTION: You cannot provide a final answer yet. You must continue using tools.",
				)}, c.Chat.Messages...)
				c.Chat.Options.ToolChoice = "required"
			}

			return next.Handle(c)
		})
	}
}
