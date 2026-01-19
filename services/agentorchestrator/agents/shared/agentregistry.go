package shared

import (
	"school-with-ai/foundation/agentsdk/core"
	"sort"
	"sync"
)

// AgentFactory creates an agent configuration with a given notifier.
type AgentFactory func(notifier core.EventNotifier) core.AgentConfig

// GlobalAgents is the global agent registry.
var GlobalAgents = NewAgentRegistry()

// AgentRegistry manages agent factories.
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]AgentFactory // agent name -> factory
}

// NewAgentRegistry creates a new agent registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]AgentFactory),
	}
}

// Register adds an agent factory to the registry.
func (r *AgentRegistry) Register(name string, factory AgentFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[name] = factory
}

// Get returns the factory for an agent by name.
func (r *AgentRegistry) Get(name string) (AgentFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.agents[name]


	return factory, ok
}

// List returns a sorted list of registered agent names.
func (r *AgentRegistry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
