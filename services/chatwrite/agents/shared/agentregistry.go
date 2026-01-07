package shared

import (
	"sync"

	"school-with-ai/sdk/agentsdk/core"
)

// AgentFactory creates an agent configuration with a given notifier and whiteboard projection
type AgentFactory func(notifier core.EventNotifier, whiteboardProjection interface{ Get(threadID string) string }) core.AgentConfig

// GlobalAgents is the global agent registry
var GlobalAgents = NewAgentRegistry()

// AgentRegistry manages agent factories
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]AgentFactory // agent name -> factory
}

// NewAgentRegistry creates a new agent registry
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]AgentFactory),
	}
}

// Register adds an agent factory to the registry
func (r *AgentRegistry) Register(name string, factory AgentFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.agents[name] = factory
}

// Get returns the factory for an agent by name
func (r *AgentRegistry) Get(name string) (AgentFactory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	factory, ok := r.agents[name]
	return factory, ok
}
