package agent

import (
	"encoding/json"
)

// UnmarshalArgs is a helper to parse JSON string arguments into a struct.
func UnmarshalArgs(args string, target any) error {
	if args == "" {
		return nil
	}
	return json.Unmarshal([]byte(args), target)
}

// ToolSet acts as a registry for collecting tools.
type ToolSet struct {
	tools []Tool
}

// NewToolSet creates a new tool set.
func NewToolSet() *ToolSet {
	return &ToolSet{
		tools: make([]Tool, 0),
	}
}

// Register adds a tool to the set.
func (ts *ToolSet) Register(t Tool) {
	ts.tools = append(ts.tools, t)
}

// AddAll adds multiple tools to the set.
func (ts *ToolSet) AddAll(tools []Tool) {
	ts.tools = append(ts.tools, tools...)
}

// All returns all registered tools.
func (ts *ToolSet) All() []Tool {
	return ts.tools
}
