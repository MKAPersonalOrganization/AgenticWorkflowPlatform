package core

// Command represents a routing command returned from tools.
// This follows the LangGraph pattern where tools can return Commands
// to control routing and state updates instead of just returning strings.
type Command struct {
	// Goto specifies the target agent/node to route to
	Goto string

	// Update contains state updates to apply (e.g., handoff information)
	Update map[string]any

	// StopIteration indicates the engine should stop iterating
	// This is set to true for handoff tools to prevent infinite loops
	StopIteration bool

	// Elicitation specifies a request for structured user input
	Elicitation *ElicitationSpec

	// Authorization specifies a request for user authorization
	Authorization *AuthorizationSpec
}

// ElicitationSpec defines the structure for user input requests
type ElicitationSpec struct {
	Question string         `json:"question"`
	Schema   map[string]any `json:"schema,omitempty"`
}

// AuthorizationSpec defines a request for user authorization.
type AuthorizationSpec struct {
	Resource string   `json:"resource"` // What's being accessed
	Action   string   `json:"action"`   // What action is requested
	Reason   string   `json:"reason"`   // Why this is needed
	Scopes   []string `json:"scopes"`   // Required permission scopes
}

// ToolResult represents the result of a tool invocation.
// It contains both the display result and optional routing command.
type ToolResult struct {
	// Result is the result to display/add to conversation (marshaled to JSON)
	Result any

	// Command is an optional routing command (nil for regular tools)
	Command *Command
}

// NewToolResult creates a simple tool result with just a result.
func NewToolResult(result any) ToolResult {
	return ToolResult{
		Result:  result,
		Command: nil,
	}
}

// NewToolResultWithCommand creates a tool result with both result and command.
func NewToolResultWithCommand(result any, cmd *Command) ToolResult {
	return ToolResult{
		Result:  result,
		Command: cmd,
	}
}

// HasCommand returns true if this result contains a routing command.
func (r ToolResult) HasCommand() bool {
	return r.Command != nil
}

// StopIteration returns true if this result should stop iteration.
func (r ToolResult) StopIteration() bool {
	return r.Command != nil && r.Command.StopIteration
}
