package toolreqs

import (
	"fmt"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agent/io"
)

// =============================================================================
// Confirmation Middleware — runtime enforcement via foundation primitives
// =============================================================================

// ConfirmationMiddleware returns a [agent.Middleware] that enforces user
// confirmation for tools configured in the [ConfirmationRequirement].
//
// This bridges the toolreqs policy layer with the foundation's middleware
// system. At execution time, the middleware:
//
//  1. Checks if the tool requires confirmation via [ConfirmationRequirement].
//  2. If yes, calls [io.Confirm] which triggers the pause/resume flow
//     (via PauseHandler in web mode, StdinHandler in CLI, etc.).
//  3. If the user denies, sets a denial result on the context.
//  4. If the user approves, proceeds with normal execution.
//
// The middleware is scoped to a specific agent so that agent-specific
// policies are resolved correctly.
func ConfirmationMiddleware(cr *ConfirmationRequirement, agentID string) agent.Middleware {
	return func(next agent.Handler) agent.Handler {
		return agent.HandlerFunc(func(c *agent.Context) error {
			// Only intercept tool (function) calls.
			if !c.IsFunction() {
				return next.Handle(c)
			}

			// Extract tool name.
			toolName := ""
			if c.Function != nil && c.Function.Tool != nil {
				toolName = c.Function.Tool.Name()
			}

			// Extract arguments.
			var args map[string]any
			if c.Function != nil {
				args = c.Function.Arguments
			}

			// Check if this tool requires confirmation.
			if !cr.RequiresConfirmation(agentID, toolName, args) {
				return next.Handle(c)
			}

			// Build confirmation prompt.
			hint := cr.getHint(agentID, toolName)

			// Ask for permission via IO handler.
			// This triggers the pause/resume flow:
			//   - PauseHandler (web): returns ErrAuthRequired → engine pauses
			//   - StdinHandler (CLI): blocks on stdin → user types y/n
			//   - NoopHandler (test): auto-approves
			approved, err := io.Confirm(c.Context, hint, toolName)
			if err != nil {
				// Sentinel errors (ErrAuthRequired, ErrInputRequired)
				// propagate up to the engine for pause handling.
				return err
			}

			if !approved {
				// User denied — return a denial result without executing the tool.
				c.Result = map[string]any{
					"result": fmt.Sprintf("Tool '%s' execution was denied by the user.", toolName),
					"denied": true,
					"tool":   toolName,
				}
				return nil
			}

			// User approved — proceed with tool execution.
			return next.Handle(c)
		})
	}
}
