package custom

import (
	"context"
	"fmt"
	"iter"

	"school-with-ai/foundation/agent"
)

// CustomAgent is a concrete implementation of agent.Agent that holds the configuration.
// It is used by the custom adapter to manage agents without a global registry.
type CustomAgent struct {
	config agent.Config
}

// NewAgent creates a new CustomAgent from the given configuration.
func NewAgent(cfg agent.Config) (agent.Agent, error) {
	return &CustomAgent{config: cfg}, nil
}

// ID returns the agent's unique identifier.
func (a *CustomAgent) ID() agent.ID {
	return a.config.Name // Use name as ID for now
}

// Name returns the agent's name.
func (a *CustomAgent) Name() string {
	return a.config.Name
}

// Description returns the agent's description.
func (a *CustomAgent) Description() string {
	return a.config.Description
}

// Run executes the agent.
// Note: This implementation currently returns an error because execution logic
// is handled by the RunnerAdapter/Engine. In the future, we might inject the runner here.
func (a *CustomAgent) Run(ctx context.Context, inv *agent.Invocation) iter.Seq2[agent.Event, error] {
	return func(yield func(agent.Event, error) bool) {
		yield(nil, fmt.Errorf("Run not directly supported on CustomAgent; use RunnerAdapter"))
	}
}

// AgentMiddlewares returns the agent's run middlewares.
func (a *CustomAgent) AgentMiddlewares() []agent.Middleware {
	return a.config.AgentMiddlewares
}

// ChatMiddlewares returns the agent's chat middlewares.
func (a *CustomAgent) ChatMiddlewares() []agent.Middleware {
	return a.config.ChatMiddlewares
}

// FunctionMiddlewares returns the agent's function middlewares.
func (a *CustomAgent) FunctionMiddlewares() []agent.Middleware {
	return a.config.FunctionMiddlewares
}

// Tools returns the agent's tools.
func (a *CustomAgent) Tools() []agent.Tool {
	return a.config.Tools
}

// Config returns the underlying configuration.
// This is used by the adapter to extract config for the engine.
func (a *CustomAgent) Config() agent.Config {
	return a.config
}
