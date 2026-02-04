// Package blades provides an adapter for the Go-Kratos Blades agent framework.
package blades

import (
	"context"

	"school-with-ai/foundation/agent"
)

// WrapAgent wraps a Blades agent to implement agent.Agent.
func WrapAgent(bladesAgent any) agent.Agent {
	return &AgentAdapter{
		bladesAgent: bladesAgent,
	}
}

// AgentAdapter adapts a Blades agent to the agent.Agent interface.
type AgentAdapter struct {
	bladesAgent any
}

// Name returns the agent's identifier.
func (a *AgentAdapter) Name() string {
	return ""
}

// Description returns the agent's description.
func (a *AgentAdapter) Description() string {
	return ""
}

// Run executes the agent.
func (a *AgentAdapter) Run(ctx context.Context, inv *agent.Invocation) agent.Stream {
	return func(yield func(agent.Event, error) bool) {
		yield(nil, &NotImplementedError{"Blades adapter not yet implemented"})
	}
}

// Tools returns the agent's tools.
func (a *AgentAdapter) Tools() []agent.Tool {
	return nil
}

// SubAgents returns child agents.
func (a *AgentAdapter) SubAgents() []agent.Agent {
	return nil
}

// WrapTool wraps a Blades tool to implement agent.Tool.
func WrapTool(bladesTool any) agent.Tool {
	return &ToolAdapter{
		bladesTool: bladesTool,
	}
}

// ToolAdapter adapts a Blades tool to the agent.Tool interface.
type ToolAdapter struct {
	bladesTool any
}

func (t *ToolAdapter) Name() string                { return "" }
func (t *ToolAdapter) Description() string         { return "" }
func (t *ToolAdapter) InputSchema() map[string]any { return nil }
func (t *ToolAdapter) Execute(ctx context.Context, tctx *agent.ToolContext) (*agent.ToolResult, error) {
	return nil, &NotImplementedError{"Blades tool adapter not yet implemented"}
}

// WrapSession wraps a Blades session to implement agent.Session.
func WrapSession(bladesSession any) agent.Session {
	return &SessionAdapter{
		bladesSession: bladesSession,
	}
}

// SessionAdapter adapts a Blades session to the agent.Session interface.
type SessionAdapter struct {
	bladesSession any
	state         *StateAdapter
}

func (s *SessionAdapter) ID() string                { return "" }
func (s *SessionAdapter) AppName() string           { return "" }
func (s *SessionAdapter) UserID() string            { return "" }
func (s *SessionAdapter) State() agent.State        { return s.state }
func (s *SessionAdapter) History() []*agent.Message { return nil }
func (s *SessionAdapter) Append(ctx context.Context, msg *agent.Message) error {
	return &NotImplementedError{"Blades session append not yet implemented"}
}
func (s *SessionAdapter) LastUpdateTime() int64 { return 0 }

// StateAdapter adapts a Blades state to the agent.State interface.
type StateAdapter struct {
	data map[string]any
}

func (st *StateAdapter) Get(key string) (any, error) {
	v, ok := st.data[key]
	if !ok {
		return nil, agent.ErrStateKeyNotFound
	}
	return v, nil
}

func (st *StateAdapter) Set(key string, value any) error {
	if st.data == nil {
		st.data = make(map[string]any)
	}
	st.data[key] = value
	return nil
}

func (st *StateAdapter) Delete(key string) error {
	delete(st.data, key)
	return nil
}

func (st *StateAdapter) Keys() []string {
	keys := make([]string, 0, len(st.data))
	for k := range st.data {
		keys = append(keys, k)
	}
	return keys
}

func (st *StateAdapter) All() func(yield func(key string, value any) bool) {
	return func(yield func(key string, value any) bool) {
		for k, v := range st.data {
			if !yield(k, v) {
				return
			}
		}
	}
}

// NotImplementedError indicates a feature is not yet implemented.
type NotImplementedError struct {
	Feature string
}

func (e *NotImplementedError) Error() string {
	return "not implemented: " + e.Feature
}
