package agent

import (
	"context"
	"fmt"
	"school-with-ai/foundation/agentsdk/io"
)

// =============================================================================
// AskPermissionRequirement — BeeAI's AskPermissionRequirement for Go
// =============================================================================
//
// In BeeAI (Python), AskPermissionRequirement registers a blocking event
// listener on tool "start" events and calls io_confirm() before the tool
// runs. In our Go framework, the equivalent is FunctionMiddleware that
// intercepts tool execution.
//
// This decouples permission logic from tools themselves. Permission is
// a cross-cutting concern managed by middleware, not embedded in each tool.
//
// Usage (declarative on agent config):
//
//	config := core.AgentConfig{
//	    FunctionMiddleware: []agent.Middleware{
//	        agent.AskPermission("web_search", "file_write"),
//	    },
//	}
//
// Usage (approve ALL tools):
//
//	agent.AskPermission() // empty = all tools require approval
//
// The middleware calls io.Confirm(ctx, prompt, toolName):
//   - With PauseHandler (web): returns ErrAuthRequired → engine pauses
//   - With StdinHandler (CLI): blocks on stdin → user types y/n
//   - With NoopHandler (test): auto-approves → tool runs immediately
//   - With no handler: auto-approves (fail-open default)

// AskPermission returns FunctionMiddleware that requests user approval
// before executing the specified tools.
//
// If toolNames is empty, ALL tools require approval.
// If toolNames is provided, only those tools require approval.
//
// This is the Go equivalent of BeeAI's AskPermissionRequirement.
func AskPermission(toolNames ...string) Middleware {
	allowed := make(map[string]bool, len(toolNames))
	for _, name := range toolNames {
		allowed[name] = true
	}
	allTools := len(toolNames) == 0

	return func(next Handler) Handler {
		return HandlerFunc(func(c *Context) error {
			// Only intercept function (tool) calls
			if !c.IsFunction() {
				return next.Handle(c)
			}

			// Get tool name
			toolName := ""
			if c.Function != nil && c.Function.Tool != nil {
				toolName = c.Function.Tool.Name()
			}

			// Check if this tool needs approval
			if !allTools && !allowed[toolName] {
				return next.Handle(c)
			}

			// Build approval prompt
			prompt := fmt.Sprintf("Allow tool '%s' to execute?", toolName)
			if toolName == "" {
				prompt = "Allow tool execution?"
			}

			// Ask for permission via IO handler
			approved, err := io.Confirm(c.Context, prompt, toolName)
			if err != nil {
				// Sentinel errors (ErrAuthRequired, ErrInputRequired)
				// propagate up to the engine for pause handling.
				return err
			}

			if !approved {
				// User denied — return a denial result without executing the tool.
				// We set Result on the context so the caller can read it.
				c.Result = map[string]any{
					"result": fmt.Sprintf("Tool '%s' execution was denied by the user.", toolName),
					"denied": true,
					"tool":   toolName,
				}
				return nil
			}

			// User approved — proceed with tool execution
			return next.Handle(c)
		})
	}
}

// =============================================================================
// AskPermissionRequirement — Declarative Requirement wrapper
// =============================================================================

// AskPermissionRequirement wraps AskPermission as a Requirement for
// declarative agent configuration.
//
// It implements:
//   - Requirement: returns no rules (doesn't constrain tool availability)
//   - FunctionMiddlewareProvider: provides the approval middleware
//   - PromptProvider: adds a system prompt note about approval
//
// This allows using AskPermission in the Requirements list alongside
// other requirements like ConditionalRequirement:
//
//	config := core.AgentConfig{
//	    Requirements: []agent.Requirement{
//	        agent.NewAskPermissionRequirement("web_search"),
//	        agent.NewConditionalRequirement("calculator", agent.MaxInvocations(5)),
//	    },
//	}
type AskPermissionRequirement struct {
	toolNames  []string
	middleware Middleware
}

// NewAskPermissionRequirement creates a new AskPermissionRequirement.
// If toolNames is empty, all tools require approval.
func NewAskPermissionRequirement(toolNames ...string) *AskPermissionRequirement {
	return &AskPermissionRequirement{
		toolNames:  toolNames,
		middleware: AskPermission(toolNames...),
	}
}

// Run implements Requirement — returns no rules.
// AskPermission doesn't constrain tool availability; it intercepts execution.
func (r *AskPermissionRequirement) Run(_ context.Context, _ *Invocation) ([]Rule, error) {
	return nil, nil
}

// FunctionMiddleware returns the approval middleware.
// The engine should check for this interface on Requirements and include
// the returned middleware in the FunctionMiddleware chain.
func (r *AskPermissionRequirement) FunctionMiddleware() Middleware {
	return r.middleware
}

// PromptAddition implements PromptProvider — adds a note about tool approval.
func (r *AskPermissionRequirement) PromptAddition() string {
	if len(r.toolNames) == 0 {
		return "Note: All tool executions require user approval before running."
	}
	return fmt.Sprintf("Note: The following tools require user approval: %s",
		joinToolNames(r.toolNames))
}

// Verify interface compliance.
var (
	_ Requirement    = (*AskPermissionRequirement)(nil)
	_ PromptProvider = (*AskPermissionRequirement)(nil)
)

// FunctionMiddlewareProvider is an optional interface that Requirements can
// implement to inject FunctionMiddleware into the tool execution pipeline.
//
// This bridges the gap between Requirements (evaluated before LLM calls)
// and FunctionMiddleware (evaluated during tool execution). Some concerns
// like "ask permission" span both phases.
type FunctionMiddlewareProvider interface {
	// FunctionMiddleware returns middleware to wrap tool execution.
	FunctionMiddleware() Middleware
}

// joinToolNames formats tool names for display.
func joinToolNames(names []string) string {
	if len(names) == 0 {
		return "all tools"
	}
	result := ""
	for i, name := range names {
		if i > 0 {
			result += ", "
		}
		result += name
	}
	return result
}
