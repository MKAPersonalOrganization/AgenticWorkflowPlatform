package custom

import (
	"context"
	"fmt"

	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/agentsdk/registry"
)

// RegistryAdapter adapts registry.AgentRegistry to agent.AgentRegistry.
// It bridges the factory-based agentsdk registry to the abstraction layer interface.
type RegistryAdapter struct {
	sdkRegistry *registry.AgentRegistry
	runner      agent.Runner
}

// NewRegistryAdapter creates an adapter for the SDK registry.
// The runner is used to create agents from configs when Get is called.
func NewRegistryAdapter(sdkRegistry *registry.AgentRegistry, runner agent.Runner) agent.AgentRegistry {
	return &RegistryAdapter{
		sdkRegistry: sdkRegistry,
		runner:      runner,
	}
}

// Get retrieves an agent by name.
// Since the SDK registry contains factories (not agents), this creates
// a temporary wrapper that defers to the runner's config-based execution.
func (r *RegistryAdapter) Get(name string) (agent.Agent, error) {
	factory, ok := r.sdkRegistry.Get(name)
	if !ok {
		return nil, fmt.Errorf("agent %q not found", name)
	}

	// Create config from factory
	config := factory(context.Background())

	// Return a wrapper that uses the config
	return &configAgentWrapper{
		name:   name,
		config: config,
		runner: r.runner,
	}, nil
}

// List returns all available agent names.
func (r *RegistryAdapter) List() []string {
	return r.sdkRegistry.List()
}

// Register adds an agent to the registry.
// Note: This is not supported for the SDK registry which uses factories.
func (r *RegistryAdapter) Register(ag agent.Agent) error {
	return fmt.Errorf("direct agent registration not supported; use SDK registry.Register with a factory")
}

// configAgentWrapper wraps an agent.Config to implement agent.Agent.
// This allows the registry adapter to return agents even though the
// underlying registry contains config factories.
type configAgentWrapper struct {
	name   string
	config agent.Config
	runner agent.Runner
}

func (w *configAgentWrapper) Name() string {
	return w.name
}

func (w *configAgentWrapper) Description() string {
	return w.config.Description
}

func (w *configAgentWrapper) Run(ctx context.Context, inv *agent.Invocation) agent.Stream {
	// Use the runner to execute with the config
	if w.runner == nil {
		return func(yield func(agent.Event, error) bool) {
			yield(nil, fmt.Errorf("no runner configured for agent %q", w.name))
		}
	}
	// Convert Invocation to RunRequest
	req := &agent.RunRequest{
		AgentName: w.name,
		SessionID: inv.SessionID,
		UserID:    inv.UserID,
		Message:   inv.Message,
		Context:   inv.Context,
	}
	return w.runner.Run(ctx, req)
}

func (w *configAgentWrapper) Tools() []agent.Tool {
	return w.config.Tools
}

func (w *configAgentWrapper) SubAgents() []agent.Agent {
	return w.config.SubAgents
}

// ListOnlyRegistryAdapter is a simpler adapter that only supports List.
// Use this when you only need to list available agents (like in handlers).
type ListOnlyRegistryAdapter struct {
	sdkRegistry *registry.AgentRegistry
}

// NewListOnlyRegistryAdapter creates an adapter that only supports List.
func NewListOnlyRegistryAdapter(sdkRegistry *registry.AgentRegistry) agent.AgentRegistry {
	return &ListOnlyRegistryAdapter{sdkRegistry: sdkRegistry}
}

func (r *ListOnlyRegistryAdapter) Get(name string) (agent.Agent, error) {
	return nil, fmt.Errorf("Get not supported; use NewRegistryAdapter for full functionality")
}

func (r *ListOnlyRegistryAdapter) List() []string {
	return r.sdkRegistry.List()
}

func (r *ListOnlyRegistryAdapter) Register(ag agent.Agent) error {
	return fmt.Errorf("Register not supported on list-only adapter")
}
