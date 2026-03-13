package adk

import (
	"context"
	"iter"

	"school-with-ai/foundation/agent"
)

// Adapter wraps an ADK agent to implement our Agent interface.
// This is a thin adapter that delegates to the foundation/agent/adapters/adkgo
// package for full ADK-Go support.
type Adapter struct {
	config agent.Config
}

// NewAgent creates a new agent using the ADK framework.
func NewAgent(cfg agent.Config) (agent.Agent, error) {
	return &Adapter{
		config: cfg,
	}, nil
}

// ID returns the agent's unique identifier.
func (a *Adapter) ID() agent.ID {
	return a.config.Name
}

func (a *Adapter) Name() string {
	return a.config.Name
}

func (a *Adapter) Description() string {
	return a.config.Description
}

func (a *Adapter) SubAgents() []agent.Agent {
	return a.config.SubAgents
}

func (a *Adapter) Run(ctx context.Context, input *agent.Invocation) iter.Seq2[agent.Event, error] {
	return func(yield func(agent.Event, error) bool) {
		// Delegation to ADK agent would happen here.
		// For production use, see adapters/adkgo/ which provides full
		// ADK-Go runner integration with session, tools, and streaming.
	}
}

// Config returns the agent's configuration.
func (a *Adapter) Config() agent.Config {
	return a.config
}

// Compile-time interface verification.
var (
	_ agent.Agent        = (*Adapter)(nil)
	_ agent.Configurable = (*Adapter)(nil)
)
