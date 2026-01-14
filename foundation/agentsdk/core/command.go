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
}

// IsCommand checks if a tool result is a Command
func IsCommand(result any) (*Command, bool) {
	cmd, ok := result.(*Command)
	return cmd, ok
}
